package common

// Error code ranges: 1xxx params, 2xxx auth, 3xxx business, 5xxx system.
const (
	CodeSuccess      = 0
	CodeParamInvalid = 1001

	CodeUnauthorized    = 2001
	CodeTokenExpired    = 2002
	CodeForbidden       = 2003
	CodeLoginFailed     = 2004
	CodeAccountDisabled = 2005

	CodeUserNotFound   = 3001
	CodeTenantNotFound = 3008
	CodeTenantExists   = 3009
	CodeTenantInUse    = 3010
	CodeTenantDisabled = 3011
	CodeTenantExpired  = 3012
	CodeUserExists     = 3002
	CodeRoleNotFound   = 3003
	CodeRoleExists     = 3004
	CodeMenuNotFound   = 3005
	CodeDeptNotFound   = 3006
	CodeRecordNotFound = 3007

	CodeInternalError = 5001
	CodeDBError       = 5002
	CodeRedisError    = 5003
)

var codeMessages = map[int]string{
	CodeSuccess:      "success",
	CodeParamInvalid: "参数错误",

	CodeUnauthorized:    "未登录或登录已失效",
	CodeTokenExpired:    "登录已过期",
	CodeForbidden:       "没有操作权限",
	CodeLoginFailed:     "用户名或密码错误",
	CodeAccountDisabled: "账号已被停用",

	CodeUserNotFound:   "用户不存在",
	CodeTenantNotFound: "租户不存在",
	CodeTenantExists:   "租户编码已存在",
	CodeTenantInUse:    "租户下存在数据，不可删除",
	CodeTenantDisabled: "租户已被停用",
	CodeTenantExpired:  "租户已过期",
	CodeUserExists:     "用户名已存在",
	CodeRoleNotFound:   "角色不存在",
	CodeRoleExists:     "角色标识已存在",
	CodeMenuNotFound:   "菜单不存在",
	CodeDeptNotFound:   "部门不存在",
	CodeRecordNotFound: "记录不存在",

	CodeInternalError: "系统内部错误",
	CodeDBError:       "数据库操作失败",
	CodeRedisError:    "缓存服务异常",
}

func CodeMessage(code int) string {
	if msg, ok := codeMessages[code]; ok {
		return msg
	}
	return "未知错误"
}
