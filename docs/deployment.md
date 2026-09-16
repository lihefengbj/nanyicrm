# 部署文档

本文档介绍 Nanyi CRM 的生产部署方式。技术方案与架构见 [solution.md](solution.md)。

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

首次启动后端自动建表（AutoMigrate）并写入种子数据：

- `superAdmin`（密码来自 `BOOTSTRAP_SUPER_ADMIN_PASSWORD`）
- `admin`（密码来自 `BOOTSTRAP_ADMIN_PASSWORD`）

两个默认密码登录后请立即修改。

说明：

- 后端使用 `backend/config/config.prod.yaml`，其中 `${VAR}` 占位符由环境变量展开（配置加载器在读取 YAML 时执行 `os.ExpandEnv`）。
- 后端日志按天写入容器卷 `backend-log`（`log/server_yyyymmdd.log`，默认保留 30 天）。
- 前端 nginx 配置见 `frontend/nginx.conf`：静态资源 + `/api/` 反代 + SPA history 回退。

## 二、手动部署

### 后端

```bash
cd backend
CGO_ENABLED=0 go build -o server ./cmd/server
# 准备配置文件（生产环境建议用 CONFIG_PATH 指定独立路径）
CONFIG_PATH=/etc/nanyicrm/config.yaml ./server
```

生产数据库如需用 golang-migrate 管理，迁移脚本在 `backend/migrations/`：

```bash
migrate -path backend/migrations -database "mysql://user:pwd@tcp(host:3306)/nanyicrm" up
```

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
| 应用日志 | `backend/log/server_yyyymmdd.log`，按天滚动 |
| 操作/登录审计 | 后台「系统管理 → 操作日志 / 登录日志」 |
| 数据库备份 | `mysqldump nanyicrm` 定期任务；快照前无需停服（InnoDB） |

## 四、升级流程

1. 升级前备份数据库。
2. 执行 `backend/migrations` 中尚未应用的版本化迁移。
3. `git pull` 后重新 `up -d --build`。
4. 后端当前仍会执行 AutoMigrate，用于校验并补齐非破坏性结构；后续将完全切换到版本化迁移。

应用 `000005_tenant_integrity` 前，应先确认同一租户内不存在重复合同编号：

```sql
SELECT tenant_id, code, COUNT(*) AS total
FROM crm_contract
WHERE deleted_at IS NULL AND code <> ''
GROUP BY tenant_id, code
HAVING COUNT(*) > 1;
```

本次升级会使旧 Refresh Token 失效，在线用户需要重新登录。

## 五、CI

`.github/workflows/ci.yml` 在 push 到 main 或 PR 时执行：

- 后端：`gofmt -l` 检查、`go vet`、`go build`、`go test`
- 前端：`pnpm install --frozen-lockfile`、`pnpm build`（含 vue-tsc 类型检查）

端到端冒烟测试需在能连数据库的环境手动跑：`pwsh scripts/smoke-test.ps1`。
