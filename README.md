# Nanyi CRM

Nanyi CRM 是一个面向客户关系管理场景的基础框架，目标是为客户、联系人、商机、合同及跟进记录等业务模块提供统一、可扩展的工程底座。

> 技术选型与架构设计已定，详见 [docs/solution.md](docs/solution.md)；部署方式见 [docs/deployment.md](docs/deployment.md)。
> AI 客户意向分析设计与实施计划见 [docs/ai-customer-intent.md](docs/ai-customer-intent.md)。
> M1~M6 全部里程碑已完成，端到端冒烟测试 51 项断言全部通过（可重复执行）。
> 生产上线前的安全与质量整改项见 [docs/hardening-plan.md](docs/hardening-plan.md)。

## 项目目标

- 建立清晰、可维护的 CRM 领域模型
- 提供统一的用户、角色与权限管理能力
- 支持客户全生命周期的信息沉淀与跟进
- 为后续业务模块提供稳定的扩展基础
- 统一开发、测试、构建与发布流程

## 规划模块

| 模块 | 说明 |
| --- | --- |
| 工作台 | 汇总待办事项、业务数据和常用入口 |
| 客户管理 | 管理客户档案、归属、状态及标签 |
| 联系人管理 | 维护客户联系人及沟通信息 |
| 跟进管理 | 记录电话、拜访、会议等跟进活动 |
| 商机管理 | 管理销售阶段、预计金额和赢单进度 |
| 合同管理 | 维护合同信息、状态及关联客户 |
| AI 客户意向 | 基于客户资料和沟通记录分析客户意向，辅助后续跟进 |
| 组织权限 | 管理用户、部门、角色和数据权限 |
| 系统设置 | 管理字典、参数、日志等基础配置 |

## 目录结构

```text
.
├── docs/          # 产品、架构和接口文档
├── frontend/      # Vue 3 + Element Plus 管理端
├── backend/       # Go (Gin + GORM) 服务端
├── deploy/        # docker-compose 等部署配置
├── scripts/       # 冒烟测试等辅助脚本
└── README.md
```

## 当前进度

| 里程碑 | 状态 | 说明 |
| --- | --- | --- |
| M1 基础框架 | ✅ 已完成 | 登录认证、RBAC 权限、用户/角色/部门/菜单管理、操作与登录日志 |
| M1.5 平台化 | ✅ 已完成 | YAML 配置、按天滚动日志、多数据库连接、多租户、多标签页、权限三级模型 |
| M2 客户域 | ✅ 已完成 | 客户、联系人、跟进记录 CRUD 与数据权限 |
| M3 销售域 | ✅ 已完成 | 商机、合同、工作台数据汇总 |
| M4 打磨 | ✅ 已完成 | 字典管理、审计日志、CI、Docker 生产部署与部署文档 |
| M5 治理能力 | ✅ 已完成 | 菜单管理 CRUD、Swagger 接口文档、API 接口管理（自动采集） |
| M6 动态菜单 | ✅ 已完成 | 侧边栏与路由按后端菜单树动态生成，菜单管理闭环 |
| M7 AI 客户意向 | ✅ 已完成首期 | 大模型适配、意向分析、历史记录、前端展示及跟进后异步分析 |

### M6 动态菜单明细

| 能力 | 说明 |
| --- | --- |
| 菜单树下发 | `GET /auth/profile` 增加 `menus`：按角色授权返回启用中的目录+菜单树（superAdmin 全量，隐藏菜单也下发供路由注册） |
| 动态路由 | `import.meta.glob` 组件映射 + `router.addRoute` 按菜单树注册；登出时清理；component 填错显示「页面未实现」占位 |
| 动态侧边栏 | 递归组件渲染菜单树，硬编码菜单项全部移除；隐藏菜单不进侧边栏但可直达，停用即不可访问 |
| 设计文档 | [docs/dynamic-menu.md](docs/dynamic-menu.md) |

### M5 治理能力明细

| 能力 | 说明 |
| --- | --- |
| 菜单管理 | 目录/菜单/按钮三级 CRUD：类型字段约束、perms 唯一、防成环、有子级或被角色引用禁止删除；前端树形表格页 `system/menu`；设计见 [docs/menu-management.md](docs/menu-management.md) |
| Swagger 接口文档 | 接入 swaggo：58 个接口全部注解，`/swagger/index.html` 在线浏览调试；`app.env=prod` 时自动关闭；生成物在 `backend/internal/apidocs/`；规范见 [docs/api-docs-swagger.md](docs/api-docs-swagger.md) |
| API 接口管理 | `sys_api` 注册表：启动时自动从路由注册处采集 method/path/handler/perms 幂等同步（代码即事实源）；页面可检索、维护接口名称、手动同步；迁移脚本 `000004_sys_api`；设计见 [docs/api-management.md](docs/api-management.md) |
| 字典删除修复 | 字典/字典项改为硬删除（与部门同一唯一索引原因），冒烟测试可重复执行；`cmd/purge` 一次性清理历史软删除残留 |

### M1.5 平台化改造明细

| 能力 | 说明 |
| --- | --- |
| YAML 配置 | 配置从环境变量迁移到 `backend/config/config.yaml`，支持 `CONFIG_PATH` 覆盖 |
| 按天日志 | 日志输出到 `backend/log/server_yyyymmdd.log`，跨天自动滚动，可配保留天数 |
| 多数据库连接 | `config.yaml` 的 `mysql` 为列表，支持同时连接多个 MySQL，按 name 区分 |
| 多租户 | 用户/部门/日志按租户隔离；角色全局共享；超管可看全部数据并按租户筛选 |
| 租户生命周期 | 支持停用/启用、过期时间（到期禁止登录）、有数据时禁止删除 |
| 权限三级模型 | superAdmin（硬编码超管，全放开）→ admin 角色（跨租户数据，菜单按授权）→ 普通角色（仅本租户） |
| 多标签页 | 管理端支持多页签 + keep-alive 缓存，右键可关闭当前/其他/全部 |
| 视觉统一 | 全局 teal 主题色，登录页与后台同一套视觉语言 |

### M2 客户域明细

| 能力 | 说明 |
| --- | --- |
| 客户管理 | 客户档案 CRUD、等级 A/B/C、状态（跟进中/已成交/已流失）、归属人、`mine=1` 只看自己负责的客户 |
| 联系人管理 | 按客户维护联系人，支持首要联系人标记，客户跨租户校验 |
| 跟进记录 | 电话/拜访/会议/其他四类跟进，记录人快照，下次跟进时间，只能改自己写的跟进 |
| 数据权限 | 沿用三级模型：超管/admin 跨租户，普通角色仅本租户；删除客户级联删除其联系人与跟进 |
| 权限点 | `crm:customer:*`、`crm:contact:*`、`crm:follow:*`，菜单与按钮权限自动播种并授权给内置角色 |

### M3 销售域明细

| 能力 | 说明 |
| --- | --- |
| 商机管理 | 6 级销售阶段（初步接触→商务谈判→赢单/输单）、预计金额与成交日期、归属人 |
| 合同管理 | 编号租户内唯一、可选关联商机（关联即自动赢单）、履行周期、4 态状态机 |
| 工作台汇总 | `GET /dashboard/summary`：客户/商机/合同/跟进聚合指标 + 商机阶段分布，按租户隔离 |

### M4 打磨明细

| 能力 | 说明 |
| --- | --- |
| 字典管理 | 字典 + 字典项 CRUD，租户隔离、平台级字典全员共享；`/system/dict/items/:type` 供下拉框免鉴权点使用 |
| CI | GitHub Actions：后端 gofmt/vet/build/test + 前端 pnpm build |
| 生产部署 | 后端/前端多阶段 Dockerfile + `deploy/docker-compose.prod.yml` 一体化编排；配置支持 `${ENV}` 占位符 |
| 部署文档 | [docs/deployment.md](docs/deployment.md)：一体化部署、手动部署、健康检查、升级流程 |

M1~M4 已通过端到端冒烟测试（51 项断言全部通过），重跑方式：

```bash
# 先启动后端，再执行：
pwsh scripts/smoke-test.ps1
```

## 快速开始

### 环境要求

- Go 1.22+、Node 18+（建议 20 LTS）、pnpm 9+、Docker Desktop

### 启动本地依赖（MySQL 8 + Redis 7）

```bash
docker compose -f deploy/docker-compose.dev.yml up -d
```

### 启动后端

```bash
cd backend
# 配置文件位于 backend/config/config.yaml，按需修改
go run ./cmd/server
```

首次启动会自动建表并写入种子数据。开发配置的初始账号为：

- 内置超级管理员 `superAdmin / superAdmin123`（硬编码最高权限，全部菜单与数据放开）
- 内置管理员 `admin / admin123`（跨租户数据视野，权限按角色分配）

生产环境必须通过 `BOOTSTRAP_ADMIN_PASSWORD` 和
`BOOTSTRAP_SUPER_ADMIN_PASSWORD` 设置至少 12 位的初始密码。

### 启动前端

```bash
cd frontend
pnpm install
pnpm dev
```

访问 http://localhost:5173 ，前端通过 Vite 代理将 `/api` 转发到 8080 端口的后端。

### 测试与构建

```bash
# 后端
cd backend
go test ./...
go build ./...

# 重新生成 Swagger 接口文档（修改注解后执行）
cd backend
go run github.com/swaggo/swag/cmd/swag@v1.16.4 init -g cmd/server/main.go -o internal/apidocs --parseInternal

# 前端
cd frontend
pnpm build

# 端到端冒烟测试（需要后端已启动）
pwsh scripts/smoke-test.ps1
```

## 开发约定

- 从最新的主分支创建功能分支
- 一个提交只处理一个明确的变更
- 提交前完成格式检查和相关测试
- 配置模板可以提交，密钥和生产环境配置不得提交
- 新增或变更接口时同步更新接口文档
- 重要业务规则应具备对应的自动化测试

推荐的提交信息格式：

```text
<type>(<scope>): <summary>
```

常用 `type`：

- `feat`：新增功能
- `fix`：修复问题
- `docs`：文档变更
- `refactor`：代码重构
- `test`：测试相关
- `chore`：构建、依赖或辅助任务

## 后续工作

- [x] 确定前后端技术栈（见 [docs/solution.md](docs/solution.md)）
- [x] 初始化基础工程和环境配置
- [x] 建立数据库迁移机制（GORM AutoMigrate + golang-migrate SQL 脚本）
- [x] 完成登录、组织、角色及权限框架（JWT 双 token + RBAC）
- [x] 建立统一异常处理和操作日志
- [x] 多租户改造（共享库行级隔离，租户生命周期管理）
- [x] 权限三级模型（superAdmin 硬编码超管 / admin 跨租户 / 普通租户内）
- [x] 客户、联系人、跟进记录模块（M2）
- [x] 商机、合同模块（M3）
- [x] 接入代码检查、自动化测试和持续集成
- [x] 补充架构设计、接口规范和部署文档
- [x] 菜单管理 CRUD（目录/菜单/按钮三级，M5）
- [x] Swagger 接口文档（swaggo 注解 + 在线调试，M5）
- [x] API 接口管理（sys_api 自动采集与接口清单页，M5）
- [x] 动态菜单与动态路由（侧边栏/路由按后端菜单树生成，M6，见 [docs/dynamic-menu.md](docs/dynamic-menu.md)）

## 贡献

提交变更前，请确认代码风格一致、相关测试通过，并在合并请求中说明变更目的、实现方式和验证结果。

## 许可证

当前项目暂未指定开源许可证。



