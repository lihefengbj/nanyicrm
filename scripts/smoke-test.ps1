# M1 smoke test: exercises auth, RBAC and system CRUD against a running backend.
# Usage: pwsh scripts/smoke-test.ps1 [-BaseUrl http://127.0.0.1:8080]
param(
    [string]$BaseUrl = "http://127.0.0.1:8080"
)

$ErrorActionPreference = "Stop"
$script:passed = 0
$script:failed = 0

function Check($name, $cond) {
    if ($cond) {
        $script:passed++
        Write-Output "PASS  $name"
    } else {
        $script:failed++
        Write-Output "FAIL  $name"
    }
}

function Invoke-Api($method, $path, $body = $null, $token = $null) {
    $headers = @{ "Content-Type" = "application/json" }
    if ($token) { $headers["Authorization"] = "Bearer $token" }
    $params = @{
        Method      = $method
        Uri         = "$BaseUrl$path"
        Headers     = $headers
        ErrorAction = "Stop"
    }
    if ($body) { $params["Body"] = ($body | ConvertTo-Json -Depth 10) }
    try {
        $resp = Invoke-WebRequest @params
        return @{ Status = [int]$resp.StatusCode; Body = ($resp.Content | ConvertFrom-Json) }
    } catch {
        $status = 0
        $content = $null
        if ($_.Exception.Response) {
            $status = [int]$_.Exception.Response.StatusCode
        }
        # PS7 puts the error response body in ErrorDetails.Message.
        if ($_.ErrorDetails -and $_.ErrorDetails.Message) {
            try { $content = $_.ErrorDetails.Message | ConvertFrom-Json } catch { }
        }
        return @{ Status = $status; Body = $content }
    }
}

# 1. health check
$r = Invoke-Api GET /healthz
Check "healthz returns ok" ($r.Status -eq 200 -and $r.Body.status -eq "ok")

# 2. login with wrong credential should fail with 2004
$r = Invoke-Api POST /api/v1/auth/login @{ username = "admin"; pwd = "wrong-pwd" }
Check "wrong login rejected (2004)" ($r.Status -eq 401 -and $r.Body.code -eq 2004)

# 3. login with admin/admin123
$r = Invoke-Api POST /api/v1/auth/login @{ username = "admin"; pwd = "admin123" }
Check "admin login returns token pair" ($r.Status -eq 200 -and $r.Body.code -eq 0 -and $r.Body.data.accessToken -and $r.Body.data.refreshToken)
$access = $r.Body.data.accessToken
$refresh = $r.Body.data.refreshToken

# 4. protected route without token -> 401
$r = Invoke-Api GET /api/v1/auth/profile
Check "profile without token rejected" ($r.Status -eq 401)

# 5. profile with token
$r = Invoke-Api GET /api/v1/auth/profile -token $access
Check "profile returns admin info" ($r.Status -eq 200 -and $r.Body.data.username -eq "admin" -and $r.Body.data.isAdmin -eq $true)

# cleanup leftovers from previous incomplete runs
$r = Invoke-Api GET "/api/v1/system/user?username=smokeuser" -token $access
if ($r.Body -and $r.Body.data -and $r.Body.data.records) {
    foreach ($u in @($r.Body.data.records)) {
        Invoke-Api DELETE "/api/v1/system/user/$($u.id)" -token $access | Out-Null
    }
}
$r = Invoke-Api GET "/api/v1/system/role?pageSize=100" -token $access
if ($r.Body -and $r.Body.data -and $r.Body.data.records) {
    foreach ($role in @($r.Body.data.records)) {
        if ($role.code -eq "sales") {
            Invoke-Api DELETE "/api/v1/system/role/$($role.id)" -token $access | Out-Null
        }
    }
}

# 6. create a role
$menuTree = (Invoke-Api GET /api/v1/system/menu/tree -token $access).Body.data
Check "menu tree seeded" ($menuTree.Count -gt 0)
$r = Invoke-Api POST /api/v1/system/role @{ name = "销售专员"; code = "sales"; sort = 10; status = 1; remark = "联调测试角色"; menuIds = @() } -token $access
Check "create role" ($r.Body.code -eq 0)
$roleId = $r.Body.data.id

# 7. duplicate role code rejected
$r = Invoke-Api POST /api/v1/system/role @{ name = "销售专员2"; code = "sales"; sort = 10; status = 1; remark = ""; menuIds = @() } -token $access
Check "duplicate role code rejected (3004)" ($r.Body.code -eq 3004)

# 8. create a user bound to that role
$r = Invoke-Api POST /api/v1/system/user @{ username = "smokeuser"; pwd = "smoke123"; nickname = "联调用户"; email = ""; phone = ""; status = 1; remark = ""; roleIds = @($roleId) } -token $access
Check "create user" ($r.Body.code -eq 0)
$userId = $r.Body.data.id

# 9. user list contains the new user
$r = Invoke-Api GET "/api/v1/system/user?username=smokeuser" -token $access
Check "user list finds smokeuser" ($r.Body.data.total -ge 1 -and $r.Body.data.records[0].username -eq "smokeuser")

# 10. login as the new user; role has no perms -> forbidden on user list
$r = Invoke-Api POST /api/v1/auth/login @{ username = "smokeuser"; pwd = "smoke123" }
Check "smokeuser login ok" ($r.Body.code -eq 0)
$userAccess = $r.Body.data.accessToken
$r = Invoke-Api GET /api/v1/system/user -token $userAccess
Check "no-perm user forbidden (2003)" ($r.Status -eq 403 -and $r.Body.code -eq 2003)

# 11. update user nickname
$r = Invoke-Api PUT "/api/v1/system/user/$userId" @{ username = "smokeuser"; pwd = ""; nickname = "联调用户改"; email = ""; phone = ""; status = 1; remark = "updated"; roleIds = @($roleId) } -token $access
Check "update user" ($r.Body.code -eq 0)
$r = Invoke-Api GET "/api/v1/system/user?username=smokeuser" -token $access
Check "update took effect" ($r.Body.data.records[0].nickname -eq "联调用户改")

# 12. dept create + tree + delete
$r = Invoke-Api POST /api/v1/system/dept @{ parentId = 1; name = "联调部门"; leader = ""; sort = 1; status = 1 } -token $access
Check "create dept" ($r.Body.code -eq 0)
$deptId = $r.Body.data.id
$r = Invoke-Api GET /api/v1/system/dept/tree -token $access
$found = $false
foreach ($d in $r.Body.data) { foreach ($ch in $d.children) { if ($ch.name -eq "联调部门") { $found = $true } } }
Check "dept appears in tree" $found
$r = Invoke-Api DELETE "/api/v1/system/dept/$deptId" -token $access
Check "delete dept" ($r.Body.code -eq 0)

# 13. refresh token rotation
$r = Invoke-Api POST /api/v1/auth/refresh @{ refreshToken = $refresh }
Check "refresh returns new pair" ($r.Body.code -eq 0 -and $r.Body.data.accessToken -ne $access)
$r = Invoke-Api POST /api/v1/auth/refresh @{ refreshToken = $refresh }
Check "old refresh token revoked" ($r.Body.code -eq 2001)

# 14. logs recorded
$r = Invoke-Api GET /api/v1/system/log/login -token $access
Check "login logs recorded" ($r.Body.data.total -ge 3)
$r = Invoke-Api GET /api/v1/system/log/oper -token $access
Check "oper logs recorded" ($r.Body.data.total -ge 3)

# 15. cleanup: delete test user + role
$r = Invoke-Api DELETE "/api/v1/system/user/$userId" -token $access
Check "delete user" ($r.Body.code -eq 0)
$r = Invoke-Api DELETE "/api/v1/system/role/$roleId" -token $access
Check "delete role" ($r.Body.code -eq 0)

# 16. logout revokes refresh token
$newRefresh = ((Invoke-Api POST /api/v1/auth/login @{ username = "admin"; pwd = "admin123" }).Body.data).refreshToken
$r = Invoke-Api POST /api/v1/auth/logout @{ refreshToken = $newRefresh } -token $access
Check "logout ok" ($r.Body.code -eq 0)
$r = Invoke-Api POST /api/v1/auth/refresh @{ refreshToken = $newRefresh }
Check "refresh after logout rejected" ($r.Body.code -eq 2001)

Write-Output ""
Write-Output "==== smoke test: $($script:passed) passed, $($script:failed) failed ===="
if ($script:failed -gt 0) { exit 1 }
