# API 文档（Swagger）接入方案

本文档补齐技术方案中承诺但未落地的接口文档能力：`docs/solution.md` 第 1.1 节选型 `swaggo/swag`、第 4.3 节约定"接口文档由代码注解自动生成"，当前 `go.mod` 无相关依赖、接口无注解。本文档定义接入方式与注解规范。

## 1. 目标与原则

- 全部 `/api/v1/**` 接口可通过 Swagger UI 在线浏览、调试
- 注解随代码维护，生成物（`docs.go` / `swagger.json` / `swagger.yaml`）不进版本库或进库但由 CI 校验一致性
- 生产环境默认关闭 Swagger UI，仅开发/测试环境开启

## 2. 依赖与工具

| 项 | 选型 | 说明 |
| --- | --- | --- |
| 注解生成 | `github.com/swaggo/swag/cmd/swag` v1.x | `swag init` 扫描注解生成文档 |
| Gin 集成 | `github.com/swaggo/gin-swagger` | 挂载 Swagger UI 路由 |
| 文档文件 | `github.com/swaggo/files` | 静态资源 |

安装与生成命令（加入 `scripts/` 与 README）：

```bash
go install github.com/swaggo/swag/cmd/swag@latest
cd backend
swag init -g cmd/server/main.go -o internal/apidocs --parseInternal
```

生成目录定为 `backend/internal/apidocs/`（避免与根 `docs/` 设计文档混淆），由 `main.go` 以空白导入引用。

## 3. 路由挂载

在 `internal/router/router.go` 中追加：

```go
import (
    swaggerFiles "github.com/swaggo/files"
    ginSwagger "github.com/swaggo/gin-swagger"
    _ "github.com/lihefengbj/nanyicrm/backend/internal/apidocs"
)

// 仅非生产环境开启
if cfg.App.Env != "prod" {
    r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}
```

- 环境判定复用现有 `config.Config`，新增 `App.Env` 配置项（默认 `dev`），不引入新的配置框架。
- Swagger UI 不需要 JWT：它只读文档；在线调试时由用户在 UI 中粘贴 Bearer Token（见 §4 安全定义）。

## 4. 通用注解（main.go）

```go
// @title        Nanyi CRM API
// @version      1.0
// @description  Nanyi CRM 管理后台接口文档。统一前缀 /api/v1，响应结构 { code, message, data }。
// @host         localhost:8080
// @BasePath     /api/v1
// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
// @description                 输入 "Bearer {access_token}"
```

分页响应、统一响应结构通过泛型包装在 swag 中表达成本高，约定：每个接口的 `@Success` 直接描述 `data` 内容，统一外壳在文档描述中说明一次，不逐接口重复。

## 5. 接口注解规范

每个 handler 方法上方按以下模板注解，以用户列表为例：

```go
// List 用户列表
// @Summary  用户分页列表
// @Tags     系统管理-用户
// @Param    pageNum   query  int     false "页码，默认 1"
// @Param    pageSize  query  int     false "每页条数，默认 10"
// @Param    username  query  string  false "用户名模糊"
// @Success  200  {object}  common.PageResp{records=[]system.UserVO}
// @Security BearerAuth
// @Router   /system/user [get]
```

规范约定：

- `@Tags` 按"模块-资源"两级命名，与前端菜单结构对应：`系统管理-用户`、`系统管理-菜单`、`CRM-客户`、`CRM-合同` 等
- 路径参数用 `@Param id path int true "主键"`；请求体用 `@Param body body system.CreateUserReq true "请求体"`
- 请求/响应结构体独立定义在模块内（`*Req` / `*VO` 后缀），避免把 GORM 模型（含 `PwdHash`、`json:"-"` 字段）直接暴露给文档
- 涉及权限的接口在 `@Description` 中注明所需 perms，如 `需要权限 system:user:list`
- 错误响应统一不逐接口注解，在文档描述中说明异常码段（1xxx/2xxx/3xxx/5xxx）

## 6. 实施范围与步骤

按模块分批补注解，每批可独立合入：

| 批次 | 范围 | 接口数（约） |
| --- | --- | --- |
| 1 | 基础设施：依赖接入、main.go 通用注解、路由挂载、auth 模块 | 4 |
| 2 | system 模块：user / role / menu / dept / tenant / dict / log | 30 |
| 3 | crm 模块：customer / contact / follow / opportunity / contract / dashboard | 25 |

验收标准：

- `swag init` 无警告通过，`/swagger/index.html` 可浏览全部接口
- 在 Swagger UI 配置 Bearer Token 后可直接调试受保护接口
- `prod` 环境启动时 `/swagger/**` 返回 404
- README 与 `scripts/` 补充文档生成命令；CI（`.github/workflows`）加一步 `swag init` 校验，防止注解与代码漂移
