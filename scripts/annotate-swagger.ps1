# One-shot helper: insert swag annotations before each handler function.
# Idempotent: skips a function if the line above already has an annotation.
$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
$mod = Join-Path $root 'backend\internal\modules'

$entries = @(
  # ---- system/user ----
  @{ f = 'system\user.go'; fn = 'func (h *UserHandler) List('; sum = '用户分页列表'; tag = '系统管理-用户'; perm = 'system:user:list'; rt = '/system/user [get]' },
  @{ f = 'system\user.go'; fn = 'func (h *UserHandler) Create('; sum = '新增用户'; tag = '系统管理-用户'; perm = 'system:user:create'; rt = '/system/user [post]' },
  @{ f = 'system\user.go'; fn = 'func (h *UserHandler) Update('; sum = '编辑用户'; tag = '系统管理-用户'; perm = 'system:user:update'; rt = '/system/user/{id} [put]' },
  @{ f = 'system\user.go'; fn = 'func (h *UserHandler) Delete('; sum = '删除用户'; tag = '系统管理-用户'; perm = 'system:user:delete'; rt = '/system/user/{id} [delete]' },
  # ---- system/role ----
  @{ f = 'system\role.go'; fn = 'func (h *RoleHandler) List('; sum = '角色分页列表'; tag = '系统管理-角色'; perm = 'system:role:list'; rt = '/system/role [get]' },
  @{ f = 'system\role.go'; fn = 'func (h *RoleHandler) All('; sum = '全部角色（下拉用）'; tag = '系统管理-角色'; perm = 'system:role:list'; rt = '/system/role/all [get]' },
  @{ f = 'system\role.go'; fn = 'func (h *RoleHandler) Create('; sum = '新增角色'; tag = '系统管理-角色'; perm = 'system:role:create'; rt = '/system/role [post]' },
  @{ f = 'system\role.go'; fn = 'func (h *RoleHandler) Update('; sum = '编辑角色（含菜单授权）'; tag = '系统管理-角色'; perm = 'system:role:update'; rt = '/system/role/{id} [put]' },
  @{ f = 'system\role.go'; fn = 'func (h *RoleHandler) Delete('; sum = '删除角色'; tag = '系统管理-角色'; perm = 'system:role:delete'; rt = '/system/role/{id} [delete]' },
  # ---- system/menu ----
  @{ f = 'system\menu.go'; fn = 'func (h *MenuHandler) Tree('; sum = '菜单树（角色授权/菜单管理）'; tag = '系统管理-菜单'; perm = 'system:menu:list'; rt = '/system/menu/tree [get]' },
  @{ f = 'system\menu.go'; fn = 'func (h *MenuHandler) Create('; sum = '新增菜单'; tag = '系统管理-菜单'; perm = 'system:menu:create'; rt = '/system/menu [post]' },
  @{ f = 'system\menu.go'; fn = 'func (h *MenuHandler) Update('; sum = '编辑菜单'; tag = '系统管理-菜单'; perm = 'system:menu:update'; rt = '/system/menu/{id} [put]' },
  @{ f = 'system\menu.go'; fn = 'func (h *MenuHandler) Delete('; sum = '删除菜单'; tag = '系统管理-菜单'; perm = 'system:menu:delete'; rt = '/system/menu/{id} [delete]' },
  # ---- system/dept ----
  @{ f = 'system\dept.go'; fn = 'func (h *DeptHandler) Tree('; sum = '部门树'; tag = '系统管理-部门'; perm = 'system:dept:list'; rt = '/system/dept/tree [get]' },
  @{ f = 'system\dept.go'; fn = 'func (h *DeptHandler) Create('; sum = '新增部门'; tag = '系统管理-部门'; perm = 'system:dept:create'; rt = '/system/dept [post]' },
  @{ f = 'system\dept.go'; fn = 'func (h *DeptHandler) Update('; sum = '编辑部门'; tag = '系统管理-部门'; perm = 'system:dept:update'; rt = '/system/dept/{id} [put]' },
  @{ f = 'system\dept.go'; fn = 'func (h *DeptHandler) Delete('; sum = '删除部门'; tag = '系统管理-部门'; perm = 'system:dept:delete'; rt = '/system/dept/{id} [delete]' },
  # ---- system/tenant ----
  @{ f = 'system\tenant.go'; fn = 'func (h *TenantHandler) List('; sum = '租户分页列表'; tag = '系统管理-租户'; perm = '平台超管'; rt = '/system/tenant [get]' },
  @{ f = 'system\tenant.go'; fn = 'func (h *TenantHandler) All('; sum = '全部租户（下拉用）'; tag = '系统管理-租户'; perm = '平台超管'; rt = '/system/tenant/all [get]' },
  @{ f = 'system\tenant.go'; fn = 'func (h *TenantHandler) Create('; sum = '新增租户'; tag = '系统管理-租户'; perm = '平台超管'; rt = '/system/tenant [post]' },
  @{ f = 'system\tenant.go'; fn = 'func (h *TenantHandler) Update('; sum = '编辑租户'; tag = '系统管理-租户'; perm = '平台超管'; rt = '/system/tenant/{id} [put]' },
  @{ f = 'system\tenant.go'; fn = 'func (h *TenantHandler) Delete('; sum = '删除租户'; tag = '系统管理-租户'; perm = '平台超管'; rt = '/system/tenant/{id} [delete]' },
  # ---- system/dict ----
  @{ f = 'system\dict.go'; fn = 'func (h *DictHandler) List('; sum = '字典分页列表'; tag = '系统管理-字典'; perm = 'system:dict:list'; rt = '/system/dict [get]' },
  @{ f = 'system\dict.go'; fn = 'func (h *DictHandler) Items('; sum = '按类型取字典项（下拉框用）'; tag = '系统管理-字典'; perm = '登录即可'; rt = '/system/dict/items/{type} [get]' },
  @{ f = 'system\dict.go'; fn = 'func (h *DictHandler) Create('; sum = '新增字典'; tag = '系统管理-字典'; perm = 'system:dict:create'; rt = '/system/dict [post]' },
  @{ f = 'system\dict.go'; fn = 'func (h *DictHandler) Update('; sum = '编辑字典'; tag = '系统管理-字典'; perm = 'system:dict:update'; rt = '/system/dict/{id} [put]' },
  @{ f = 'system\dict.go'; fn = 'func (h *DictHandler) Delete('; sum = '删除字典'; tag = '系统管理-字典'; perm = 'system:dict:delete'; rt = '/system/dict/{id} [delete]' },
  @{ f = 'system\dict.go'; fn = 'func (h *DictHandler) CreateItem('; sum = '新增字典项'; tag = '系统管理-字典'; perm = 'system:dict:update'; rt = '/system/dict/item [post]' },
  @{ f = 'system\dict.go'; fn = 'func (h *DictHandler) UpdateItem('; sum = '编辑字典项'; tag = '系统管理-字典'; perm = 'system:dict:update'; rt = '/system/dict/item/{id} [put]' },
  @{ f = 'system\dict.go'; fn = 'func (h *DictHandler) DeleteItem('; sum = '删除字典项'; tag = '系统管理-字典'; perm = 'system:dict:update'; rt = '/system/dict/item/{id} [delete]' },
  # ---- system/log ----
  @{ f = 'system\log.go'; fn = 'func (h *LogHandler) OperList('; sum = '操作日志分页列表'; tag = '系统管理-日志'; perm = 'system:log:oper'; rt = '/system/log/oper [get]' },
  @{ f = 'system\log.go'; fn = 'func (h *LogHandler) LoginList('; sum = '登录日志分页列表'; tag = '系统管理-日志'; perm = 'system:log:login'; rt = '/system/log/login [get]' },
  # ---- crm/customer ----
  @{ f = 'crm\customer.go'; fn = 'func (h *CustomerHandler) List('; sum = '客户分页列表'; tag = 'CRM-客户'; perm = 'crm:customer:list'; rt = '/crm/customer [get]' },
  @{ f = 'crm\customer.go'; fn = 'func (h *CustomerHandler) All('; sum = '全部客户（下拉用）'; tag = 'CRM-客户'; perm = 'crm:customer:list'; rt = '/crm/customer/all [get]' },
  @{ f = 'crm\customer.go'; fn = 'func (h *CustomerHandler) Create('; sum = '新增客户'; tag = 'CRM-客户'; perm = 'crm:customer:create'; rt = '/crm/customer [post]' },
  @{ f = 'crm\customer.go'; fn = 'func (h *CustomerHandler) Update('; sum = '编辑客户'; tag = 'CRM-客户'; perm = 'crm:customer:update'; rt = '/crm/customer/{id} [put]' },
  @{ f = 'crm\customer.go'; fn = 'func (h *CustomerHandler) Delete('; sum = '删除客户（级联联系人与跟进）'; tag = 'CRM-客户'; perm = 'crm:customer:delete'; rt = '/crm/customer/{id} [delete]' },
  # ---- crm/contact ----
  @{ f = 'crm\contact.go'; fn = 'func (h *ContactHandler) List('; sum = '联系人分页列表'; tag = 'CRM-联系人'; perm = 'crm:contact:list'; rt = '/crm/contact [get]' },
  @{ f = 'crm\contact.go'; fn = 'func (h *ContactHandler) Create('; sum = '新增联系人'; tag = 'CRM-联系人'; perm = 'crm:contact:create'; rt = '/crm/contact [post]' },
  @{ f = 'crm\contact.go'; fn = 'func (h *ContactHandler) Update('; sum = '编辑联系人'; tag = 'CRM-联系人'; perm = 'crm:contact:update'; rt = '/crm/contact/{id} [put]' },
  @{ f = 'crm\contact.go'; fn = 'func (h *ContactHandler) Delete('; sum = '删除联系人'; tag = 'CRM-联系人'; perm = 'crm:contact:delete'; rt = '/crm/contact/{id} [delete]' },
  # ---- crm/follow ----
  @{ f = 'crm\follow.go'; fn = 'func (h *FollowUpHandler) List('; sum = '跟进记录分页列表'; tag = 'CRM-跟进'; perm = 'crm:follow:list'; rt = '/crm/follow [get]' },
  @{ f = 'crm\follow.go'; fn = 'func (h *FollowUpHandler) Create('; sum = '新增跟进记录'; tag = 'CRM-跟进'; perm = 'crm:follow:create'; rt = '/crm/follow [post]' },
  @{ f = 'crm\follow.go'; fn = 'func (h *FollowUpHandler) Update('; sum = '编辑跟进记录（仅本人）'; tag = 'CRM-跟进'; perm = 'crm:follow:update'; rt = '/crm/follow/{id} [put]' },
  @{ f = 'crm\follow.go'; fn = 'func (h *FollowUpHandler) Delete('; sum = '删除跟进记录（仅本人）'; tag = 'CRM-跟进'; perm = 'crm:follow:delete'; rt = '/crm/follow/{id} [delete]' },
  # ---- crm/opportunity ----
  @{ f = 'crm\opportunity.go'; fn = 'func (h *OpportunityHandler) List('; sum = '商机分页列表'; tag = 'CRM-商机'; perm = 'crm:opportunity:list'; rt = '/crm/opportunity [get]' },
  @{ f = 'crm\opportunity.go'; fn = 'func (h *OpportunityHandler) All('; sum = '全部商机（下拉用）'; tag = 'CRM-商机'; perm = 'crm:opportunity:list'; rt = '/crm/opportunity/all [get]' },
  @{ f = 'crm\opportunity.go'; fn = 'func (h *OpportunityHandler) Create('; sum = '新增商机'; tag = 'CRM-商机'; perm = 'crm:opportunity:create'; rt = '/crm/opportunity [post]' },
  @{ f = 'crm\opportunity.go'; fn = 'func (h *OpportunityHandler) Update('; sum = '编辑商机'; tag = 'CRM-商机'; perm = 'crm:opportunity:update'; rt = '/crm/opportunity/{id} [put]' },
  @{ f = 'crm\opportunity.go'; fn = 'func (h *OpportunityHandler) Delete('; sum = '删除商机'; tag = 'CRM-商机'; perm = 'crm:opportunity:delete'; rt = '/crm/opportunity/{id} [delete]' },
  # ---- crm/contract ----
  @{ f = 'crm\contract.go'; fn = 'func (h *ContractHandler) List('; sum = '合同分页列表'; tag = 'CRM-合同'; perm = 'crm:contract:list'; rt = '/crm/contract [get]' },
  @{ f = 'crm\contract.go'; fn = 'func (h *ContractHandler) Create('; sum = '新增合同（关联商机自动赢单）'; tag = 'CRM-合同'; perm = 'crm:contract:create'; rt = '/crm/contract [post]' },
  @{ f = 'crm\contract.go'; fn = 'func (h *ContractHandler) Update('; sum = '编辑合同'; tag = 'CRM-合同'; perm = 'crm:contract:update'; rt = '/crm/contract/{id} [put]' },
  @{ f = 'crm\contract.go'; fn = 'func (h *ContractHandler) Delete('; sum = '删除合同'; tag = 'CRM-合同'; perm = 'crm:contract:delete'; rt = '/crm/contract/{id} [delete]' },
  # ---- crm/dashboard ----
  @{ f = 'crm\dashboard.go'; fn = 'func (h *DashboardHandler) Summary('; sum = '工作台汇总指标'; tag = '工作台'; perm = '登录即可'; rt = '/dashboard/summary [get]' }
)

$byFile = $entries | Group-Object f
foreach ($group in $byFile) {
  $path = Join-Path $mod $group.Name
  $lines = [System.Collections.Generic.List[string]]([System.IO.File]::ReadAllLines($path))
  # Insert from bottom to top so line numbers stay valid.
  $targets = @()
  for ($i = 0; $i -lt $lines.Count; $i++) {
    foreach ($e in $group.Group) {
      if ($lines[$i].TrimStart().StartsWith($e.fn)) { $targets += @{ i = $i; e = $e } }
    }
  }
  $targets = $targets | Sort-Object { $_.i } -Descending
  foreach ($t in $targets) {
    $e = $t.e
    # Skip if an annotation block already sits above (idempotency).
    if ($t.i -gt 0 -and $lines[$t.i - 1] -match '@Router') { continue }
    $block = @(
      "// @Summary  $($e.sum)"
      "// @Tags     $($e.tag)"
      "// @Description 需要权限：$($e.perm)"
      "// @Success  200  {object}  map[string]interface{}"
      "// @Security BearerAuth"
      "// @Router   $($e.rt)"
    )
    $lines.InsertRange($t.i, [string[]]$block)
  }
  [System.IO.File]::WriteAllLines($path, $lines)
  Write-Output "annotated $($group.Name): $($targets.Count) handlers"
}
