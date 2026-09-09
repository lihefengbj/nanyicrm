# Nanyi CRM 技术方案

本文档确定 Nanyi CRM 的整体技术选型、架构设计与落地计划，作为后续开发的基础依据。

## 1. 技术选型

### 1.1 选型结论

| 层次 | 选型 | 版本基线 |
| --- | --- | --- |
| 后端语言 | Go | 1.22+ |
| 后端框架 | Gin + GORM | Gin 1.10 / GORM 1.25 |
| 数据库 | MySQL | 8.0 |
| 缓存 | Redis | 7.x |
| 前端框架 | Vue 3 + TypeScript + Vite | Vue 3.4+ |
| 前端 UI | Element Plus | 2.x |
| 状态管理 | Pinia | 2.x |
| 认证授权 | JWT（golang-jwt）+ 自实现 RBAC 模型 | - |
| 数据库迁移 | golang-migrate | 4.x |
| API 文档 | swaggo/swag（Swagger UI） | 1.x |
| 构建工具 | Go Modules（后端）/ pnpm（前端） | - |
| 部署 | Docker Compose + Nginx | 单机起步，预留 K8s |

### 1.2 选型理由

- **Go**：团队对 Go 更熟悉；编译为单文件二进制，部署和运维简单，并发模型适合 API 服务，学习成本低。
- **Gin**：Go 生态中最主流的 Web 框架，中间件机制清晰，路由、参数校验、错误处理开箱即用，社区资料丰富。
- **GORM**：CRM 业务以结构化 CRUD 和复杂条件查询为主，GORM 在保留 SQL 可控性的同时显著减少模板代码。
- **Vue 3 + Element Plus**：管理后台场景的事实标准，表格、表单、权限指令等配套完善，开发效率高。
- **JWT + RBAC**：无状态认证便于水平扩展；RBAC 覆盖用户、角色、菜单、数据权限，直接支撑"组织权限"模块。基于 GORM 模型自实现即可，无需引入额外授权框架。

### 1.3 备选方案

| 备选 | 放弃原因 |
| --- | --- |
| Java (Spring Boot) | 生态成熟但团队不熟悉，Go 部署更简单、上手更快，满足当前规模需求 |
| Node.js (NestJS) | 与前端同语言有协作优势，但弱类型在复杂权限和查询场景下维护成本更高 |
| Python (Django) | 开发快，但性能与部署形态（解释型、多进程）不如 Go 单二进制简洁 |
| Rust (Axum) | 性能上限高，但开发效率和招聘成本不适合快速交付的 CRM 项目 |
| React + Ant Design | 与 Vue 方案能力相当，依据国内后台开发社区习惯选择 Vue |
| PostgreSQL | 功能更强，但 MySQL 在国内运维与托管生态更普及，CRM 场景无硬性 PG 需求 |

## 2. 总体架构

```text
浏览器
   │  HTTPS
   ▼
Nginx（静态资源 + 反向代理）
   │  /api/**
   ▼
Go 应用（无状态单二进制，可多实例）
   ├── MySQL（业务数据，golang-migrate 管理迁移）
   └── Redis（会话令牌、验证码、热点缓存）
```

架构原则：

- 前后端完全分离，仅通过 REST API 交互
- 后端无状态，认证信息放在 JWT 中，便于后续水平扩容
- 所有写操作记录操作日志，关键业务规则有自动化测试覆盖
- 配置外置（环境变量 / application-*.yml），密钥不入库不入仓

## 3. 目录结构

```text
.
├── docs/                # 产品、架构和接口文档
├── frontend/            # Vue 3 + Vite 管理端
│   ├── src/
│   │   ├── api/         # 接口封装
│   │   ├── views/       # 页面（按业务模块划分）
│   │   ├── router/      # 动态路由（按权限生成）
│   │   ├── store/       # Pinia
│   │   └── components/  # 通用组件
├── backend/             # Go 应用
│   ├── cmd/server/      # 程序入口
│   ├── internal/
│   │   ├── common/      # 统一响应、错误码、分页
│   │   ├── config/      # 配置加载
│   │   ├── middleware/  # JWT 认证、权限、操作日志、CORS
│   │   ├── modules/
│   │   │   ├── system/      # 用户、部门、角色、权限、字典、日志
│   │   │   ├── customer/    # 客户、联系人、跟进
│   │   │   ├── business/    # 商机
│   │   │   └── contract/    # 合同
│   │   └── router/      # 路由注册
│   ├── migrations/      # golang-migrate SQL 脚本
│   └── go.mod
├── deploy/              # docker-compose、Nginx 配置
├── scripts/             # 开发、构建和部署脚本
└── README.md
```

## 4. 核心设计

### 4.1 认证与权限

- 登录颁发 Access Token（2 小时）+ Refresh Token（7 天），Refresh Token 存 Redis 可强制下线
- RBAC：用户 - 角色 - 权限（菜单 / 按钮 / 接口三级）
- 数据权限：按部门树过滤（本人 / 本部门 / 本部门及以下 / 全部），客户归属字段驱动

### 4.2 核心数据模型（一期）

```text
sys_user / sys_dept / sys_role / sys_menu / sys_user_role / sys_role_menu
biz_customer          # 客户（归属人、状态、标签、来源）
biz_contact           # 联系人（属客户）
biz_follow_record     # 跟进记录（客户/联系人/商机多态关联）
biz_opportunity       # 商机（阶段、预计金额、预计成交日期）
biz_contract          # 合同（关联客户与商机）
sys_dict / sys_dict_item / sys_oper_log / sys_login_log
```

### 4.3 接口约定

- 统一前缀 `/api`，REST 风格，版本号预留（`/api/v1`）
- 统一响应结构：`{ code, message, data }`，分页 `{ records, total, pageNum, pageSize }`
- 统一异常码段：1xxx 参数 / 2xxx 认证授权 / 3xxx 业务 / 5xxx 系统
- 接口文档由 swaggo/swag 从代码注解自动生成，随代码更新

## 5. 里程碑

| 阶段 | 内容 | 目标 |
| --- | --- | --- |
| M1 基础框架 | 工程初始化、golang-migrate、登录认证、RBAC、统一异常与响应、操作日志 | 可登录、可配权限的空壳系统 |
| M2 客户域 | 客户、联系人、跟进记录 CRUD 与数据权限 | 客户全生命周期可录入可追踪 |
| M3 销售域 | 商机、合同、工作台数据汇总 | 销售流程闭环 |
| M4 打磨 | 系统设置、审计报表、CI/CD、部署文档 | 可交付试运行 |

## 6. 质量与流程

- 后端：golangci-lint + go test（table-driven）；覆盖率门槛先在 M2 引入（核心业务 ≥ 70%）
- 前端：ESLint + Prettier + Vitest；关键交互补 e2e（Playwright）
- 提交信息遵循 Conventional Commits（见 README）
- 每次合并前跑 `go test ./...` 与 `pnpm build`，CI 在 M1 末接入

## 7. 环境要求

- Go 1.22+、Node 18+（建议 20 LTS）、pnpm 9+
- 本地依赖（MySQL / Redis）通过 `deploy/docker-compose.dev.yml` 一键启动
