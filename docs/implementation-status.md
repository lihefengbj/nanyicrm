# 实现进度与后续开发记录

更新时间：2026-09-20

本文档用于记录当前安全、AI 治理、数据生命周期、迁移和测试能力的实现状态。后续开发前先阅读本文件，避免重复实现或误用生产配置。

## 一、当前状态总览

| 能力 | 状态 | 主要入口 |
| --- | --- | --- |
| AI 费用估算 | 已完成 | `GET /api/v1/crm/intent/metrics` |
| AI 模型质量门禁 | 已完成 | 管理端「系统管理 → AI模型治理」 |
| 模型切换审计 | 已完成 | `sys_ai_model_change` |
| 模型灰度与稳定分流 | 已完成 | `sys_ai_model_config`、输入哈希分桶 |
| 模型激活与回滚 | 已完成 | 模型治理页面及治理 API |
| AI 数据保留、归档、清理 | 已完成 | `llm.retention_days`、`llm.archive_days` |
| 生产版本化迁移 | 已完成 | `backend/migrations/`、嵌入式 `golang-migrate` |
| 异常登录告警 | 已完成 | Redis 计数 + Webhook 去重告警 |
| 集中监控 | 已完成 | `/livez`、`/readyz`、`/metrics` |
| HttpOnly 会话 Cookie | 已完成 | Access/Refresh Cookie |
| 真实 MySQL/Redis 集成测试 | 已接入 | `backend/integration/`、CI 服务容器 |
| AI Playwright E2E | 已接入 | `frontend/e2e/ai-operations.spec.ts` |

## 二、AI 模型治理

### 使用流程

1. 进入「系统管理 → AI模型治理」。
2. 新增候选模型配置。
3. 执行固定样本质量门禁。
4. 质量门禁通过后，设置 1～99% 灰度。
5. 观察指标后激活模型。
6. 出现异常时执行回滚。

所有创建、门禁、灰度、激活和回滚动作会写入 `sys_ai_model_change`。

主要代码：

- `backend/internal/modules/crm/model_governance.go`
- `backend/internal/model/models.go`
- `backend/migrations/000013_governance_archive.up.sql`
- `frontend/src/views/system/ai-model/index.vue`

运行时会记录配置模型、实际模型、配置版本、Prompt 版本和 Provider 请求 ID，便于审计和问题定位。

## 三、AI 费用估算与数据生命周期

费用统计接口：

```text
GET /api/v1/crm/intent/metrics
```

支持：

- 输入命中缓存 Token；
- 输入未命中缓存 Token；
- 输出 Token；
- idle/peak/unified 计费模式；
- 按 Provider、模型、配置版本和 Prompt 版本分组；
- CNY 等配置货币的费用估算。

配置位置：

```yaml
llm:
  retention_days: 30
  archive_days: 365
```

清理任务服务启动后执行一次，之后每天执行一次：

- 超过 `retention_days` 的分析记录先归档；
- 热表中的原始输入和结果快照清理；
- 不再被当前意向引用的历史分析物理删除；
- 归档表超过 `retention_days + archive_days` 后删除；
- 客户删除时同步清理其 AI 数据。

主要代码：

- `backend/internal/retention/intent.go`
- `backend/cmd/purge-intent-data/main.go`
- `crm_customer_intent_analysis_archive`

## 四、生产迁移模式

### 开发环境

```yaml
app:
  env: dev
  auto_migrate: true
```

服务启动时允许使用 GORM `AutoMigrate`。

### 生产环境

```yaml
app:
  env: prod
  auto_migrate: false
```

服务启动时执行：

```go
database.RunVersionedMigrations(db)
```

迁移脚本通过 `embed.FS` 编译进后端二进制，不依赖运行时访问源码目录。迁移状态保存在 MySQL 的 `schema_migrations` 表。

查看状态：

```sql
SELECT version, dirty FROM schema_migrations;
```

新增表结构时必须创建新的递增迁移，例如：

```text
backend/migrations/000014_example.up.sql
backend/migrations/000014_example.down.sql
```

已发布迁移不得修改；需要修复时新增更高版本迁移。

## 五、生产配置注意事项

生产模式会快速拒绝以下配置：

- 默认 JWT signing key；
- 默认管理员密码；
- 空或过短监控 Token；
- 无效 MySQL DSN；
- LLM 已启用但缺少 API Key、模型或 BaseURL；
- 生产 LLM BaseURL 不是 HTTPS。

本地验证生产迁移时，建议使用独立数据库和独立配置文件，不要直接覆盖日常开发数据库。

## 六、监控与告警

健康检查：

```text
GET /livez
GET /readyz
GET /healthz
```

Prometheus 指标：

```text
GET /metrics
Header: X-Metrics-Token: <metrics-token>
```

异常登录告警配置：

```yaml
alerts:
  enabled: true
  webhook_url: "https://your-alert-endpoint"
  login_failure_threshold: 5
  window: 10m
```

Redis 负责失败次数统计和告警去重，避免多实例重复发送告警。

## 七、会话安全

当前登录方式：

- Access Token：HttpOnly Cookie；
- Refresh Token：HttpOnly Cookie；
- `Secure`：生产环境开启；
- `SameSite=Lax`；
- 前端不再把长期 Token 写入 `localStorage`；
- 兼容清理旧版本遗留的 localStorage Token；
- Refresh Token 在 Redis 中原子轮换并支持注销吊销。

相关代码：

- `backend/internal/modules/system/auth.go`
- `backend/internal/middleware/jwt.go`
- `frontend/src/api/request.ts`
- `frontend/src/store/user.ts`

## 八、验证记录

最近一次代码验证：

```text
go test ./...       通过
go vet ./...       通过
pnpm build         通过
git diff --check   通过
```

真实 MySQL/Redis 测试已加入 CI，并使用独立 MySQL 8 与 Redis 7 服务容器执行。进行本地真实集成测试时，应确保测试数据库可访问、迁移状态不是 dirty，并避免直接使用生产业务库。

## 九、后续开发清单

- [ ] 在真实部署环境完成一次全量迁移演练，并记录 `schema_migrations` 版本。
- [ ] 为 Prometheus 配置正式采集目标和告警规则。
- [ ] 配置正式异常登录 Webhook，并验证去重行为。
- [ ] 为 AI 模型治理增加更丰富的固定样本、质量阈值和人工审批流。
- [ ] 为归档数据增加独立归档存储或离线备份策略。
- [ ] 将费用字段从 `float64` 迁移为定点金额或整数分。
- [ ] 为关键治理操作增加更细粒度的角色和审批权限。
- [ ] 在 CI 中执行完整浏览器依赖安装并保存 E2E 报告。
