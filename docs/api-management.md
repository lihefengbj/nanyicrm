# API 接口管理功能设计

本文档补齐 RBAC 体系中的 API 层空缺。现状：接口鉴权依赖路由注册处硬编码的 `middleware.RequirePerm(db, "system:user:list")`，系统内没有接口清单数据，无法在页面上查看"有哪些接口、各接口需要什么权限"，也无法按 API 粒度授权。本文档定义 `sys_api` 接口注册表与配套管理功能。

## 1. 功能定位与边界

- **核心目标**：接口清单可视化（自动采集、页面可查），并让接口与权限标识（perms）的关联从代码注释升级为数据
- **不做的事**：不替代现有 `RequirePerm` 鉴权机制；角色授权仍通过菜单/按钮的 perms 完成，本功能第一阶段只读展示，不做"按 API 逐个授权给角色"（该能力依赖开放 API 场景出现后再扩展，见 §7）
- 接口数据为平台级全局数据，仅平台超管可见管理入口

## 2. 数据模型

新增 `sys_api` 表（golang-migrate 新增迁移脚本）：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| id | bigint | 主键 |
| method | varchar(8) | HTTP 方法：GET/POST/PUT/DELETE |
| path | varchar(255) | 路由路径，含 `:id` 风格参数，如 `/api/v1/system/user/:id` |
| handler | varchar(128) | 处理函数名，如 `system.UserHandler.Update` |
| title | varchar(64) | 接口名称（人工维护，如"用户-编辑"） |
| module | varchar(32) | 所属模块：system / crm / auth / dashboard |
| perms | varchar(128) | 所需权限标识，来自 `RequirePerm` 参数；公开接口为空 |
| status | tinyint | 1 启用，0 停用（预留，一期不用于拦截） |
| createdAt / updatedAt / deletedAt | - | 软删除 |

唯一约束：`uk_method_path (method, path)`（软删除行通过代码层保证不冲突）。

## 3. 接口采集（核心机制）

不依赖人工录入，服务启动时自动同步：

1. Gin 引擎构建完成后，遍历 `r.Routes()` 得到全部 `method + path + handler` 三元组。
2. perms 的获取：router 包维护一个显式的 `apiRegistry` 切片，注册路由时同时登记 `{method, path, perms}`，启动尾声调用 `SyncApis(db, apiRegistry)`。
3. `SyncApis` 逻辑（幂等）：
   - 注册表中有、库中无 → 插入（`title` 默认空，待人工补）
   - 库中有、注册表中无 → 标记软删除（接口已从代码移除，保留历史）
   - 两边都有、perms 变化 → 更新
   - `title` 一旦被人工维护过，同步过程不覆盖
4. 排除项：`/healthz`、`/swagger/**`、静态资源不入库。

该设计保证"代码即事实源"，页面永远与真实路由一致，杜绝手工登记漂移。

## 4. 接口设计

统一前缀 `/api/v1`，均走 JWT + `RequirePerm`：

| 方法 | 路径 | 权限标识 | 说明 |
| --- | --- | --- | --- |
| GET | /system/api | system:api:list | 分页/条件查询（method、path 模糊、module、perms） |
| GET | /system/api/all | system:api:list | 全量列表（按 module 分组展示用） |
| PUT | /system/api/:id | system:api:update | 仅允许编辑 `title`；method/path/perms 由代码同步，页面只读 |
| POST | /system/api/sync | system:api:update | 手动触发一次同步（调试用，常规由启动时自动完成） |

不提供新增/删除接口：接口的生死由代码决定，页面不可手工增删。

## 5. 前端设计

页面：`frontend/src/views/system/api/index.vue`，挂在"系统管理"下（菜单项由 seed 的 `ensure*` 幂等函数补充，perms：`system:api:list` / `system:api:update`）。

- 顶部工具栏：请求方法下拉、路径搜索框、模块下拉、手动同步按钮
- 表格列：方法（tag 按 GET 绿/POST 蓝/PUT 橙/DELETE 红着色）、路径、接口名称、所属模块、所需权限（perms，空显示"公开"）、更新时间、操作（编辑名称）
- 支持按 module 分组展示的切换视图（平铺表格 / 分组树）
- 编辑对话框只含 `title` 一个字段，其余只读展示

## 6. 后端实现要点

- 模型 `SysApi` 加入 `internal/model`；迁移脚本编号顺延（当前最大 000003，新增 `000004_sys_api`）
- `internal/modules/system/api.go` 新建 `ApiHandler`，含 `List / All / UpdateTitle / Sync`
- router 改造最小化：新增 `reg` 辅助函数包装现有注册（登记 + 挂路由），逐行替换现有 `authed.GET/POST/PUT/DELETE` 调用，保持路由行为不变
- 启动顺序：`router.New` 返回引擎与 registry → `main.go` 在数据库就绪后调用 `SyncApis`
- 同步失败不阻断启动，记 error 日志（接口清单降级为下次启动再同步）

## 7. 后续扩展（不在本期范围）

- **按 API 授权**：`sys_role_api` 关联表 + 鉴权中间件从"查 perms"升级为"查 role → api 直连"，用于开放 API / 第三方接入场景
- **接口启停拦截**：`status=0` 时在网关中间件直接拒绝请求，作为紧急下线开关
- **调用统计**：结合 `sys_oper_log` 按接口聚合 QPS、耗时、错误率，形成接口监控页

## 8. 验收标准

- 服务启动后 `sys_api` 表自动包含全部 `/api/v1/**` 接口（含 method、path、perms），与 `r.Routes()` 输出一致
- 删除代码中某接口后重启，库中对应记录被软删除；修改某接口 perms 后重启，库中记录同步更新
- 页面上可按条件检索接口，编辑接口名称后同步不覆盖；无 `system:api:list` 权限的账号访问页面接口返回 403
- seed 幂等补充"接口管理"菜单项；新迁移脚本在全新库与存量库上均可执行
