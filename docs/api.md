# Meteor CMS 接口文档

服务默认监听 `http://localhost:8888`，本文按「模块 + 方法 + 路径」列出全部接口。

## 通用约定

### 响应格式

```json
{ "code": 0, "msg": "success", "data": {} }
```

- `code = 0` 成功
- `code > 0` 业务错误，`msg` 为可直接展示给用户的提示
- 完整错误码见 [`internal/common/error_code.go`](../internal/common/error_code.go)

### 认证

受保护接口需携带：

```
Authorization: Bearer <token>
```

未携带返回 `2001 请先登录`；token 无效或已登出返回 `2009 无效的token`；会话上下文缺失返回 `2004 用户会话已过期，请重新登录`。

### 鉴权

除以下接口外，其余受保护接口均由权限中间件按 `method + api_path` 匹配 `Type=3` 权限记录后校验：

- `POST /admin/logout`
- `GET /admin/test-token`
- `PUT /admin/admin-users/me/change-password`
- `GET /admin/profile`
- `PUT /admin/profile`
- `GET /admin/profile/permissions`

超级管理员跳过全部鉴权与数据范围过滤。

### 分页

`page` 小于 1 按 1 处理；`page_size` 小于 1 取接口默认值，**大于 100 一律截断为 100**，响应中的 `page_size` 为实际生效值。

---

## 公共接口（无需认证）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/captcha` | 获取图形验证码；`LOGIN_CAPTCHA_ENABLED=false` 时返回 `enabled=false` |
| POST | `/admin/login` | 登录；验证码开启时需提交 `captcha_id` 与 `captcha` |
| POST | `/admin/register` | 注册管理员 |
| POST | `/admin/forgot-password` | 申请密码重置令牌（账号不存在也返回成功，防账号枚举） |
| POST | `/admin/reset-password` | 用令牌设置新密码，令牌一次性、1 小时有效 |
| POST | `/admin/refresh` | 用 refresh_token 换取新的 access_token |
| GET | `/healthz` | 存活探活：返回 `database` / `redis` 状态；数据库不可用返回 503，Redis 故障仅标 `degraded`（有内存降级） |
| GET | `/readyz` | 就绪探针：额外校验 `schema_migrations` 版本，返回 `ready` / `version` / `schema`；版本不匹配返回 503。与 `/healthz` 一样不走统一响应信封，直接按 HTTP 状态码判断 |

### 登录

```json
// 请求
{ "username": "admin", "password": "123456", "captcha_id": "xxx", "captcha": "1234" }
// 响应
{
  "code": 0, "msg": "success",
  "data": {
    "access_token": "eyJhbGci...（短期，默认 24h）",
    "refresh_token": "eyJhbGci...（长期，默认 7d，用于刷新）"
  }
}
```

失败场景：`4001` 参数缺失、`2006` 验证码错误、`2010` 用户名或密码错误、`2007` 账号未激活、`2011` 账号已被临时锁定。

> **登录失败锁定**：连续登录失败次数达到 `Login.MaxFailCount`（默认 5）后，账号被锁定 `Login.LockMinutes`（默认 15）分钟；锁定期间登录返回 `2011` 并提示剩余锁定分钟数。登录成功后自动清零失败计数。该机制仅针对已存在的账号，账号不存在时不计数（避免被用于探测账号是否存在）。
>
> **令牌刷新**：access_token 过期后，用登录时拿到的 refresh_token 调用 `POST /admin/refresh` 换取新的 access_token，无需重新登录。refresh_token 与 access_token 使用不同密钥签发，且携带 `token_type=refresh` 标识，不可互换。

### 刷新令牌

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/admin/refresh` | 用 refresh_token 换取新的 access_token |

```json
// 请求
{ "refresh_token": "eyJhbGci..." }
// 响应
{ "code": 0, "msg": "success", "data": { "access_token": "eyJhbGci..." } }
```

> 刷新令牌使用独立密钥（`Auth.RefreshSecret`）与更长有效期（`Auth.RefreshExpire`，默认 7 天）。用户被删除或禁用时刷新立即失效。刷新接口本身**不要求** access_token 鉴权。

> **刷新令牌轮换与并发宽限期**：每次刷新会签发新的 refresh token 并将旧令牌写入黑名单，防止长期复用。为避免并发请求同时用同一个旧令牌刷新时被误判为失效，服务端保留 `Auth.RefreshGraceSeconds`（默认 5 秒）宽限期：刚被轮换的旧令牌在此窗口内仍被接受，超出则判定为盗用重放并拒绝。
> 注意：登出同样会把 refresh token 写入黑名单，因此在登出后的宽限期内理论上存在极短的有效窗口（客户端已即时清除令牌，风险极低）。如需零宽限，可将 `RefreshGraceSeconds` 设为 0 并配合前端的刷新串行化。

> **前端无感刷新**：access_token 过期（后端返回 `code=2001`）时，前端自动用 refresh_token 调用 `/admin/refresh` 换新 token 并重试原请求（并发请求排队共享一次刷新）；刷新接口自身失败或无 refresh_token 时才跳转登录页。前端登出（`POST /admin/logout`）会在请求头携带 `X-Refresh-Token`，后端将其与 access_token 一并加入黑名单（TTL = `Auth.RefreshExpire`），此后该 refresh_token 无法再换发 access_token，实现真正的"登出即失效"。

### 图形验证码

```json
{ "code": 0, "msg": "success", "data": { "captcha_id": "xxx", "image": "data:image/png;base64,..." } }
```

### 重置密码

```json
// 请求
{ "token": "邮件中的令牌", "new_password": "newpassword123" }
// 失败码：3012 令牌无效 / 3013 令牌已过期 / 4001 新密码少于 6 位
```

---

## 个人中心

| 方法 | 路径 | 权限码 | 说明 |
| --- | --- | --- | --- |
| GET | `/admin/profile` | 免鉴权 | 当前登录管理员的个人信息 |
| PUT | `/admin/profile` | 免鉴权 | 更新昵称/邮箱/手机，未传字段保持不变 |
| GET | `/admin/profile/permissions` | 免鉴权 | 当前用户的菜单树与权限码 |
| PUT | `/admin/admin-users/me/change-password` | 免鉴权 | 修改自己的密码 |
| POST | `/admin/logout` | 免鉴权 | 登出，token 进入黑名单 |
| GET | `/admin/test-token` | 免鉴权 | 校验 token 是否有效 |

### GET `/admin/profile`

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "id": 1, "username": "admin", "nickname": "超级管理员",
    "email": "admin@example.com", "phone": "", "avatar": "",
    "is_super": true, "dept_id": 0,
    "last_login_at": "2026-10-30T10:00:00+08:00",
    "created_at": "2026-10-01T10:00:00+08:00"
  }
}
```

### PUT `/admin/profile`

```json
{ "nickname": "新昵称", "email": "new@example.com", "phone": "13800138000" }
```

邮箱已被他人使用时返回 `3008 邮箱已被注册`。

### GET `/admin/profile/permissions`

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "is_super": false,
    "menus": [
      { "id": 2, "name": "管理员管理", "path": "/system/users",
        "component": "views/admin/user/index.vue", "icon": "user",
        "permission": "system:admin_user:list", "sort": 10, "children": [] }
    ],
    "permissions": ["system:admin_user:list", "system:admin_user:add"]
  }
}
```

> `menus` 为已按权限过滤并按 `sort` 升序的菜单树，前端侧边栏直接据此渲染（动态菜单），
> `icon` 是语义键（如 `user / dict / setting`），由前端映射为具体图标组件。
> 超级管理员返回全部菜单，且 `is_super=true` 时后端鉴权与前端守卫均直接放行。
> 登录成功后前端只需拉取一次（用 `accessLoaded` 标记），零权限用户也不会重复请求。

---

## 管理员管理

列表接口按当前用户的数据权限收敛结果，详见 [数据权限说明](#数据权限)。

| 方法 | 路径 | 权限码 | 说明 |
| --- | --- | --- | --- |
| GET | `/admin/admin-users` | `system:admin_user:list` | 管理员列表（分页、按用户名/手机/状态筛选） |
| GET | `/admin/admin-users/:id` | `system:admin_user:detail` | 管理员详情 |
| POST | `/admin/admin-users/create` | `system:admin_user:add` | 新增管理员 |
| PUT | `/admin/admin-users/:id` | `system:admin_user:edit` | 更新资料（局部更新，未传字段不变） |
| DELETE | `/admin/admin-users/:id` | `system:admin_user:delete` | 删除管理员（软删除），不可删自己与超管 |
| PUT | `/admin/admin-users/:id/disable` | `system:admin_user:disable` | 禁用，不可禁用自己与超管 |
| PUT | `/admin/admin-users/:id/enable` | `system:admin_user:enable` | 启用 |
| PUT | `/admin/admin-users/:id/bind-department` | `system:admin_user:bind_dept` | 绑定部门 |
| PUT | `/admin/admin-users/reset-password` | `system:admin_user:reset_pwd` | 重置他人密码，仅超管，目标不能是超管 |
| GET | `/admin/admin-users/:id/roles` | `system:admin_user:role_list` | 查询已分配角色 |
| PUT | `/admin/admin-users/:id/roles` | `system:admin_user:role_assign` | 分配角色（覆盖式，事务保证） |
| DELETE | `/admin/admin-users/:id/roles/:role_id` | `system:admin_user:role_remove` | 移除单个角色 |

### 新增管理员

```json
{
  "username": "editor", "password": "123456", "nickname": "编辑",
  "email": "editor@example.com", "phone": "13800138000",
  "is_active": true, "is_super": false, "dept_id": 1
}
```

用户名重复返回 `3001 管理员已存在`，邮箱重复返回 `3008 邮箱已被注册`。

### 分配角色

```json
{ "role_ids": [1, 2] }
```

角色 ID 去重后覆盖写入，角色不存在返回 `5201 角色不存在`。

---

## 部门管理

| 方法 | 路径 | 权限码 | 说明 |
| --- | --- | --- | --- |
| GET | `/admin/departments/tree` | `system:dept:list` | 部门树（仅返回启用部门） |
| GET | `/admin/departments/:id` | `system:dept:detail` | 部门详情 |
| POST | `/admin/departments` | `system:dept:add` | 新增部门 |
| PUT | `/admin/departments/:id` | `system:dept:edit` | 更新部门 |
| DELETE | `/admin/departments/:id` | `system:dept:delete` | 删除部门（软删除），存在子部门时拒绝；删除或停用后旧的 `dept_id` 引用不再产生可见范围 |

部门树节点含 `children`，父部门未启用时其下的启用子部门不会出现在树中。

---

## 角色管理

| 方法 | 路径 | 权限码 | 说明 |
| --- | --- | --- | --- |
| GET | `/admin/roles` | `system:role:list` | 角色列表（分页、按名称/编码/状态筛选） |
| GET | `/admin/roles/:id` | `system:role:detail` | 角色详情 |
| POST | `/admin/roles` | `system:role:add` | 新增角色，含 `data_scope` 数据范围 |
| PUT | `/admin/roles/:id` | `system:role:edit` | 更新角色，系统内置角色不可改 |
| DELETE | `/admin/roles/:id` | `system:role:delete` | 删除角色，仍有关联用户时拒绝 |
| GET | `/admin/roles/:id/permissions` | `system:role:perm_list` | 角色已分配的权限 ID 列表 |
| GET | `/admin/roles/:id/permissions-with-names` | `system:role:perm_list_names` | 同上，附带权限名称 |
| PUT | `/admin/roles/:id/permissions` | `system:role:perm_assign` | 分配权限（覆盖式，事务保证） |
| GET | `/admin/roles/:id/departments` | `system:role:dept_list` | 自定义数据范围的部门列表 |
| PUT | `/admin/roles/:id/departments` | `system:role:dept_assign` | 设置部门范围，仅 `data_scope=5` 可用 |

### 分配权限

```json
{ "permission_ids": [1, 2, 3] }
```

系统内置角色返回 `5208 系统内置角色，禁止修改`；权限不存在返回 `5401 权限不存在`。

### 设置部门范围

```json
{ "dept_ids": [1, 2] }
```

非 `data_scope=5` 的角色返回 `4001 参数错误`；部门不存在或未启用返回 `5001 部门不存在`。

---

## 权限管理

| 方法 | 路径 | 权限码 | 说明 |
| --- | --- | --- | --- |
| GET | `/admin/permissions` | `system:permission:list` | 权限列表（分页，可按类型/名称筛选） |
| GET | `/admin/permissions/tree` | `system:permission:tree` | 权限树 |
| GET | `/admin/permissions/:id` | `system:permission:detail` | 权限详情 |
| POST | `/admin/permissions` | `system:permission:add` | 新增权限 |
| PUT | `/admin/permissions/:id` | `system:permission:edit` | 更新权限 |
| DELETE | `/admin/permissions/:id` | `system:permission:delete` | 删除权限，存在子权限时拒绝 |

### 权限类型

| type | 含义 | 用途 |
| --- | --- | --- |
| 1 | 菜单 | 前端路由，`/admin/profile/permissions` 据此下发菜单树 |
| 2 | 按钮 | 前端功能点，控制按钮显隐 |
| 3 | API | 后端接口鉴权，按 `method + api_path` 匹配 |

新增受保护接口后，请同步在 `internal/seed/permissions.go` 追加一条 `Type=3` 权限，并执行 `go run ./cmd/migrate` 写入，否则非超管访问会返回 `2002 权限不足`。

---

## 数据权限

角色通过 `data_scope` 声明可见数据范围：

| data_scope | 含义 |
| --- | --- |
| 1 | 全部数据 |
| 2 | 本部门 |
| 3 | 本部门及子部门 |
| 4 | 仅自己 |
| 5 | 自定义部门（读取 `admin_role_depts`） |

- 多角色取并集，任一角色为「全部数据」时不过滤
- 范围解析失败、未分配启用角色、或要求按部门过滤但未分配部门时，按最严格处理（返回空集）
- 当前仅在管理员列表接口落地

---

## 审计日志

| 方法 | 路径 | 权限码 | 说明 |
| --- | --- | --- | --- |
| GET | `/admin/logs/operations` | `system:log:operation_list` | 操作日志列表 |
| GET | `/admin/logs/logins` | `system:log:login_list` | 登录日志列表 |
| GET | `/admin/logs/operations/:id` | `system:log:operation_list` | 操作日志详情 |
| GET | `/admin/logs/logins/:id` | `system:log:login_list` | 登录日志详情 |
| GET | `/admin/logs/export` | `system:log:export` | 导出 CSV |
| GET | `/admin/logs/stats` | `system:log:stats` | 近 N 天统计 |

### GET `/admin/logs/operations`

查询后台接口调用记录，用于安全审计与问题排查。支持按操作人、模块、路径、结果、时间范围筛选。

**请求参数（query）**：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| page | int | 页码，默认 1 |
| page_size | int | 每页条数，默认 20，上限 100 |
| user_id | int64 | 操作人 ID（仅超管可指定；非超管强制只看自己） |
| username | string | 操作人账号（模糊匹配） |
| module | string | 业务模块，如「管理员管理」 |
| path | string | 请求路径（模糊匹配） |
| status | int（可选） | 0 成功 / 1 失败；**不传表示不筛选**，传 `0` 才是「只看成功」 |
| start_time | string | 起始时间，支持 `2006-01-02 15:04:05` / `2006-01-02` / RFC3339 |
| end_time | string | 结束时间；纯日期 `YYYY-MM-DD` 自动扩展到当天 `23:59:59.999999999`，含当天全部记录 |

**响应 data**：

| 字段 | 说明 |
| --- | --- |
| total | 总条数 |
| list[].module | 业务模块 |
| list[].action | 操作描述，由请求路径关键词与 HTTP 方法推断（如「新增」「审核通过」「强制下线」） |
| list[].status | 0 成功 / 1 失败 |
| list[].params | JSON / 表单参数快照（敏感字段递归脱敏、超长截断） |
| list[].query | URL query / 表单参数快照（敏感键脱敏、超长截断） |
| list[].request_id | 请求链路标识，可与登录日志及运行日志串联（响应头 `X-Request-ID`） |
| list[].biz_code | 业务响应码，0 表示成功 |
| list[].http_status | HTTP 响应状态码 |
| list[].duration_ms | 耗时（毫秒） |

> 数据权限：超管或持有 `system:log:view_all` 的审计员可查全量，其余用户只能查自己的日志；`user_id` 仅对全量可见者生效。按 `created_at` 倒序返回。
> 只有两个列表查询接口与验证码获取跳过采集，详情 / 导出 / 统计接口本身会写操作日志。
> 日志保留策略由 `Maintenance.AuditLogRetentionDays` 配置（默认 90 天，<=0 表示永久保留），后台任务分批删除并用 Redis 选主。

### GET `/admin/logs/logins`

查询登录成功与失败记录，用于账号安全审计。参数与操作日志类似（`user_id`/`username`/`status`/`start_time`/`end_time`），其中 `status` 不传表示不筛选、传 `0` 表示只看成功，纯日期型 `end_time` 含当天；按 `login_at` 倒序。

**响应 data.list[]** 字段：`id, user_id, username, ip, user_agent, status, fail_reason, request_id, login_at`。

### GET `/admin/logs/operations/:id` 与 `/admin/logs/logins/:id`

按 ID 返回单条完整记录，数据范围与列表一致；记录不存在或越权统一返回 `2004`，避免探测他人日志是否存在。权限分别复用 `system:log:operation_list` 与 `system:log:login_list`。

### GET `/admin/logs/export`

按 `type=operation|login` 导出 CSV，筛选参数 `status` / `start_time` / `end_time` 与列表同语义。响应为 `text/csv`（带 UTF-8 BOM），单次最多 10000 条；以 `= + - @` 开头的单元格会加 `'` 前缀防止 CSV 注入。权限 `system:log:export`。

### GET `/admin/logs/stats`

近 `days` 天（1-30，默认 7）操作日志聚合，返回 `total` / `success` / `failed` 与 `by_day`、`by_module`、`by_biz_code` 三组分布。数据范围与列表一致。权限 `system:log:stats`。

---

## 文件上传

| 方法 | 路径 | 权限码 | 说明 |
| --- | --- | --- | --- |
| POST | `/admin/upload` | `system:upload:file` | 上传文件（multipart/form-data，字段名 `file`） |

### POST `/admin/upload`

接收单个文件，落盘到 `public/uploads/YYYY/MM/<uuid>.<ext>`，返回可直接访问的 URL（前缀 `/uploads/`）。静态资源通过 `rest.WithFileServer` 对外暴露，无需鉴权即可 GET。

**校验规则**：
- 扩展名白名单：`.jpg .jpeg .png .gif .webp`（配置于 `etc/cms-api.yaml` 的 `Upload.AllowedExts`）
- 文件头魔数校验：真实类型必须与扩展名一致，伪造后缀会被拒绝（返回 `4001`）
- 大小上限：默认 5MB（`Upload.MaxSizeMB`），超限返回 `4001`
- 文件名统一用 UUID 重命名，杜绝路径穿越与同名覆盖

**请求**：`multipart/form-data`，字段名 `file`

**响应 data**：

| 字段 | 说明 |
| --- | --- |
| url | 可访问的相对路径，如 `/uploads/2026/10/xxxx.png` |
| filename | 原始文件名 |
| size | 文件大小（字节） |

> 回填头像：把返回的 `url` 通过 `PUT /admin/profile` 的 `avatar` 字段更新即可；更新头像时会自动清理旧头像文件（默认头像除外）。
> 上传目录（`public/uploads/*`，默认头像除外）已在 `.gitignore` 忽略，避免运行时文件污染仓库。

---

## 文件管理

| 方法 | 路径 | 权限码 | 说明 |
| --- | --- | --- | --- |
| GET | `/admin/files` | `system:file:list` | 列出上传目录下的文件（支持目录、关键字、分页） |
| POST | `/admin/files/delete` | `system:file:delete` | 删除指定文件（空目录也可删） |
| PUT | `/admin/files/rename` | `system:file:rename` | 同级目录内重命名 |

> 以上接口需登录（JWT 鉴权）并具备对应权限码，超级管理员天然放行；权限清单由迁移时的种子数据自动写入（`internal/seed/permissions.go`）。路径均相对上传根目录，服务端做防穿越校验（`../` 会被拒绝）。

### GET `/admin/files`

**请求参数（query）**：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| dir | string | 子目录，相对上传根；为空表示根目录 |
| keyword | string | 按文件名模糊匹配 |
| page | int | 页码，默认 1 |
| page_size | int | 每页条数，默认 20，上限 100 |

**响应 data**：`list[]`（字段 `name / path / url / size / modified / is_dir`）、`total`、`current_dir`。

### POST `/admin/files/delete`

请求体：`{ "path": "相对上传根目录的路径" }`。目录仅允许在为空时删除。

### PUT `/admin/files/rename`

请求体：`{ "path": "...", "new_name": "新名称" }`。`new_name` 不含路径分隔符，且目标名称不得已存在。

---

## 数据字典

| 方法 | 路径 | 权限码 | 说明 |
| --- | --- | --- | --- |
| GET | `/admin/dicts` | `system:dict:list` | 字典类型列表（关键字、状态、分页） |
| POST | `/admin/dicts` | `system:dict:add` | 新增字典类型，编码唯一 |
| PUT | `/admin/dicts/:id` | `system:dict:edit` | 修改字典类型（名称/描述/状态/排序） |
| DELETE | `/admin/dicts/:id` | `system:dict:delete` | 删除字典类型，级联删除其下字典项 |
| GET | `/admin/dicts/items` | `system:dict_item:list` | 某字典下的字典项列表，`dict_code` 必填 |
| POST | `/admin/dicts/items` | `system:dict_item:add` | 新增字典项，同字典内 value 唯一 |
| PUT | `/admin/dicts/items/:id` | `system:dict_item:edit` | 修改字典项 |
| DELETE | `/admin/dicts/items/:id` | `system:dict_item:delete` | 删除字典项 |

**错误码**：`5601` 字典不存在、`5602` 字典编码已存在、`5603/5604/5605` 字典增删改失败、`5606` 字典项不存在、`5607` 字典项已存在。

### GET `/admin/dicts`

**请求参数（query）**：`keyword`（编码/名称模糊）、`status`（1 启用 / 0 禁用）、`page`（默认 1）、`page_size`（默认 20，上限 100）。

**响应 data**：`list[]`（`id / code / name / description / status / sort / created_at`）、`total`。

### GET `/admin/dicts/items`

**请求参数（query）**：`dict_code`（**必填**）、`keyword`（标签/值模糊）、`status`。响应 data：`list[]`（`id / dict_code / label / value / status / sort`）。

---

## 系统配置

| 方法 | 路径 | 权限码 | 说明 |
| --- | --- | --- | --- |
| GET | `/admin/configs` | `system:config:list` | 配置列表（按配置键/名称模糊） |
| POST | `/admin/configs` | `system:config:add` | 新增配置，`config_key` 唯一，新增项固定为非内置 |
| PUT | `/admin/configs/:id` | `system:config:edit` | 修改配置值/名称/备注 |
| DELETE | `/admin/configs/:id` | `system:config:delete` | 删除配置；`is_system=true` 的内置项拒绝删除 |

**错误码**：`5701` 配置不存在、`5702` 配置键已存在、`5703/5704` 配置增删改失败、`5705` 内置配置不可删除。

**响应 data**：`list[]`（`id / config_key / config_value / name / remark / is_system / created_at`）——不分页。

---

## 仪表盘统计

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/admin/dashboard/stats` | 后台首页统计指标，需登录（JWT 鉴权） |

### GET `/admin/dashboard/stats`

聚合常用运营指标，单项统计失败不影响其余字段返回（失败项记日志、值为 0）。

**响应 data**：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| admin_count | int64 | 管理员总数（admin_users） |
| dept_count | int64 | 部门总数（departments） |
| role_count | int64 | 角色总数（admin_roles） |
| permission_count | int64 | 权限总数（admin_permissions） |
| today_login_count | int64 | 今日登录成功次数（login_logs.status=0 且 login_at 为今日） |
| today_fail_login | int64 | 今日登录失败次数（login_logs.status=1 且 login_at 为今日） |
| week_op_count | int64 | 近 7 天操作日志数（operation_logs.created_at 在 7 天内） |

**示例**：

```json
{
  "code": 0, "msg": "success",
  "data": {
    "admin_count": 3, "dept_count": 5, "role_count": 4, "permission_count": 42,
    "today_login_count": 12, "today_fail_login": 1, "week_op_count": 318
  }
}
```

---

## 文章发布

> 本模块路由注册在 `api/cms/v1/internal/handler/contentroutes.go`，请求/响应类型在
> `api/cms/v1/internal/types/article_types.go`。与字典 / 配置 / 文件管理一样，它们没有写进
> `cms.api`；文档源单独放在 `api/cms/v1/content.doc.api`（**只供文档生成，不参与 goctl**）。
> 执行 `make gen-doc` 会由 `cmd/apidoc` 把两份源合并输出到 `openapi.json`，
> 因此 `openapi.json` 已覆盖内容模块的路径、query/path 参数与响应模型。
>
> 独立注册而非追加到 `routes.go` 的原因：`routes.go` / `types.go` 是 goctl 生成文件且被项目
> 手工追加过内容，重跑 `make gen-api` 会冲掉这些追加段。

### 数据模型与状态机

| 表 | 说明 |
| --- | --- |
| `articles` | 文章主表：标题、slug、摘要、封面、Markdown 正文、状态、发布时间、浏览量、置顶、SEO 字段、作者 |
| `categories` | 分类：支持父子层级，slug 唯一 |
| `tags` | 标签：名称与 slug 唯一 |
| `article_tags` | 文章与标签关联表；刻意不继承软删除，避免复合唯一索引被软删行占用 |
| `slug_redirects` | 旧别名历史：`(entity_type, old_slug, entity_id)`，支撑 301 与回收站恢复 |
| `article_revisions` | 修订快照：编辑前 / 恢复版本前自动存档，最多在列表中展示最近 50 条 |
| `article_reviews` | 审核动作留痕：提交 / 通过 / 驳回各一条，含操作人快照与驳回备注，随文章彻底删除一并清理 |

六张表由 `cmd/migrate` 的 `client.Schema.Create` 自动创建。文章 / 分类 / 标签继承
`BaseMixin + SoftDeleteMixin`，查询自动过滤软删除行。

**状态机**：`0 草稿 → 3 待审核 → 1 已发布 → 2 已下线`，下线后可再次发布；
`3 待审核` 可驳回退回 `0 草稿`。新建文章一律落草稿，状态只能经发布 / 撤回 / 审核接口变更，
编辑接口不会绕过状态机。发布状态是流程状态，刻意不放进数据字典（避免运营改字典破坏状态机）。

**列表展示**：`status_text` 为服务端派生文案；已发布但 `published_at` 在未来的文章显示
「定时中」。**公开可见条件**：`status = 1 AND published_at <= NOW()`，定时发布到点后
公开接口自动放行，无需后台定时任务。

**数据权限**：超级管理员可管理全部文章；普通用户只能编辑 / 删除 / 提交审核 / 恢复版本
自己创建的文章（`ensureOwner`），越权返回 `6031`。

**slug 与旧地址**：全局唯一，仅允许小写字母 / 数字 / 连字符。留空时从标题派生，纯中文标题
退化为 `post-<纳秒时间戳>`。改名或软删除时旧 slug 写入 `slug_redirects`；前台访问旧地址且
目标仍公开时返回 `301` 到新地址。软删除前把 slug 改为一次性占位值，避免唯一索引被已删行占用。

### 后台接口（需登录 + 权限码）

| 方法 | 路径 | 权限码 | 说明 |
| --- | --- | --- | --- |
| GET | `/admin/articles` | `cms:article:list` | 分页列表：`keyword` / `status` / `category_id` / `tag_id` |
| GET | `/admin/articles/:id` | `cms:article:detail` | 详情，含 Markdown 正文与 SEO 字段 |
| POST | `/admin/articles` | `cms:article:add` | 新建，落草稿；返回 `data.id` |
| PUT | `/admin/articles/:id` | `cms:article:edit` | 整单更新（不改状态与发布时间），事务内存档旧版本 |
| DELETE | `/admin/articles/:id` | `cms:article:delete` | 事务内软删除，移入回收站 |
| POST | `/admin/articles/:id/publish` | `cms:article:publish` | 发布 / 定时发布 |
| POST | `/admin/articles/:id/withdraw` | `cms:article:withdraw` | 撤回下线 |
| POST | `/admin/articles/:id/toggle-top` | `cms:article:toggle_top` | 切换置顶 |
| POST | `/admin/articles/batch-offline` | `cms:article:withdraw` | 批量下线，仅处理已发布文章；请求体 `{"ids":[1,2]}` |
| POST | `/admin/articles/batch-delete` | `cms:article:delete` | 批量移入回收站，事务内留痕并释放 slug |
| POST | `/admin/articles/:id/submit` | `cms:article:submit` | 提交审核：仅草稿且正文非空 |
| POST | `/admin/articles/:id/approve` | `cms:article:approve` | 审核通过，可带 `published_at` 定时发布 |
| POST | `/admin/articles/:id/reject` | `cms:article:reject` | 驳回，回退草稿；`remark` 写入审核记录 |
| GET | `/admin/articles/:id/reviews` | `cms:article:review` | 最近 50 条审核动作：提交 / 通过 / 驳回、操作人快照与备注 |
| GET | `/admin/articles/:id/revisions` | `cms:article:revision` | 最近 50 条修订记录 |
| GET | `/admin/articles/:id/revisions/:rid` | `cms:article:revision` | 修订详情（标题 / 摘要 / 封面 / 正文） |
| POST | `/admin/articles/:id/revisions/:rid/restore` | `cms:article:revision_restore` | 恢复版本，恢复前先存档当前内容 |
| GET | `/admin/trash` | `cms:trash:list` | 回收站：`type=article\|category\|tag` / `page` / `page_size` |
| POST | `/admin/trash/:type/:id/restore` | `cms:trash:restore` | 恢复并尝试还原原 slug；冲突时返回消息提示重设别名 |
| DELETE | `/admin/trash/:type/:id/purge` | `cms:trash:purge` | 事务内物理删除并清理不再被引用的上传文件 |
| GET | `/admin/categories` | `cms:category:list` | 分类列表（平铺 + `article_count`） |
| POST | `/admin/categories` | `cms:category:add` | 新增分类 |
| PUT | `/admin/categories/:id` | `cms:category:edit` | 修改分类（含父级成环检测） |
| DELETE | `/admin/categories/:id` | `cms:category:delete` | 删除空分类（有子分类 / 文章时拒绝） |
| GET | `/admin/tags` | `cms:tag:list` | 标签列表（含 `article_count`） |
| POST | `/admin/tags` | `cms:tag:add` | 新增标签 |
| PUT | `/admin/tags/:id` | `cms:tag:edit` | 修改标签 |
| DELETE | `/admin/tags/:id` | `cms:tag:delete` | 删除标签并清理关联 |

**列表请求参数（query）**：`page`（默认 1）、`page_size`（默认 20，上限 100）、
`keyword`（标题 / 摘要模糊）、`status`（0 草稿 / 1 已发布 / 2 已下线 / 3 待审核）、
`category_id`、`tag_id`、`sort` + `order`。`sort` 走白名单：`published_at` /
`view_count` / `updated_at` / `created_at` / `id`，`order` 为 `asc` / `desc`；
不传或传非法值时回退「置顶优先 + ID 倒序」，用户输入不会拼进 SQL。
响应 data：`list[]`、`total`、`page`、`page_size`，列表项含 `author_name` 与
派生 `status_text`。

批量接口单次最多 100 篇，会先去重、过滤非法 ID，再逐条做数据权限校验；
批量删除与单条删除行为一致（事务内写旧 slug 历史、释放 slug、写 `deleted_at`）。

**审核与修订请求体**：`approve` 与 `publish` 相同（`{"published_at": ""}` 留空为立即发布）；
`reject` 为 `{"remark": "..."}`；其余动作类接口发送空 JSON 对象即可。

**审核留痕**：提交 / 通过 / 驳回三个动作都在同一事务里写状态与一条 `article_reviews` 记录，
包含动作类型、操作人 ID 与名称快照、驳回备注；任何一步失败整体回滚。
`GET /admin/articles/:id/reviews` 返回最近 50 条，编辑页「文章轨迹」抽屉的审核记录页签展示。
文章彻底删除时会一并清理这些记录。

**新建 / 更新文章请求体**：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| title | string | 必填 |
| slug | string | 可选，留空自动生成 |
| summary / cover / content | string | 摘要、封面 URL（来自 `/admin/upload`）、Markdown 正文 |
| category_id | int64 | 可选，0 表示未分类；必须指向启用中的分类 |
| tag_ids | int64[] | 可选，服务端去重并校验标签存在且启用 |
| is_top | bool | 是否置顶 |
| seo_title / seo_description / seo_keywords | string | SEO 字段，留空时前台取标题 / 摘要 |

**发布请求体**：`{ "published_at": "2026-11-06 09:30:00" }`。留空表示立即发布；
支持 RFC3339、`YYYY-MM-DD HH:mm:ss`、`YYYY-MM-DDTHH:mm`。正文为空时拒绝发布。

**错误码**：`5801` 文章不存在、`5802` slug 被占用、`5803~5806` 文章增改删查失败、
`5807` slug 格式不合法、`5808` 发布失败、`5809` 撤回失败、`5810` 分类无效、
`5811` 标签无效、`5812` 时间格式不合法、`5813` 置顶失败；
`5901~5910` 分类错误、`5921~5926` 标签错误。

扩展能力错误码 `6000~6099`：`6001` 回收站类型非法、`6002` 回收站记录不存在、
`6003` 恢复失败、`6004` 彻底删除失败、`6005` 原 slug 冲突；
`6021` 修订版本不存在、`6022` 获取修订历史失败、`6023` 恢复版本失败；
`6031` 无权操作他人文章、`6032` 待审核文章需审核通过、`6033` 非待审核状态、
`6034` 缺少审核权限、`6035` 缺少发布权限、`6036` 内容实体类型非法。

### 公开接口（无需登录）

只挂 `ClientInfoMiddleware`（及可选的公开接口限流），不经过 JWT / 权限 / 操作日志中间件。
`/api` 前缀同时被 `SPAHandler` 排除，因此前台页面刷新不会误回退到 API 404。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/articles` | 已发布文章列表：`page` / `page_size`（默认 10）/ `keyword` / `category`（slug，含子孙分类）/ `tag`（slug） |
| GET | `/api/articles/:slug` | 按 slug 取详情，返回 Markdown 正文、作者、`canonical_url`、相关文章与上下篇 |
| GET | `/api/taxonomy` | 前台分类 / 标签导航，含 `parent_id` 便于前端渲染分类树 |
| GET | `/api/rss` | RSS 2.0（最近 50 篇），`application/rss+xml` |
| GET | `/sitemap.xml` | 站点地图（最近 200 篇），`application/xml` |
| GET | `/robots.txt` | 允许抓取、屏蔽 `/admin`，并指向 sitemap，`text/plain` |

公开接口不返回草稿 / 下线 / 未到点 / 待审核文章，列表也不返回正文。对应前台页面为
`/articles`（列表）与 `/articles/:slug`（详情），二者是顶层路由、不套后台 Layout、无需登录。

**服务端 SEO 与旧地址**：纯 SPA 客户端改 `document.title` 对爬虫无效，`SPAHandler` 会在返回
`index.html` 时服务端注入 `<title>` / `description` / `keywords` / `og:*` / `canonical`；
所有动态值经 `html.EscapeString`。访问历史 slug 且目标文章仍公开时返回 `301` 到新地址。
SPA 内部切换文章时由前端同步这些 meta。

**浏览量去重与原子累加**：详情接口按「IP 哈希 + 文章 ID」去重，Redis `SetnxEx` 24 小时窗口；
Redis 不可用时降级为进程内 `sync.Map`，并每小时惰性清理过期项避免内存无限增长。
通过去重后用 SQL `view_count = view_count + 1` 原子自增，避免并发访问丢计数；
自增失败会释放去重占位，保证该访客下次访问仍可重新计数。

**聚合计数**：`/api/taxonomy` 用「一次全量分类建树 + 两条 `GROUP BY` 聚合」得出分类与标签的
公开文章数，不再对每个分类 / 标签各执行一次 COUNT；父分类计数包含所有子孙分类。
`/api/articles/:slug` 把相关文章与上下篇一并纳入批量作者名查询，避免这些条目作者为空。

**公开接口限流**：`PUBLIC_RATE_LIMIT_ENABLED` / `PUBLIC_RATE_LIMIT_MAX` /
`PUBLIC_RATE_LIMIT_WINDOW` / `PUBLIC_RATE_LIMIT_BLOCK` 控制 `/api/` 前缀按 IP 的固定窗口限流，
与登录限流共用 Lua 保证 `INCR + EXPIRE` 原子性。Redis 不可用时降级为进程内计数；
超限时按项目统一响应约定返回 `HTTP 200` + `code=2029`（而非 429），并带上 `Retry-After`。

**正文安全**：`content` 是 Markdown 源码而非 HTML。前端 `web/src/utils/markdown.ts`
先做 HTML 转义再套用有限语法（标题、列表、任务列表、表格、引用、代码块、删除线等），
链接 / 图片地址白名单为 `http(s)`、站内绝对路径与 `#`，显式拒绝 `//` 协议相对地址与
`javascript:` 等协议，然后才用于 `v-html`。

**操作日志归类**：`/admin/articles`、`/admin/categories`、`/admin/tags` 已加入
`operationlogmiddleware.go` 的 `moduleByPrefix`，分别记为「文章管理 / 分类管理 / 标签管理」。

---

### 扩展能力实现要点

| 能力 | 落点 | 说明 |
| --- | --- | --- |
| 事务化 | `articlelogic.go` / `categorylogic.go` / `taglogic.go` / `trashlogic.go` | 删除、改 slug、存档修订、恢复与彻底删除统一走 `ent.Client.Tx`；软删除用 `SetDeletedAt` 而非绕过 hook 的 `DeleteOneID` |
| 事务超时隔离 | `etc/cms-api.yaml` / `logic/txcontext.go` | go-zero `RestConf.Timeout` 默认 3000ms 会给请求 ctx 加 deadline，事务继承后一旦到期会被 `database/sql` 自动回滚，产生 `context deadline exceeded` 与「事务回滚失败」噪声。现显式配置请求 15s，17 处写事务统一用 `context.WithoutCancel` 剥离取消信号、保留 trace/claims，并设 10s 独立上限，避免客户端先超时而事务随后提交 |
| 事务后文件清理 | `trashlogic.go` | 彻底删除文章时，收集仅被该文章引用的 `/uploads/` 封面与正文图片，事务提交后再删文件；失败只记日志 |
| 审核流 | `articlelogic.go` + `internal/seed/permissions.go` | `submit / approve / reject` 三个动作与权限码，待审核文章不能直接发布（`6032`） |
| 审核留痕 | `article_reviews` + `GET /admin/articles/:id/reviews` | 三个审核动作与状态变更同事务写记录（含操作人快照、驳回备注），编辑页「文章轨迹」可查，彻底删除文章时一并清理 |
| 数据权限 | `articleguard.go` / `ensureOwner` | 写操作由 `ensureOwner` 校验作者归属；读操作由 `articleReadScope` 统一判定：超管与持有 `cms:article:approve` / `cms:article:reject` 任一权限的审核角色可查看全部，其余用户按 `author_id` 过滤列表、详情、审核记录、修订历史与回收站（含恢复/彻底删除），越权读返回 `6037`；`cms:article:review` 仅能查看自己文章的审核记录，分类/标签为共享字典不过滤 |
| 权限种子去重 | `internal/seed/permissions.go` | API 权限按 `(type, permission, method, api_path)` 幂等 upsert，允许同一权限码绑定多条接口；早期只按 `(type, permission)` 去重，批量下线/删除等后写入的 spec 会覆盖单条撤回/删除记录，使非超管被误判「权限不足」 |
| 删除用户清理关联 | `admindeletelogic.go` | 软删管理员与清理 `admin_user_roles` 同事务；该关联表无软删除字段，残留行会让角色删除接口一直误判「角色正在被使用」 |
| 统一权限缓存 | `internal/pkg/permcheck` / `permissionmiddleware.go` | 中间件与文章行级守卫共用同一份权限快照（注入 ctx），消除每请求重复三表查询与两套判定结论；缓存 key 带全局/单用户版本号，角色授权、用户角色绑定、角色更新/删除、用户删除后立即失效；全量 API 权限定义按全局缓存后在内存做路径匹配，不再为每个具体 URL 生成缓存 key |
| 后台字典计数 | `taxonomycounts.go` | 分类/标签文章数由逐条 COUNT 改为一次 `GroupBy` 聚合，并复用 `articleReadScope` 叠加 `author_id`，计数口径与文章列表一致 |
| 修订历史 | `article_revisions` | 编辑前存档；恢复版本前再次存档，保证可逆 |
| 分类树筛选 | `publicarticlelogic.go` | `descendantCategoryIDs` 让父分类筛选自动包含所有子孙分类文章 |
| 验证码开关 | `LOGIN_CAPTCHA_ENABLED` | 关闭时 `/api/captcha` 返回 `enabled=false`，登录不再校验 `captcha_id` / `captcha`，前端登录页隐藏验证码输入；仅限本地开发与 E2E，生产必须开启 |
| 站点绝对地址 | `WEB_BASE_URL` | 用于 canonical、RSS、sitemap 的绝对链接；缺省时回退到请求 Host |
| 组合索引 | `ent/schema/article.go` / `articletag.go` | `deleted_at` 作为最左列覆盖软删除条件，配合 `status/published_at/is_top/category_id/author_id`（行级数据权限）及 `tag_id` 反查，避免列表全表扫描与 filesort |
| 健康检查 | `/healthz` + `docker-compose.yml` | DB `EXISTS` 探活（2 秒超时），Redis 故障只标 `degraded` 不判死；compose 每 15 秒探活 |
| 批量操作 | `/admin/articles/batch-offline` / `batch-delete` | 单次最多 100 篇，逐条数据权限校验 + 单事务提交 |
| 前端按需加载 | `web/vite.config.ts` / `src/utils/element.ts` / `MenuItem.vue` | 组件样式按需解析、菜单图标按映射表精确引入（171KB → 29KB）、服务式 API 走组件深路径、后台 Layout 动态 import；Element Plus 组件 JS 仍是单个 790KB chunk，进一步压缩需要组件级懒加载或替换 UI 库 |

### 文档生成链路

```bash
make gen-doc   # go run ./cmd/apidoc
```

生成器读取 `api/cms/v1/cms.api` 与 `content.doc.api`、`system.doc.api`、`log.doc.api`、
`site.doc.api`、`ops.doc.api` 五个手工路由文档源，输出根目录 `openapi.json`。
它会把 `:id` / `:slug` / `:type` / `:rid` 转成 OpenAPI 占位符，把 `form` / `path` 标签
转成 query / path parameters，把复合类型转成 `$ref`，并对 RSS / sitemap / robots
标注纯文本响应。`go test ./cmd/apidoc` 会遍历 `routes.go`、`contentroutes.go`、
`logroutes.go`、`sysroutes.go`、`siteextroutes.go` 中注册的全部路由，逐条校验方法 + 路径
都已出现在文档中。

新增手工接口时，请同时更新对应路由注册文件、类型定义、文档源、权限种子与本文档，再执行
`make gen-doc`；漏改会直接被覆盖门禁拦下。

## 配置 / 字典 / 站点扩展接口

### 公开接口（无需认证）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/site` | 字段白名单的站点信息，含 `comment_enabled`，不含任何密钥配置 |
| GET | `/api/dicts/:code` | 只返回启用字典的启用项，按 `sort` 排序；字典停用或不存在返回空列表 |
| GET | `/api/links` | 启用友链；`feature_link_enabled=false` 时返回空列表 |
| GET | `/api/comments/:slug` | 已发布文章的已通过评论，不下发邮箱与 IP |
| POST | `/api/comments` | 提交评论；受 `feature_comment_enabled` / `feature_comment_review` 控制，支持一级回复 |
| POST | `/api/subscribe` | 幂等邮件订阅；受 `feature_subscription_enabled` 控制 |
| GET | `/api/search` | 全文检索（标题/摘要/正文），返回 `keyword`、`took_ms`、`total`、`list` |

### 后台接口（需登录 + 权限码）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET / POST | `/admin/configs`、`/admin/configs/batch` | 配置列表（密文掩码）与事务批量保存 |
| GET | `/admin/configs/key/:key`、`/admin/configs/:id` | 按 key / 详情查询 |
| GET / POST | `/admin/config-versions/:id`、`/admin/config-versions/:id/rollback` | 变更历史与回滚 |
| GET / POST | `/admin/content/export`、`/admin/content/import` | 内容包导出（文件下载）与单事务幂等导入；`overwrite` 走 query |
| GET | `/admin/monitor` | 进程运行时指标与依赖状态 |
| GET / POST / PUT / DELETE | `/admin/links`、`/admin/links/:id` | 友链管理 |
| GET / PUT / DELETE | `/admin/comments`、`/admin/comments/:id/status`、`/admin/comments/:id` | 评论审核 |
| GET / DELETE | `/admin/subscribers`、`/admin/subscribers/:id` | 订阅管理 |

`GET /admin/files` 在原有字段上增加 `original_name`、`mime`、`width`、`height`、`ref_count`；
被文章封面或正文引用的文件执行删除 / 重命名会返回参数错误，避免站内死链。

`GET /admin/content/export` 直接输出 JSON 文件（不走 `{ code, msg, data }` 封装），
前端需用 `fetch` + `blob` 下载；其余接口均为统一响应封装。

