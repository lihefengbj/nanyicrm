# 菜单管理功能设计

本文档补齐系统管理模块中缺失的"菜单管理"功能，作为后续开发依据。现状：菜单数据由后端 seed 硬编码写入（`internal/seed/seed.go`），后端仅有 `GET /system/menu/tree` 只读接口，前端无菜单管理页面，新增菜单必须改代码重新 seed。本文档定义菜单的完整 CRUD、约束规则与前端页面设计。

## 1. 功能范围

- 菜单树查询（已有，补充按条件过滤）
- 新增菜单 / 目录 / 按钮权限
- 编辑菜单（含移动父级）
- 删除菜单（含子级校验）
- 启停、显示隐藏控制
- 前端：系统管理下新增"菜单管理"页面（树形表格 + 编辑对话框）
- 受影响方：角色管理（菜单授权树）、登录后动态路由生成

## 2. 数据模型

复用现有 `sys_menu` 表（`internal/model` 中 `SysMenu`），无需迁移变更：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| id | bigint | 主键 |
| parentId | bigint | 父菜单 ID，0 为根 |
| title | varchar(64) | 菜单/按钮名称 |
| type | tinyint | 1 目录，2 菜单，3 按钮 |
| path | varchar(128) | 路由路径（目录/菜单必填） |
| component | varchar(128) | 前端组件路径，如 `system/user/index`（仅菜单） |
| perms | varchar(128) | 权限标识，如 `system:user:list`（菜单/按钮必填） |
| icon | varchar(64) | 图标（目录/菜单） |
| sort | int | 同级排序，升序 |
| visible | tinyint | 1 显示，0 隐藏（隐藏仍可路由访问） |
| status | tinyint | 1 启用，0 停用（停用即不可访问） |
| createdAt / updatedAt / deletedAt | - | 软删除 |

字段约束按类型区分：

| 字段 | 目录(1) | 菜单(2) | 按钮(3) |
| --- | --- | --- | --- |
| path | 必填，以 `/` 开头 | 必填 | 留空 |
| component | 留空 | 必填 | 留空 |
| perms | 可空 | 必填 | 必填 |
| icon | 建议填 | 建议填 | 留空 |
| 允许的父级 | 根 / 目录 | 根 / 目录 | 菜单 |

## 3. 接口设计

统一前缀 `/api`，响应结构 `{ code, message, data }`，与现有 system 模块一致。所有接口需登录，且通过 `RequirePerm` 校验对应权限标识。

### 3.1 菜单树查询（已有）

`GET /system/menu/tree`，权限 `system:menu:list`

- 返回完整树（含 `children` 嵌套），按 `sort` 升序。
- 支持可选查询参数：`title`（模糊）、`status`（精确），命中节点时保留其祖先链，便于树形展示定位。
- 本接口同时被角色授权弹窗复用，保持现有返回结构不变。

### 3.2 新增菜单

`POST /system/menu`，权限 `system:menu:create`

请求体：

```json
{
  "parentId": 0,
  "title": "客户管理",
  "type": 2,
  "path": "/crm/customer",
  "component": "crm/customer/index",
  "perms": "crm:customer:list",
  "icon": "user",
  "sort": 0,
  "visible": 1,
  "status": 1
}
```

校验规则：

- `title` 必填，长度 ≤ 64
- `type` 必填，取值 1/2/3
- `parentId` 必须存在；类型与父级关系须满足 §2 的约束（目录下可挂目录/菜单，菜单下只能挂按钮，按钮不可有子级）
- `perms` 非空时全局唯一（现存数据范围内），冲突返回 3xxx 业务错误
- 按类型校验 `path` / `component` 必填性
- 未传字段使用默认值：`sort=0`、`visible=1`、`status=1`

### 3.3 编辑菜单

`PUT /system/menu/{id}`，权限 `system:menu:update`

- 请求体同新增，全量更新。
- `parentId` 不允许指向自身或自身的子孙节点（防止成环），服务端递归校验。
- 修改 `perms` 时做唯一性校验（排除自身）。
- 类型变更受限：存在子级的节点不允许变更 `type`。

### 3.4 删除菜单

`DELETE /system/menu/{id}`，权限 `system:menu:delete`

- 存在子级时拒绝删除，提示先删除子级。
- 已被角色引用（`sys_role_menu` 存在记录）时拒绝删除，提示先解除角色授权。
- 删除为软删除（`deleted_at`），保留历史审计能力。

### 3.5 权限标识汇总

| 标识 | 用途 |
| --- | --- |
| system:menu:list | 查看菜单树 / 菜单管理页 |
| system:menu:create | 新增 |
| system:menu:update | 编辑、启停 |
| system:menu:delete | 删除 |

## 4. 后端实现要点

- 位置：`internal/modules/system/menu.go`，在现有 `MenuHandler` 上扩展 `Create / Update / Delete` 方法，风格对齐 `dept.go`。
- 路由：在 `internal/router` 现有 system 分组下补注册，沿用 `middleware.RequirePerm`。
- 成环校验：更新时沿新 `parentId` 向上遍历，命中自身 ID 即拒绝；菜单层级有限（≤ 5 层），简单循环即可，无需闭包表。
- 操作日志：新增/编辑/删除走现有操作日志中间件，模块记为 `system-menu`。
- seed 调整：在 `seed.go` 中补充"菜单管理"菜单项（挂在 `/system` 下，perms 为上述四个标识），`ensure*` 系列函数保持幂等，老库启动时自动补齐。

## 5. 前端设计

页面：`frontend/src/views/system/menu/index.vue`，API 封装追加到 `frontend/src/api/system.ts`（`createMenu / updateMenu / deleteMenu`），类型补入 `src/types/api.ts`。

布局与交互（对齐部门管理页）：

- 主体为树形表格（`el-table` + `tree-props`），列：菜单名称、图标、类型（目录/菜单/按钮 tag）、路由路径、组件、权限标识、排序、状态（开关）、可见、操作（新增下级/编辑/删除）。
- 顶部工具栏：标题搜索框、状态下拉、新增按钮、"展开/折叠"切换。
- 新增/编辑使用对话框表单：类型单选联动显隐字段（选按钮时隐藏 path/component/icon），父级用 `el-tree-select` 选择，perms 唯一性由后端报错提示。
- 删除前二次确认；后端返回"存在子级/已被角色引用"时以消息提示原样展示。
- 编辑保存后刷新树；若改动影响当前登录用户可见菜单，提示重新登录或触发路由刷新（前端调用现有动态路由重建逻辑）。

## 6. 影响面与注意事项

- **角色授权**：角色管理弹窗复用菜单树接口，无需改动；菜单删除前强制解除引用，避免脏数据。
- **动态路由**：登录后按用户权限菜单生成路由，新增/停用菜单即时生效于下一次登录；`visible=0` 的菜单不进侧边栏但路由保留。
- **安全**：所有写接口必须走 `RequirePerm`；`perms` 字符串是接口鉴权的唯一依据，禁止前端绕过。
- **多租户**：菜单为平台级全局数据（`sys_menu` 无 tenant_id），仅平台超管可管理，租户侧不出现该页面入口。

## 7. 验收标准

- 可在页面上完成目录/菜单/按钮的增删改查与启停，约束规则（§2、§3）全部生效。
- 删除有子级或已被角色引用的菜单被正确拦截并提示。
- 新增菜单并在角色中授权后，对应账号重新登录即可看到入口并访问；停用后立即不可访问。
- seed 幂等：全新库初始化与存量库启动均能得到"菜单管理"菜单项且不产生重复数据。
