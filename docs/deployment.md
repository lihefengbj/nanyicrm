# 部署文档

本文档介绍 Nanyi CRM 的生产部署方式。技术方案与架构见 [solution.md](solution.md)；当前实现进度和后续清单见 [implementation-status.md](implementation-status.md)。

## 一、Docker Compose 一体化部署（推荐）

一条命令拉起完整栈：nginx 托管前端并反代 `/api`，Go 后端，MySQL 8，Redis 7。

```bash
# 1. 配置密钥（不要提交真实 .env）
cp deploy/.env.example deploy/.env
# 编辑 deploy/.env：数据库、Redis、JWT 及两个初始化管理员密码

# 2. 构建并启动
docker compose -f deploy/docker-compose.prod.yml --env-file deploy/.env up -d --build

# 3. 验证
curl http://localhost/healthz   # 经前端 nginx 反代
```

首次启动后端执行嵌入式版本化迁移并写入种子数据：

- `superAdmin`（密码来自 `BOOTSTRAP_SUPER_ADMIN_PASSWORD`）
- `admin`（密码来自 `BOOTSTRAP_ADMIN_PASSWORD`）

两个默认密码登录后请立即修改。

说明：

- 后端使用 `backend/config/config.prod.yaml`，其中 `${VAR}` 占位符由环境变量展开（配置加载器在读取 YAML 时执行 `os.ExpandEnv`）。
- 后端日志按天写入容器卷 `backend-log`（`log/server_yyyymmdd.log`，默认保留 30 天）。
- 前端 nginx 配置见 `frontend/nginx.conf`：静态资源 + `/api/` 反代 + SPA history 回退。

生产配置要求：

- `app.env` 必须为 `prod`；
- `app.auto_migrate` 必须为 `false`；
- JWT signing key 至少 32 位且不能使用开发默认值；
- 初始化管理员密码至少 12 位；
- `metrics.token` 在生产环境至少 16 位且不能使用示例值；
- LLM 开启时必须提供 HTTPS BaseURL 和模型名称；API Key 可通过环境变量 `LLM_API_KEY` 提供，或在 AI模型治理的凭证管理中加密保存并绑定到模型配置。生产环境启用数据库凭证时还必须提供 `LLM_CREDENTIAL_ENCRYPTION_KEY`。

生产模式启动时会读取并执行嵌入到后端二进制中的 `backend/migrations/*.up.sql`，迁移状态保存在 MySQL `schema_migrations` 表。后续升级只需构建并重启后端，服务会自动执行尚未应用的迁移。

### 开发环境配置

`backend/config/config.yaml` 同样不保存 MySQL/Redis 实际连接信息，启动前请为当前用户配置以下环境变量：

```powershell
[Environment]::SetEnvironmentVariable("MYSQL_DSN", "用户名:密码@tcp(主机:端口)/nanyicrm?charset=utf8mb4&parseTime=True&loc=Local", "User")
[Environment]::SetEnvironmentVariable("REDIS_ADDR", "主机:端口", "User")
[Environment]::SetEnvironmentVariable("REDIS_PASSWORD", "Redis密码", "User")
[Environment]::SetEnvironmentVariable("LLM_API_KEY", "大模型API Key", "User")
```

设置后需重新打开终端或重启后端进程，配置加载器会自动展开 YAML 中的 `${VAR}` 占位符。生产环境的变量名以 `deploy/.env.example` 和 `backend/config/config.prod.yaml` 为准。

## 二、手动部署

### 后端

```bash
cd backend
CGO_ENABLED=0 go build -o server ./cmd/server
# 准备配置文件（生产环境建议用 CONFIG_PATH 指定独立路径）
CONFIG_PATH=/etc/nanyicrm/config.yaml ./server
```

迁移脚本位于 `backend/migrations/`，生产服务启动时自动执行同一组嵌入式迁移。正常部署不需要单独安装 `migrate` 命令：

```sql
SELECT version, dirty FROM schema_migrations;
```

如果 `dirty = 1`，先查看后端迁移日志并完成数据库备份，不要重复修改业务表或直接跳过迁移版本。

### 前端

```bash
cd frontend
pnpm install --frozen-lockfile
pnpm build        # 产物在 frontend/dist
```

将 `dist/` 交给任意静态服务器，并把 `/api` 反代到后端 8080 端口；SPA 需要 history 回退到 `index.html`。

## 三、健康检查与运维

| 项目 | 方式 |
| --- | --- |
| 存活探针 | `GET /healthz`（未鉴权） |
| 进程存活 | `GET /livez`（未鉴权） |
| 服务就绪 | `GET /readyz`（检查 MySQL 和 Redis） |
| Prometheus 指标 | `GET /metrics`，生产使用 `X-Metrics-Token` |
| 应用日志 | `backend/log/server_yyyymmdd.log`，按天滚动 |
| 操作/登录审计 | 后台「系统管理 → 操作日志 / 登录日志」 |
| 数据库备份 | `mysqldump nanyicrm` 定期任务；快照前无需停服（InnoDB） |

## 四、升级流程

1. 升级前备份数据库。
2. 执行 `backend/migrations` 中尚未应用的版本化迁移。
3. `git pull` 后重新 `up -d --build`。
4. 后端生产配置 `app.auto_migrate: false`，启动时只执行版本化迁移；开发配置才允许 AutoMigrate。
5. 新增数据库结构必须创建递增版本的 `.up.sql` 和 `.down.sql`，不得修改已经发布的迁移文件。

应用 `000005_tenant_integrity` 前，应先确认同一租户内不存在重复合同编号：

```sql
SELECT tenant_id, code, COUNT(*) AS total
FROM crm_contract
WHERE deleted_at IS NULL AND code <> ''
GROUP BY tenant_id, code
HAVING COUNT(*) > 1;
```

本次升级会使旧 Refresh Token 失效，在线用户需要重新登录。

## 五、AI模型治理、归档与监控

- 后台「系统管理 → AI模型治理」创建候选配置。
- 候选配置必须先通过固定样本质量门禁，之后才能灰度或激活；所有操作写入模型变更审计表。
- `/metrics` 提供 Prometheus 兼容指标，生产环境使用 `X-Metrics-Token` 保护。
- 异常登录达到配置阈值后，通过 Redis 去重并向 `ALERTS_WEBHOOK_URL` 发送 JSON 告警。
- AI 原始分析在 `llm.retention_days` 后归档至归档表并从热表清理；归档再保留 `llm.archive_days`。

## 六、CI

`.github/workflows/ci.yml` 在 push 到 main 或 PR 时执行：

- 后端：`gofmt -l` 检查、`go vet`、`go build`、`go test`
- 前端：`pnpm install --frozen-lockfile`、`pnpm build`（含 vue-tsc 类型检查）

端到端冒烟测试需在能连数据库的环境手动跑：`pwsh scripts/smoke-test.ps1`。

## 七、本地验证生产迁移

如果需要在本地验证生产行为，建议使用独立配置和独立数据库：

```powershell
$env:CONFIG_PATH = "D:\path\to\config.local-prod.yaml"
go run ./cmd/server
```

不要把日常开发库直接作为生产迁移演练库。验证重点：

1. 服务日志显示版本化迁移完成；
2. `schema_migrations.dirty = 0`；
3. `/livez` 返回 200；
4. `/readyz` 同时显示 MySQL 和 Redis 可用；
5. `/swagger/index.html` 在 `env: prod` 时不可访问。
