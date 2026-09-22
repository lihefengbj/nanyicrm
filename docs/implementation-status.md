# 实现进度与后续开发记录

更新时间：2026-09-22

本文档用于记录当前安全、AI 治理、数据生命周期、迁移和测试能力的实现状态。后续开发前先阅读本文件，避免重复实现或误用生产配置。

## 一、当前状态总览

| 能力 | 状态 | 主要入口 |
| --- | --- | --- |
| AI 费用估算 | 已完成 | `GET /api/v1/crm/intent/metrics` |
| AI 模型质量门禁 | 已完成 | 管理端「系统管理 → AI模型治理」；四类固定样本、等级匹配和字段完整率 |
| 动态 Prompt 治理 | 已完成 | `sys_ai_prompt`、质量门禁、输入哈希灰度、激活、回滚和审计 |
| 模型切换审计 | 已完成 | `sys_ai_model_change` |
| 模型灰度与稳定分流 | 已完成 | `sys_ai_model_config`、输入哈希分桶 |
| 模型激活与回滚 | 已完成 | 模型治理页面及治理 API |
| AI 模型凭证管理 | 已完成 | `sys_ai_credential`；AES-GCM 加密存储、末四位掩码、模型配置 `credentialId` 绑定 |
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
3. 对草稿/已通过/历史配置选择「绑定凭证」，可绑定凭证管理中的 Key，也可解除绑定回退到 `LLM_API_KEY`。
4. 执行固定样本质量门禁。样本覆盖高、中、低、未知四类意向，要求四条结果均可解析且字段合法、意向等级全部匹配，字段完整率至少 75%。
5. 质量门禁通过后，设置 1～99% 灰度。
6. 观察指标后激活模型。
7. 出现异常时执行回滚。
8. 在每个配置的「审计记录」中查看创建、凭证绑定、门禁、灰度、激活和回滚动作。

所有创建、门禁、灰度、激活和回滚动作会写入 `sys_ai_model_change`。

API Key 通过「凭证管理」保存或轮换，服务端只返回凭证名称、Provider 和末四位。模型配置绑定凭证后，质量门禁与正式调用都会使用该凭证；未绑定凭证的历史配置继续使用 `LLM_API_KEY`。因此全局 `LLM_API_KEY` 现在是兼容回退项，不再是数据库凭证模式的启动必填项。

主要代码：

- `backend/internal/modules/crm/model_governance.go`
- `backend/internal/model/models.go`
- `backend/migrations/000013_governance_archive.up.sql`
- `frontend/src/views/system/ai-model/index.vue`

运行时会记录配置模型、实际模型、配置版本、Prompt 版本和 Provider 请求 ID，便于审计和问题定位；分析完成或失败时还会输出结构化调用日志，包含 Prompt/模型版本、输入哈希、请求 ID、状态、错误类型和耗时，不记录完整客户输入、Prompt 正文或 API Key。

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
- [ ] 为 AI 模型治理增加更多业务领域固定样本和人工审批流；当前已完成四类基础样本、等级匹配和字段完整率门槛。
- [ ] 将分析去重键扩展为 `input_hash + prompt_version + model_config_version`，当前仍按 `input_hash` 去重。
- [ ] 为归档数据增加独立归档存储或离线备份策略。
- [ ] 将费用字段从 `float64` 迁移为定点金额或整数分。
- [ ] 为关键治理操作增加更细粒度的角色和审批权限。
- [ ] 在 CI 中执行完整浏览器依赖安装并保存 E2E 报告。

本轮（2026-09-22）补充：

- 质量门禁从单个连通性样本升级为高/中/低/未知四类固定样本。
- 质量门禁指标记录样本数、合法结果数、等级匹配率、字段完整率、总 Token、耗时和标准化失败原因。
- 新增质量门禁单测，覆盖全样本通过、等级漂移拦截和鉴权失败分类。
- 管理端模型治理增加审计记录弹窗，关键治理 E2E 增加审计入口验证。
- 动态 Prompt 治理已落地：新增 `sys_ai_prompt`、`sys_ai_prompt_change` 及 `000015_ai_prompt_governance` 迁移。
- Prompt 内容按规范化内容 SHA-256 派生不可重复版本号；支持草稿编辑、四类固定样本质量门禁、active/canary 稳定分流、激活、回滚和独立审计。
- OpenAI 兼容调用已拆分为代码固定 JSON 契约段与数据库研判段，分析历史记录实际 Prompt 版本；管理端 AI 模型治理页增加 Prompt 治理区域，客户分析历史展示 Prompt 版本、模型配置版本和实际模型。
- 调用失败时仍保留本次已选择的 Prompt/模型元数据；结构化日志记录非敏感调用元数据。
- 当前分析去重仍仅使用 `input_hash`，Prompt/模型版本组合去重列为后续任务。
- 本轮验证：`go test ./...`、`go vet ./...`、`pnpm build`、Prompt/模型治理定向 Playwright 均通过；完整 AI E2E 使用单 worker 时 6 passed、1 skipped（真实 AI 调用按开关跳过）。
