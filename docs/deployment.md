# 部署文档

本文档介绍 Nanyi CRM 的生产部署方式。技术方案与架构见 [solution.md](solution.md)。

## 一、Docker Compose 一体化部署（推荐）

一条命令拉起完整栈：nginx 托管前端并反代 `/api`，Go 后端，MySQL 8，Redis 7。

```bash
# 1. 配置密钥（不要提交真实 .env）
cp deploy/.env.example deploy/.env
# 编辑 deploy/.env：MYSQL_Password / REDIS_Password / JWT_SIGNING_KEY

# 2. 构建并启动
docker compose -f deploy/docker-compose.prod.yml --env-file deploy/.env up -d --build

# 3. 验证
curl http://localhost/api/v1/healthz   # 经前端 nginx 反代
```

首次启动后端自动建表（AutoMigrate）并写入种子数据：

- `superAdmin / superAdmin123`（硬编码超管）
- `admin / admin123`（跨租户管理员）

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

1. `git pull` 后重新 `up -d --build`
2. 后端启动时自动执行 AutoMigrate，新表/新列/新菜单种子幂等写入，无需人工干预
3. 如有破坏性变更会在提交信息中注明，需要先备份再升级

## 五、CI

`.github/workflows/ci.yml` 在 push 到 main 或 PR 时执行：

- 后端：`gofmt -l` 检查、`go vet`、`go build`、`go test`
- 前端：`pnpm install --frozen-lockfile`、`pnpm build`（含 vue-tsc 类型检查）

端到端冒烟测试需在能连数据库的环境手动跑：`pwsh scripts/smoke-test.ps1`。
