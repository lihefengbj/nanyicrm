# M6 动态菜单与动态路由设计

本文档规划把前端侧边栏与路由从硬编码改造为按后端菜单数据动态生成，使"菜单管理"功能形成闭环：在页面上新增/调整菜单后，授权账号下次登录即可看到入口并访问，无需改前端代码。

## 1. 现状与问题

- 侧边栏在 `frontend/src/layout/index.vue` 中硬编码每个菜单项，`router/index.ts` 静态声明全部路由（含 `meta.perm`）
- `sys_menu` 数据目前只服务于权限校验（perms 字符串）和角色授权弹窗
- 后果：菜单管理页的增删改不影响实际导航；每加一个页面要同时改 DB 菜单、layout、router 三处，容易漏（M5 就漏了侧边栏）

## 2. 目标与边界

- 侧边栏完全由后端菜单树渲染（目录 → `el-sub-menu`，菜单 → `el-menu-item`，按钮不显示）
- 路由按当前用户可见菜单动态注册（`router.addRoute`），`visible=0` 的菜单注册路由但不进侧边栏
- 权限指令、按钮级控制仍走现有 `hasPerm`，不变
- 不改动的部分：登录页/404 等公共路由保持静态；后端鉴权逻辑不变

## 3. 后端改动

### 3.1 Profile 返回菜单树

`GET /auth/profile` 响应增加 `menus` 字段：当前用户角色授权的、**status=1** 的目录+菜单（type 1/2），按 `sort` 升序组装成树。按钮（type 3）不下发，按钮权限继续走 `perms` 数组。

- superAdmin（含硬编码账号）：返回全部启用菜单，绕过角色关联
- 复用 `buildMenuTree`；只查一次 `sys_menu` JOIN `sys_role_menu`，去重
- `visible=0` 的菜单包含在返回中（前端用于注册路由），由前端决定不进侧边栏

### 3.2 一致性要求

- 菜单的 `component` 字段必须与 `frontend/src/views/` 下的相对路径一致（如 `system/user/index`），菜单管理页保存时前端做格式提示，后端不校验文件是否存在（构建期无法感知）
- 目录的 `path` 以 `/` 开头，菜单的 `path` 为相对路径，拼接规则：父目录 path + `/` + 菜单 path，与现有静态路由保持一致（`/system` + `user` → `/system/user`）

## 4. 前端改动

### 4.1 组件映射

`router/index.ts` 中用 Vite 的 glob 导入建立组件表：

```ts
const viewModules = import.meta.glob('@/views/**/*.vue')
function resolveComponent(component: string) {
  return viewModules[`/src/views/${component}.vue`]
}
```

未匹配到组件时渲染一个内置的"页面未实现"占位组件，避免整站白屏。

### 4.2 动态注册

- 保留静态骨架：`/login`、`/`（Layout 壳 + 动态 children）、`/:pathMatch(.*)*` 404
- 路由守卫中 profile 拉取成功后，遍历 `profile.menus`：目录跳过（其本身无可访问页面），菜单注册为 Layout 的 child，`path` 用拼接后的完整路径，`meta: { title, perm: menu.perms, icon }`
- 用 `router.hasRoute(name)` 防重复注册；登出时移除动态路由（`router.removeRoute`）
- `meta.perm` 校验逻辑保留，作为绕过 URL 直接访问的第二道闸（第一道是后端 RequirePerm）

### 4.3 侧边栏渲染

`layout/index.vue` 的静态 `<el-menu-item>` 全部删除，改为递归组件渲染 `profile.menus`：

- type=1 目录 → `el-sub-menu`（`index` 用 path），type=2 且 `visible=1` → `el-menu-item`（`index` 用完整路径）
- 图标：`icon` 字段存 Element Plus 图标名，前端 `component :is` 动态渲染，未知图标回退为默认图标
- 排序、层级、隐藏全部由数据驱动，不再有任何写死的菜单名

### 4.4 类型与 Store

- `types/api.ts`：`UserInfo` 增加 `menus: Menu[]`
- `store/user.ts`：`logout()` 时触发动态路由清理（通过 router 模块导出的 reset 函数，避免 store 直接依赖 router 实例造成循环引用）

## 5. 兼容与迁移

- 现有 seed 菜单数据（component、path 写法）已满足拼接规则，无需数据迁移；上线后 admin/superAdmin 重新登录即生效
- 菜单管理页增加一个提示：component 填写的值必须对应 `src/views/` 下的真实文件，否则页面显示"未实现"占位
- 回滚方案：保留静态路由表一个版本周期，通过 `import.meta.env` 开关切换动态/静态，确认稳定后删除静态表

## 6. 验收标准

- 在菜单管理页新增一个菜单（component 指向已有页面文件）、给角色授权后，该角色账号重新登录可直接看到并访问，全程不改前端代码
- 菜单设为隐藏后侧边栏消失但直接输 URL 可访问；停用后 URL 访问被路由守卫拦回工作台
- 删除某菜单授权后，对应账号刷新即看不到入口，直接输 URL 被拦截
- component 填错时页面显示"页面未实现"占位而非白屏
- 冒烟测试 51 项断言保持全通过（后端行为不变）
