# Meteor CMS

[![Go Report Card](https://goreportcard.com/badge/github.com/yuwen002/meteor-cms)](https://goreportcard.com/report/github.com/yuwen002/meteor-cms)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

基于 Go 和 Go-Zero 开发的内容管理系统后端 API 服务。

## 功能特性

- ✅ 管理员登录认证
- 🔑 JWT Token 认证
- 🔐 密码重置功能
- 📝 管理员注册功能
- 📋 管理员列表功能
- 🔍 Token 验证测试接口
- 🚀 高性能 API 服务
- 📦 使用 Ent 作为 ORM
- 🛡️ 统一的错误处理
- 📊 结构化的日志记录
- 🗄️ 数据库自动迁移
- 🌱 初始化数据种子
- 🖼️ 图形验证码功能
- 🔐 管理员密码修改功能
- 🔁 管理员密码重置功能
- 🎯 RBAC 权限模型（菜单 / 按钮 / API 三级鉴权）
- 🧭 我的菜单接口（按角色动态下发菜单树与权限码）
- 🔒 数据权限（按部门范围收敛可见数据）
- ✉️ 忘记密码闭环（邮件令牌 + 一次性重置）
- 📄 统一分页参数（每页上限 100）
- 🧭 动态菜单：侧边栏由后端按角色下发的菜单树渲染，含图标 / 排序 / 多级层级
- 📚 数据字典：字典类型 + 字典项两级管理（字典项按 `dict_code` 归属）
- ⚙️ 系统配置：键值对配置项管理
- 🖼️ 文件管理：上传 / 列表 / 重命名 / 删除
- 📈 仪表盘统计：管理员 / 部门 / 角色 / 权限总量与今日登录、本周操作数
- 🗂️ 操作日志与登录日志：分页查询，便于审计
- ✍️ 文章发布：Markdown 编辑与预览、草稿 / 待审核 / 发布 / 下线状态机、定时发布、置顶、封面、SEO
- 🗃️ 内容分类与标签：分类树、标签管理，均带 slug 与文章数统计
- 🔎 内容治理：审核流与数据权限、修订历史与版本回滚、回收站（恢复 / 彻底删除）、旧 slug 301
- 🔒 安全与体验：服务端 SEO 注入、浏览量 IP 去重、公开接口限流、登录验证码开关、安全 Markdown 渲染
- 📡 内容分发：相关文章 / 上下篇、作者展示、RSS 2.0、sitemap.xml、robots.txt
- 🌐 公开阅读站点：`/articles` 前台列表与详情，无需登录，正文安全渲染
- ⚡ 性能与运维：组合索引、列表排序与批量操作、公开与后台字典聚合计数、`/healthz` 探活、前端依赖分包、写事务独立超时、权限种子按接口幂等去重、统一权限缓存与主动失效
- 👥 活跃会话管理：查看登录设备、按会话单独吊销（Redis 存储，不改 schema）
- 🖥️ 配套 Vue3 + Element Plus 前端管理后台（见「前端 Web 管理后台」一节）

## 技术栈

- **框架**: [Go-Zero](https://go-zero.dev/) v1.10.3
- **数据库**: MySQL (通过 [Ent ORM](https://entgo.io/) v0.14.6 支持)
- **认证**: JWT (github.com/golang-jwt/jwt/v5 v5.3.1)
- **密码加密**: bcrypt (golang.org/x/crypto v0.58.0)
- **参数验证**: go-playground/validator/v10 v10.30.5
- **图形验证码**: base64Captcha (github.com/mojocn/base64Captcha v1.3.8)
- **API 规范**: RESTful API
- **依赖管理**: Go Modules (Go 1.26.0)
- **代码生成**: goctl 1.10.2 (项目级自定义模板位于 `.goctl/`)
- **前端管理后台**:
  - **框架**: [Vue 3](https://vuejs.org/) + [Vue Router 4](https://router.vuejs.org/) + [Pinia](https://pinia.vuejs.dev/)
  - **UI 组件库**: [Element Plus](https://element-plus.org/) + [@element-plus/icons-vue](https://element-plus.org/)
  - **构建工具**: [Vite](https://vitejs.dev/) 5 + [TypeScript](https://www.typescriptlang.org/) 5
  - **类型检查**: vue-tsc

## 快速开始

### 环境要求

- Go 1.26+
- MySQL 5.7+
- Git
- goctl 1.10.2（仅代码生成时需要）

### 安装

1. 克隆仓库

```bash
git clone https://github.com/yuwen002/meteor-cms.git
cd meteor-cms
```

2. 安装依赖

```bash
go mod tidy
```

3. 数据库配置

确保 MySQL 服务正在运行，并创建数据库：
```sql
CREATE DATABASE go_meteor_cms CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

数据库连接信息按以下优先级获取（API 服务与迁移工具共用同一份配置）：

1. 环境变量 `MYSQL_DSN`（推荐用于 CI / 生产）
2. `api/cms/v1/etc/cms-api.yaml` 中的 `Database.Source`

```bash
export MYSQL_DSN="root:123456@tcp(127.0.0.1:3306)/go_meteor_cms?parseTime=True&loc=Local"
```

邮件配置（`etc/cms-api.yaml` 的 `Mail` 节点）用于忘记密码时投递重置令牌：

```yaml
Mail:
  Enabled: false                  # false 时只输出到服务日志，不发真实邮件
  Host: "smtp.example.com"
  Port: 587                       # 587(STARTTLS) / 465(SSL) / 25
  Username: "noreply@example.com"
  Password: ""                    # 密码或授权码
  From: "Meteor CMS <noreply@example.com>"
```

4. 数据库迁移

```bash
# 执行数据库迁移和初始化数据（幂等，可重复执行）
go run cmd/migrate/main.go
```

迁移完成后会输出权限统计（菜单 / 按钮 / API 三类数量，以实际种子为准；不要用硬编码数字做判断），其中 API 权限直接决定非超级管理员能否访问各接口。

5. 启动服务

```bash
# 启动 API 服务（需用 -f 指定配置文件）
go run ./api/cms/v1 -f api/cms/v1/etc/cms-api.yaml
```

## 前端 Web 管理后台

仓库内 `web/` 为配套的前端管理后台，技术栈见上方「技术栈 · 前端管理后台」。

### 环境要求

- Node.js 18+（建议 LTS）
- 包管理器 npm

### 本地开发

```bash
cd web
npm install
npm run dev          # 启动开发服务器，默认 http://localhost:5173
```

开发服务器通过 Vite 代理将 `/admin`、`/api`、`/uploads` 转发到后端 `http://localhost:8888`，
因此本地联调时需要先在 8888 端口启动后端服务。

### 构建与预览

```bash
npm run build        # 类型检查（vue-tsc）+ 生产构建，产物在 web/dist
npm run preview      # 本地预览构建产物
```

### 动态菜单说明

侧边栏**不写死在代码里**，而是登录后由 `GET /admin/profile/permissions` 返回的菜单树驱动：

- 后端按角色聚合出 `menus`（字段含 `id / name / path / component / icon / children`），
  前端 `Sidebar.vue` 递归渲染 `MenuItem.vue`（`el-sub-menu` + `el-menu-item`）。
- 图标为语义键（如 `dashboard`、`dict`、`setting`），前端 `MenuItem.vue` 的 `iconMap` 映射到 Element Plus 图标组件；未命中时回落为 `Menu` 图标。
- 菜单按 `sort` 升序，后端在 `buildMenuTree` 中按 `parent_id` 组装多级层级；当父菜单对当前用户无权限时，其子菜单会自动降级为顶级菜单，不会丢失。

> 新增自定义页面时，请同步在种子 `internal/seed/permissions.go` 中维护对应「菜单 / 按钮 / API」三类权限，
> 并在 `MenuItem.vue` 的 `iconMap` 中补充图标键，否则侧边栏图标会回落为默认图标。

## 项目结构

```
.
├── api/                    # API 定义
│   └── cms/               # CMS 服务
│       └── v1/            # API 版本
├── cmd/                   # 命令行工具
│   └── migrate/           # 数据库迁移工具
├── ent/                   # Ent ORM 实体
│   └── schema/            # 数据库表结构定义
├── internal/              # 内部包
│   ├── common/            # 通用组件（响应、错误码、权限类型）
│   ├── pkg/               # 可复用子包（mail 邮件、datascope 数据权限、cleanup 维护任务）
│   ├── seed/              # 初始化数据（含权限清单）
│   └── utils/             # 工具函数（jwt、password、captcha、page）
├── web/                   # 前端管理后台（Vue3 + Element Plus，侧边栏由后端菜单树驱动）
├── rpc/                   # RPC 服务定义
├── Makefile               # 常用开发命令
├── Dockerfile             # 多阶段构建镜像
└── docker-compose.yml     # 全栈一键编排（MySQL + Redis + 迁移 + API，前端打包进镜像）
```

## API 接口

启动服务后，默认访问地址：`http://localhost:8888`

所有接口统一返回下列结构，HTTP 状态码恒为 200，业务结果由 `code` 字段表示：

```json
{
  "code": 0,
  "msg": "success",
  "data": {}
}
```

- `code = 0`：成功
- `code > 0`：业务错误，`msg` 为提示信息，完整错误码见 [`internal/common/error_code.go`](internal/common/error_code.go)

> 分页参数通用约定：`page` 小于 1 时按 1 处理，`page_size` 小于 1 时取接口默认值，大于 100 一律截断为 100，响应中的 `page_size` 为实际生效值。
> 完整的接口清单（含角色、权限、部门模块）见 [`docs/api.md`](docs/api.md) 与 `docs/api-docs/meteor-cms.apifox.json`。

### 公共接口

1. **获取验证码**
   - URL: `GET /api/captcha`
   - 响应:
     ```json
     {
       "code": 0,
       "msg": "success",
       "data": {
         "captcha_id": "验证码ID",
         "captcha_base64": "验证码图片base64编码"
       }
     }
     ```

2. **管理员登录**
   - URL: `POST /admin/login`
   - 参数:
     ```json
     {
       "username": "admin",
       "password": "123456",
       "captcha_id": "验证码ID",
       "captcha": "验证码"
     }
     ```
   - 响应:
     ```json
     {
       "code": 0,
       "msg": "success",
       "data": {
         "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
       }
     }
     ```

3. **忘记密码（第一步：申请重置令牌）**
   - URL: `POST /admin/forgot-password`
   - 说明：无论账号是否存在均返回成功，避免账号枚举；令牌 1 小时内有效
   - 参数:
     ```json
     {
       "username": "admin"
     }
     ```
   - 响应:
     ```json
     {
       "code": 0,
       "msg": "success",
       "data": {
         "message": "重置密码邮件已发送，请检查邮箱"
       }
     }
     ```

4. **忘记密码（第二步：用令牌重置密码）**
   - URL: `POST /admin/reset-password`
   - 说明：令牌一次性，使用后立即失效；新密码不少于 6 位
   - 参数:
     ```json
     {
       "token": "邮件中的重置令牌",
       "new_password": "newpassword123"
     }
     ```
   - 响应:
     ```json
     {
       "code": 0,
       "msg": "success",
       "data": {
         "message": "密码重置成功，请使用新密码登录"
       }
     }
     ```

5. **管理员注册**
   - URL: `POST /admin/register`
   - 参数:
     ```json
     {
       "username": "newadmin",
       "password": "password123",
       "email": "newadmin@example.com",
       "nickname": "新管理员"
     }
     ```
   - 响应:
     ```json
     {
       "code": 0,
       "msg": "success",
       "data": {
         "message": "管理员注册成功"
       }
     }
     ```

### 需要认证的接口

以下接口需要在请求头中添加 Authorization:

```
Authorization: Bearer <token>
```

除管理员列表接口会额外按数据权限收敛结果外，认证接口的可见性由 RBAC 决定，详见下方「权限模型」与「数据权限」两节。

1. **获取当前用户的菜单与权限**（前端据此渲染菜单与按钮）
   - URL: `GET /admin/profile/permissions`
   - 响应:
     ```json
     {
       "code": 0,
       "msg": "success",
       "data": {
         "is_super": false,
         "menus": [
           {
             "id": 3,
             "name": "部门管理",
             "path": "/admin/departments",
             "component": "views/admin/dept/list.vue",
             "icon": "dept",
             "permission": "system:dept:list",
             "children": []
           }
         ],
         "permissions": ["system:dept:list", "system:dept:add"]
       }
     }
     ```

2. **角色部门范围**
   - URL: `GET /admin/roles/:id/departments`
   - URL: `PUT /admin/roles/:id/departments`
   - 说明：仅 `data_scope=5`（自定义部门）的角色支持设置，系统内置角色不可修改
   - PUT 参数:
     ```json
     { "dept_ids": [1, 2] }
     ```

3. **Token 验证测试**
   - URL: `GET /admin/test-token`
   - 响应:
     ```json
     {
       "code": 0,
       "msg": "success",
       "data": {
         "message": "Token OK",
         "claims": {
           "user_id": 1,
           "username": "admin"
         }
       }
     }
     ```

4. **获取管理员列表**（按当前用户的数据权限收敛结果）
   - URL: `GET /admin/admin-users`
   - 查询参数:
     - `page`: 页码（必填）
     - `page_size`: 每页数量（必填）
     - `keyword`: 搜索关键词（用户名/昵称/邮箱，选填）
     - `active`: 启用状态过滤（1-启用，2-禁用，选填）
   - 响应:
     ```json
     {
       "code": 0,
       "msg": "success",
       "data": {
         "total": 10,
         "page": 1,
         "page_size": 10,
         "list": [
           {
             "id": 1,
             "username": "admin",
             "nickname": "管理员",
             "email": "admin@example.com",
             "phone": "13800138000",
             "avatar": "/uploads/avatars/meteor-default.jpg",
             "isSuper": true,
             "isActive": true,
             "last_login_at": "2023-01-01T12:00:00Z",
             "createdAt": "2023-01-01T12:00:00Z"
           }
         ]
       }
     }
     ```

5. **修改自己的密码**
   - URL: `PUT /admin/admin-users/me/change-password`
   - 参数:
     ```json
     {
       "old_password": "oldpassword",
       "new_password": "newpassword"
     }
     ```
   - 响应:
     ```json
     {
       "code": 0,
       "msg": "success",
       "data": {
         "message": "密码修改成功"
       }
     }
     ```

6. **重置其他管理员的密码** (需要超级管理员权限，且目标不能是超级管理员)
   - URL: `PUT /admin/admin-users/reset-password`
   - 参数:
     ```json
     {
       "id": 2,
       "new_password": "newpassword"
     }
     ```
   - 响应:
     ```json
     {
       "code": 0,
       "msg": "success",
       "data": {
         "message": "密码已重置"
       }
     }
     ```

### 业务模块一览

除上方「公共接口」与「需要认证的接口」示例外，其余业务接口按模块归纳如下（均需 `Authorization: Bearer <token>`，完整字段定义见 `docs/api.md`）：

| 模块 | 主要接口 | 说明 |
| --- | --- | --- |
| 仪表盘 | `GET /admin/dashboard/stats` | 管理员 / 部门 / 角色 / 权限总量，今日登录与本周操作数 |
| 管理员 | `GET/POST /admin/admin-users`、`GET/PUT/DELETE /admin/admin-users/:id`、禁用/启用、重置密码、绑定部门、角色分配 | 分页 + 数据权限收敛 |
| 部门 | `GET /admin/departments/tree`、`POST /admin/departments`、`GET/PUT/DELETE /admin/departments/:id` | 树形结构 |
| 角色 | 角色 CRUD、`GET/PUT /admin/roles/:id/permissions`、`GET/PUT /admin/roles/:id/departments` | 权限与数据范围分配 |
| 权限 | `GET /admin/permissions`、`GET /admin/permissions/tree`、CRUD | 菜单 / 按钮 / API 三级定义 |
| 数据字典 | `GET/POST /admin/dicts` + CRUD、`GET/POST /admin/dicts/items` + 字典项 CRUD | 字典类型 + 字典项 |
| 系统配置 | `GET/POST /admin/configs` + CRUD | 键值对配置 |
| 文件 | `GET /admin/files`、`PUT /admin/files/rename`、`POST /admin/files/delete`、`POST /admin/upload` | 上传 / 列表 / 重命名 / 删除 |
| 日志 | `GET /admin/logs/operations`、`GET /admin/logs/logins` | 操作 / 登录审计 |
| 个人中心 | `GET/PUT /admin/profile`、`PUT /admin/admin-users/me/change-password`、`GET /admin/profile/permissions` | 自身资料与菜单权限 |
| 登录会话 | `GET /admin/sessions`、`DELETE /admin/sessions/:sid` | 活跃设备列表与单会话吊销，详见「活跃会话管理」 |
| 文章 | `GET/POST /admin/articles`、`GET/PUT/DELETE /admin/articles/:id`、`POST /admin/articles/:id/publish`、`/withdraw`、`/toggle-top` | 草稿 / 发布 / 下线状态机、定时发布、置顶、Markdown 正文、封面与 SEO |
| 分类 | `GET/POST /admin/categories`、`PUT/DELETE /admin/categories/:id` | 父子层级、slug、删除前置校验 |
| 标签 | `GET/POST /admin/tags`、`PUT/DELETE /admin/tags/:id` | 名称 / slug 唯一、文章数统计 |
| 审核与修订 | `POST /admin/articles/:id/submit` `/approve` `/reject`、`GET/POST /admin/articles/:id/revisions...` | 待审核状态机、编辑前存档、版本回滚 |
| 回收站 | `GET /admin/trash`、`POST /admin/trash/:type/:id/restore`、`DELETE /admin/trash/:type/:id/purge` | 软删除内容恢复 / 物理删除与孤儿文件清理 |

另有**无需认证**的公开接口：`GET /api/articles`、`GET /api/articles/:slug`、`GET /api/taxonomy`、
`GET /api/rss`、`GET /sitemap.xml`、`GET /robots.txt`，
对应前台阅读页 `/articles` 与 `/articles/:slug`。字段定义与状态机说明见 `docs/api.md`「文章发布」章节。

OpenAPI 文档由 `make gen-doc` 生成：以 `api/cms/v1/cms.api` 为主体，合并
`content.doc.api`（内容模块）、`system.doc.api`（配置 / 字典 / 文件）、
`log.doc.api`（日志详情 / 导出 / 统计）、`site.doc.api`（公开站点 / 配置增强）、
`ops.doc.api`（内容包 / 监控 / 全文检索 / 站点扩展）共五份文档源。
`*.doc.api` 只供文档生成，不参与 goctl；产物为根目录 `openapi.json`，
镜像内同时发布到 `/openapi.json` 供 `/docs.html` 使用。
`cmd/apidoc` 的测试自动扫描 `internal/handler/*routes.go` 中注册的全部路由
做覆盖校验，CI 用 `git diff --exit-code -- openapi.json` 阻止新增接口漏同步文档。

## 默认管理员账户

系统初始化时会自动创建默认管理员账户：

- 用户名: `admin`
- 密码: `123456`

> `123456` 不满足后台密码复杂度策略（≥8 位且含三类字符），它是初始化种子的**唯一例外**：
> 种子直接构造用户记录，不经过业务入口校验。保留该默认值是为了本地开发与 E2E 开箱即用，
> 启动日志会固定输出一条安全告警。**生产环境首次登录后必须立即修改**，并配合
> `STRICT_SECURITY=true` 与自定义 `AUTH_ACCESS_SECRET` / `AUTH_REFRESH_SECRET` 使用。

## 权限模型

权限记录存放在 `admin_permissions` 表，由迁移时的种子写入，分三类：

| 类型 | 取值 | 用途 |
| --- | --- | --- |
| 菜单 | 1 | 前端路由菜单，`GET /admin/profile/permissions` 据此下发菜单树 |
| 按钮 | 2 | 前端功能点，同 `permissions` 字段下发，用于控制按钮显隐 |
| API | 3 | 后端接口，权限中间件按 `method + api_path` 匹配后校验角色是否拥有对应标识 |

关键约定：

- **按钮与 API 使用相同的权限标识**（如 `system:role:add`），因此授权一次即可同时让按钮可见、接口可调用
- **API 权限的 `api_path` 必须与 `cms.api` 中的真实路由一致**，否则非超级管理员访问时会被判定为「未配置权限」而拒绝；新增受保护接口后请同步追加一条 Type=3 权限
- 路由匹配按「静态段命中数最多者优先」，`/admin/departments/tree` 不会被 `/admin/departments/:id` 抢占
- 以下路由不参与 API 鉴权：`/admin/logout`、`/admin/test-token`、`/admin/admin-users/me/change-password`、`/admin/profile/permissions`
- 超级管理员（`admin_users.is_super`）跳过全部鉴权与数据范围过滤
- **授权边界（防提权）**：非超管在「分配角色权限」与「给用户分配角色」时，只能授予自己权限快照内已有的权限；内置 `is_system` 角色（含 `SUPER_ADMIN`）与停用角色只允许超管下发；创建管理员时 `is_super=true` 仅超管可设置。判定取 DB 中的 `is_super` 真值而非 token claim，避免旧令牌携带过期身份
- 角色权限分配允许提交空 `permission_ids`（清空全部权限）；前端权限树同时提交半选的父级节点，保证动态菜单父节点不丢失。移除用户角色后立即失效该用户的权限快照，不留缓存空窗
- 删除角色会单事务清理 `admin_role_permissions` / `admin_user_roles` / `admin_role_depts` 并物理删除角色，避免软删行占用 `name` / `code` 唯一索引导致同名无法重建；角色被占用返回专用错误码 5209

## 数据权限

角色通过 `data_scope` 声明可见数据范围：

| data_scope | 含义 | 说明 |
| --- | --- | --- |
| 1 | 全部数据 | 不做行级过滤 |
| 2 | 本部门 | 与当前管理员的 `dept_id` 一致 |
| 3 | 本部门及子部门 | 沿部门树展开全部子孙部门 |
| 4 | 仅自己 | 只能看到自己的记录 |
| 5 | 自定义部门 | 读取 `admin_role_depts`，通过 `PUT /admin/roles/:id/departments` 维护 |

- 多个角色时取各角色可见范围的并集；任一角色为「全部数据」时不过滤
- **安全原则（fail-closed）**：范围解析失败、用户未分配任何启用角色、或要求按部门过滤但未分配部门时，一律按最严格处理（不返回数据），绝不退回全量
- 文章模块已落地行级过滤：超管与持有 `cms:article:approve` / `cms:article:reject` 任一权限的审核角色可查看全部文章，其余用户仅能看到自己创建的文章（列表、详情、审核记录、修订历史、回收站，含恢复与彻底删除）；`cms:article:review` 只表示可查看审核记录，普通作者持有它仍只能看自己文章；分类、标签属于共享字典，不做作者过滤
- 管理员列表/详情按 `dept_id` 走 `datascope` 范围解析；文章模块按 `author_id` 判定，两套模型分别适用于不同实体
- `data_scope` 写入时校验 1-5；非法值只会让该条角色被跳过，不再让整个 `Resolve` 失败、连带其它角色一起退化。全部角色都无效时按空集 fail-closed
- **部门生命周期收权**：只读「仍存在且启用」的部门参与范围计算。部门被软删除或停用后，`admin_users.dept_id` 与 `admin_role_depts` 中的旧引用不再放权；「本部门及子部门」也只沿启用部门展开
- 角色从 `data_scope=5` 改为其它取值时，同事务清空 `admin_role_depts`，避免日后改回 5 时继承历史部门范围
- 部门树按指针挂载后再物化，任意层级都能完整返回；更新父级时会沿 `parent_id` 回溯拒绝挂到自己的后代下，根级统一用 `NULL`（不再写 `parent_id=0` 撞自引用外键）

## 活跃会话管理

查看登录设备并单独吊销某个会话。会话是易失的运行时状态，和全局踢人的 `auth:tv:<uid>` 一样放在 Redis：
不新增表、不改变 schema 摘要，也不需要额外迁移。

- 登录时给 access / refresh 令牌写入 `sid` 声明，并登记 `auth:session:<uid>:<sid>`（IP、UA、创建 / 最后活跃 / 过期时间、当前 access token 哈希），用户索引为 `auth:sessions:<uid>`。TTL 取 `Auth.RefreshExpire`（默认 7 天）。
- 刷新令牌时沿用原 `sid` 并更新当前 token 哈希；已吊销会话的刷新请求直接返回 2012，避免持续轮换让吊销标记先过期而「复活」。登出时吊销当前会话。升级前签发、不带 `sid` 的旧令牌仍可正常使用，只受 token 版本与黑名单控制。
- 吊销写入 `auth:session:revoked:<sid>`，JWT 中间件按 `sid` 拦截（返回 2012）；同时把该会话当前 access token 写进既有 `token_blacklists` 表作双重保险。只影响目标设备，同用户其它会话与新登录不受影响。

| 接口 | 权限标识 | 说明 |
| --- | --- | --- |
| `GET /admin/sessions` | `system:session:list` | 列出当前管理员的活跃会话，`is_current` 标记本次请求所用会话；响应不含 token 哈希 |
| `DELETE /admin/sessions/:sid` | `system:session:revoke` | 吊销指定会话；只能吊销自己的会话，不存在 / 已过期 / 他人会话返回参数错误 |

两个接口都挂 `ClientInfo → JWT → Permission → OperationLog` 受保护链；权限种子已包含对应的按钮与 API 记录（挂在「管理员管理」下），迁移时幂等写入。

与既有强制下线的区别：强制下线自增 `auth:tv:<uid>`，使该用户**全部**令牌立即失效；会话吊销只针对单个 `sid`，适合「让某台设备退出登录」。

降级约定与项目其它 Redis 能力一致：未配置 Redis 时登录 / 刷新 / 登出照常工作，中间件不校验会话状态，但列表与吊销接口返回明确错误；Redis 访问异常时中间件降级放行避免误杀。需要强隔离时请用强制下线 / 修改密码，不要依赖单会话吊销。

## 开发指南

### 代码生成

项目在 `.goctl/` 下维护了自定义的 handler 模板（统一使用 `common.Ok` / `common.Fail` 响应），因此生成时必须用 `--home` 指定该目录，否则会退回内置模板产出风格不一致的代码：

```bash
# 安装指定版本的 goctl
go install github.com/zeromicro/go-zero/tools/goctl@v1.10.2

# 生成 API 代码 (基于 api/cms/v1/cms.api)
# 注意：goctl 不会覆盖已存在的 handler/logic 文件，只会新增缺失的文件
goctl api go -api api/cms/v1/cms.api -dir api/cms/v1 --home .goctl

# 生成 Ent 代码 (详见 docs/ent.md)
go run -mod=mod entgo.io/ent/cmd/ent generate --feature intercept,schema/snapshot ./ent/schema
```

> 不要执行 `goctl template init --home .goctl`，该命令会覆盖 `.goctl/api/handler.tpl` 中的自定义模板。

### 测试

```bash
# 运行全部测试
go test ./...

# 只跑带用例的包（其余包显示 no test files）
go test ./internal/... ./api/cms/v1/internal/middleware/...

# 运行测试并生成覆盖率报告
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out

# 重新生成全量 OpenAPI 文档并校验所有注册路由覆盖
make gen-doc && go test ./cmd/apidoc
```

当前已覆盖的纯逻辑单测（均不依赖外部 MySQL / Redis / Docker，可直接进 CI）：

- `internal/utils`：分页参数规范化（默认值、负值纠正、上限截断）
- `internal/pkg/datascope`：部门树子孙收集，含脏数据成环时不死循环
- `internal/pkg/mail`：邮件报文构造（UTF-8 主题与正文编码）、SMTP 参数校验
- `internal/pkg/tokenversion`：token 版本号读取 / 自增、按用户隔离、未配置 Redis 时降级（进程内 miniredis）
- `api/cms/v1/internal/middleware`：路由匹配规则、静态路由优先于动态路由、免鉴权路由判定；
  SPA 托管的 history 回退与目录穿越防护；JWT 中间件的 token 版本比对与全局踢人
- `api/cms/v1/internal/logic`：文章状态机与公开可见性（含定时发布到点放行）、slug 规范化与中文兜底、发布时间格式解析、写事务 ctx 剥离请求取消信号后保留 trace/claims
- `internal/seed`：权限种子的 `(method, api_path)` 唯一性、文章模块 API / 按钮 / 菜单齐备、菜单父级声明顺序、contentroutes 全路由登记覆盖、同权限码多接口不被覆盖
- `cmd/apidoc`：OpenAPI 输出覆盖内容模块路径、schema、query / path 参数与 RSS / sitemap 纯文本响应
- JWT 中间件测试用进程内 Redis + 空结果集 ent driver 替代数据库，无需 MySQL / Docker

### 端到端测试（需 Docker）

跨账号的数据权限必须打真实 HTTP 才能验证，用例放在 `test/e2e`，用 `e2e` build tag 隔离，
因此普通 `go test ./...` 不会连接任何外部服务。

```bash
LOGIN_CAPTCHA_ENABLED=false LOGIN_RATE_LIMIT_ENABLED=false PUBLIC_RATE_LIMIT_ENABLED=false docker compose up -d --build
go test -tags=e2e -count=1 -v ./test/e2e/...
docker compose down -v
```

可用 `CMS_E2E_BASE_URL` / `CMS_E2E_ADMIN_PASSWORD` 覆盖接口地址与超管密码。
CI 的 `Data permission E2E` job 会起 compose、等待 `/healthz` 就绪后执行同一组用例，
用例自行创建并清理测试用的用户、角色与文章。用户/角色通过业务接口删除（软删），
本地重复运行会保留这些软删记录；CI 每次使用全新数据卷，不受影响。

只跑文章相关的回归用例：

```bash
go test ./api/cms/v1/internal/logic -run 'Test(NormalizeSlug|GenerateSlug|IsValidSlug|IsPubliclyVisible|ParsePublishTime|ArticleStatus)'
go test ./internal/seed
```

## 部署

### Docker（全栈一键）

`docker-compose.yml` 编排完整栈：MySQL（数据） + Redis（限流 / 全局踢人） + 迁移（表结构 + 种子） + API（含前端 SPA）。
镜像由仓库根目录 `Dockerfile` 多阶段构建：前端 `web/` 的构建产物被打进同一镜像，由 Go 服务在 8888 端口统一托管。

前端由 Go 服务通过 `rest.WithNotFoundHandler` 托管（见 `api/cms/v1/internal/middleware/spa.go`）：
`/`、`/login`、`/dashboard`、`/system/*` 等未匹配路由回退到 `index.html`（保证 `createWebHistory` 刷新不 404），
`/assets/*` 等真实文件直接返回；`/admin`、`/api`、`/uploads` 前缀保留给 API 与上传资源，不参与回退。
注意不要用 `server.Use` 注册 SPA 兜底：go-zero 的全局中间件不作用于 404（未匹配）路由。

> Windows / macOS 需先启动 Docker Desktop 并等引擎就绪（`docker info` 能返回 ServerVersion），
> 否则 `docker compose` 会报 `dockerDesktopLinuxEngine` 管道不存在。

```bash
cp .env.example .env          # 可选：按需修改端口、口令、构建镜像源
# 一键启动：mysql 就绪 → migrate 迁移+种子 → api 启动并托管前端
docker compose up -d

# 跟踪日志（看迁移进度 / API 启动）
docker compose logs -f api

# 停止并保留数据
docker compose down
# 停止并清空数据（含库）
docker compose down -v
```

应用通过环境变量连接依赖，无需改动 `cms-api.yaml`：`MYSQL_DSN` 指向 `mysql:3306`、`REDIS_HOST` 指向 `redis:6379`、
`WEB_DIST=/app/web/dist`。这些值由 compose 从 `.env` 注入容器，**部署到别的机器时改 `.env` 即可，不要改 `docker-compose.yml`**。

配置项集中在 `.env`（复制 `.env.example` 得到，含开发默认值）：可覆盖 root 密码、库名、各服务端口等；
不提供 `.env` 时 compose 的 `${VAR:-默认值}` 自动生效，开箱即用。注意改密码 / 库名时要同步改 `MYSQL_DSN` 中的对应部分。

受限网络下容器构建可能拉不到默认官方源，可在 `.env` 里启用镜像源（它们是 **build args**，只影响构建、不进入运行容器）：

```bash
NPM_REGISTRY=https://registry.npmmirror.com/
GOPROXY=https://goproxy.cn,direct
```

修改后需重新构建：`docker compose build && docker compose up -d`。

> 端口说明：mysql 默认**不映射**宿主机端口（避免与本地 MySQL 的 3306 冲突），
> 如需从宿主机直连排查，取消 `docker-compose.yml` 中 mysql 的 `ports` 注释并设 `MYSQL_PORT`（如 `3307`）。
> redis 默认映射 `${REDIS_PORT:-6379}:6379` 供本机调试；若 6379 已被本地 Redis 占用，
> 在 `.env` 里改 `REDIS_PORT`（如 `6380`）即可——容器内服务间仍走 `redis:6379`，不受影响。

仅想用容器里的 MySQL / Redis、应用仍在宿主机跑（需先按上方说明打开 mysql 的 `ports` 映射；
redis 默认已映射 `${REDIS_PORT:-6379}:6379`）：

```bash
docker compose up -d mysql redis
make migrate && make run                  # 或 go run ./api/cms/v1 -f api/cms/v1/etc/cms-api.yaml
```

或单独构建运行：

```bash
docker build -t meteor-cms:local .
docker run -d -p 8888:8888 \
  -e MYSQL_DSN="root:123456@tcp(host.docker.internal:3306)/go_meteor_cms?parseTime=True&loc=Local" \
  -e REDIS_HOST="host.docker.internal:6379" \
  -e WEB_DIST="/app/web/dist" \
  meteor-cms:local
```

容器内的迁移工具位于 `/app/migrate`，可在部署流程中单独执行：

```bash
docker run --rm -e MYSQL_DSN="..." --entrypoint /app/migrate meteor-cms:local
```

### 编译部署

```bash
go build -o bin/cms ./api/cms/v1
go build -o bin/migrate ./cmd/migrate

# 先迁移（幂等），再启动服务
./bin/migrate
./bin/cms -f api/cms/v1/etc/cms-api.yaml
```

### 环境变量

应用进程读取的环境变量（本地直跑 / 单独 `docker run` 时同样适用）：

| 变量 | 说明 |
| --- | --- |
| `MYSQL_DSN` | 数据库连接串，设置后覆盖配置文件中的 `Database.Source`（容器化部署推荐；未设置时默认 `127.0.0.1:3306`） |
| `REDIS_HOST` / `REDIS_PASS` | Redis 地址与密码，容器内置为 `redis:6379` |
| `WEB_DIST` | 前端构建产物目录；容器内置为 `/app/web/dist`，本地开发留空则仅暴露 API |
| `AUTH_ACCESS_SECRET` / `AUTH_REFRESH_SECRET` | JWT 签名密钥，生产必须替换仓库示例值 |
| `STRICT_SECURITY` / `SQL_DEBUG` / `SCHEMA_CHECK_STRICT` | 安全严格模式 / SQL 日志开关 / schema 版本校验开关 |
| `METRICS_ENABLED` / `METRICS_TOKEN` | Prometheus `/metrics` 抓取开关（默认关闭）与 Bearer token |
| `CMS_CONFIG` | `cmd/migrate`、`cmd/backup` 读取的配置文件路径；也可用各自的 `-f` 指定 |

其余（连接池、令牌有效期、邮件、维护策略、限流）见「程序初始化与数据迁移」章节与 `.env.example`。

compose 专用配置（写在 `.env`，由 `${VAR:-默认值}` 插值；不提供 `.env` 时用默认值）：

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `CMS_IMAGE` | `meteor-cms:local` | migrate / api 使用的镜像；生产固定为已发布版本，改回旧版本即可回滚 |
| `MYSQL_ROOT_PASSWORD` | `123456` | MySQL root 口令，需与 `MYSQL_DSN` 保持一致 |
| `MYSQL_DATABASE` | `go_meteor_cms` | 库名，需与 `MYSQL_DSN` 保持一致 |
| `API_PORT` | `8888` | API / 前端对外端口 |
| `REDIS_PORT` | `6379` | Redis 映射到宿主机的端口；被占用时可改（如 `6380`） |
| `MYSQL_PORT` | `3306` | 仅在取消 compose 中 mysql `ports` 注释后生效 |
| `NPM_REGISTRY` | `https://registry.npmjs.org/` | 构建参数：前端 `npm ci` 源，受限网络可换 npmmirror |
| `GOPROXY` | `https://proxy.golang.org,direct` | 构建参数：Go 模块代理，受限网络可换 goproxy.cn |

### 版本发布与回滚

发布由 `.github/workflows/release.yml` 完成：推送 `vX.Y.Z` 标签即自动验证、构建并推送镜像到 GHCR，同时创建 GitHub Release。
使用仓库内置 `GITHUB_TOKEN`（`packages: write`），**推送镜像无需配置额外密钥**；但**拉取方**仍需凭证，二选一准备：

- **设为 Public**：GitHub → 仓库 `Packages` → `meteor-cms` → `Package settings` → `Change visibility` → `Public`。
  Package 在**首次发版后才存在**，此项需发版完成后才能设置。
- **保持 Private**：在 `https://github.com/settings/tokens/new` 生成 classic PAT，**只勾 `read:packages`**（不要给写权限），建议设置过期时间。
  在生产机登录（`read -s` 避免 PAT 进入 shell history）：

  ```bash
  read -rsp 'GitHub PAT: ' PAT && echo
  printf '%s' "$PAT" | docker login ghcr.io -u <GitHub 用户名> --password-stdin
  unset PAT
  docker manifest inspect ghcr.io/yuwen002/meteor-cms:<版本>
  ```

  PAT 是账号级凭证，可在发版前提前创建；登录凭证由 Docker 自行保管（Linux 通常在 `~/.docker/config.json`，Docker Desktop 走凭据管理器），不要写进 `.env` 或提交到仓库。PAT 过期或轮转后需在生产机重新登录。

`make preflight` 默认会做这项镜像可达性检查；离线或镜像尚未发布时可设 `SKIP_IMAGE_CHECK=1` 跳过。

```bash
# 在已验证的 commit 上打标签触发发版
git tag -a v1.2.3 -m "release v1.2.3"
git push origin v1.2.3
# 产出：ghcr.io/yuwen002/meteor-cms:1.2.3 / :1.2 / :latest
```

生产部署在 `.env` 中固定版本（不要用 `latest`）：

```bash
CMS_IMAGE=ghcr.io/yuwen002/meteor-cms:1.2.3
docker compose pull && docker compose up -d
```

> `api` 服务仍保留 `build:`（`migrate` 复用同一镜像、不重复构建，否则两个服务会并发构建同一 tag 而报 `image already exists`），因此**必须先 `pull` 且不要加 `--build`**，否则会在本机重新构建并覆盖已发布镜像。

回滚只需把 `CMS_IMAGE` 改回上一个版本号并重复上述命令：

```bash
CMS_IMAGE=ghcr.io/yuwen002/meteor-cms:1.2.2
docker compose pull && docker compose up -d
```

> `migrate` 容器只在镜像或配置变化需要重建时执行，由 MySQL `GET_LOCK` 串行化；执行结果写入 `schema_migrations`，API 启动时会校验版本（详见「程序初始化与数据迁移」）。表结构同步坚持只增不减（`WithDropColumn(false)`），回滚镜像前请确认旧版本与当前表结构兼容，必要时临时设 `SCHEMA_CHECK_STRICT=false`；确需删列的破坏性变更须走显式版本化迁移并先备份数据。

### 后台维护任务

服务启动时会自动清理已过期的 token 黑名单记录，之后按 `Maintenance.TokenBlacklistCleanInterval`（默认 3600 秒）周期性执行，避免 `token_blacklists` 表随每次登出无限增长。

审计日志同样在启动时清理一次，之后每 24 小时按 `Maintenance.AuditLogRetentionDays`（默认 90 天，`<=0` 表示永久保留）删除过期的 `operation_logs` / `login_logs`。清理按每批 1000 条分批 DELETE，两张表独立记录错误；多副本部署时用 Redis 锁 `maintenance:audit_log_cleaner` 选主，避免重复清理，Redis 不可用时退化为单实例执行。

### 后台审计日志

后台「操作日志」「登录日志」由 `OperationLogMiddleware` 采集受保护接口与认证类接口调用；被登录限流拦截的请求另有补记链路写入 `login_logs`，登录成功/失败也独立留痕。两类日志均不支持修改或软删除。

每条操作日志包含：操作人、模块、动作、路径、`request_id` 链路标识、递归脱敏后的 JSON / 表单 / query 参数快照、业务码 `biz_code`、HTTP 状态码、结果与耗时。客户端可用 `X-Request-ID` 传入上游链路标识，服务端原样回写响应头。

日志写入统一走 `internal/pkg/audit` 的有界队列（4 worker / 1024 队列 / 单条 3s 超时）：队列满时丢弃并记 error 日志而不是拖垮业务请求，单条 panic 不会带崩 worker，服务停止时 `audit.Shutdown` 会等待在途记录落库。

排查要点：

- `status` 为可选参数，**不传**表示不筛选，传 `status=0` 才表示只看成功；
- `end_time=YYYY-MM-DD` 会包含当天 23:59:59.999999999，不会漏掉当天日志；
- 只有两个列表查询接口与验证码获取不写操作日志，详情 / 导出 / 统计接口本身都会被审计；
- 详情抽屉通过 `GET /admin/logs/operations/:id` 拉取全量字段（含 `params`、`query`、`request_id`、`user_agent`、`error_msg`），复用列表权限；
- 「统计」抽屉展示近 7 天按天 / 模块 / 失败业务码的聚合；「导出 CSV」按当前筛选条件导出，单次上限 10000 条，受 `system:log:export` 控制；
- 数据权限三档：超管可看全量，持有 `system:log:view_all` 的审计员也可看全量，其余用户只能看自己；`user_id` 仅对前两档生效。

## 配置存储、数据字典与站点扩展

**系统配置**（`sys_configs`）：分组、值类型（string/text/int/bool/json/url/image）、正则校验、排序、启停、内置保护、密文标记与变更版本。`mail_smtp_password` 等密文项由 `internal/pkg/secretbox` 以 AES-GCM 加密（密文带 `enc:v1:` 前缀，密钥从 `Auth.AccessSecret` 派生），列表统一返回 `******`，审计参数快照会过滤 `config_value`。历史明文值可平滑读取，不强制迁移。

配置统一从 `internal/pkg/settings` 读取（TTL 30s 进程内缓存 + 写后失效），前端配置页支持分组标签、批量保存、按 key 取值、详情、变更历史与一键回滚；配置值可用指针字段显式清空。

**数据字典**（`sys_dicts` / `sys_dict_items`）：分组、内置保护、层级 `parent_id`、默认项、启停、备注；删除字典在单事务内级联删除字典项并校验父项引用。`internal/pkg/dicts` 提供只读缓存，公开白名单接口 `GET /api/dicts/:code` 只返回启用字典的启用项。业务可配枚举（配置分组、值类型、文章来源、链接类型等）走字典驱动；发布状态、审核状态等流程状态机继续硬编码，避免配置误改破坏状态约束。

**公开站点信息**：`GET /api/site` 按字段白名单输出站名、Logo、描述、关键词、ICP、页脚、语言、标题模板、主题与主题色，绝不回传全量配置。SPA 首屏 meta、RSS、sitemap、robots、前台页眉/页脚/主题色都由该配置驱动；`site_url` 可覆盖 `WEB_BASE_URL`，`seo_robots_disallow` / `seo_robots_sitemap` 控制 robots 输出。`site_footer` 支持 `{year}`（当年）、`{site_name}` / `{site}`（站名）占位符：后台保存原始模板便于编辑，`GET /api/site` 返回展开后的文本；写死年份的自定义页脚原样展示，因此跨年后不会再出现过期版权年份。

**周边能力**：

| 能力 | 入口 | 说明 |
| --- | --- | --- |
| 内容导入导出 | `GET /admin/content/export`、`POST /admin/content/import` | 可移植 JSON 快照（分类/标签/文章/明文配置/字典），slug 为稳定标识，单事务幂等导入；`overwrite=true` 覆盖同 slug 文章。密文配置不导出 |
| 逻辑备份 / 恢复 | `make backup [FILE=...]`、`make restore FILE=... [OVERWRITE=true]` | `cmd/backup` 复用内容包，不含管理员密码哈希、令牌黑名单与审计日志；物理备份仍需数据库 dump |
| 媒体元数据 | `media_files` 表 + 文件管理页 | 上传时探测真实 MIME / 原文件名 / 图片宽高，展示文章引用篇数；被引用文件禁止删除与重命名 |
| 全文检索 | `GET /api/articles?keyword=`、`GET /api/search?keyword=` | 标题 + 摘要 + 正文 LIKE 匹配，仅返回已发布文章；数据量大时应改 FULLTEXT 或外部搜索引擎 |
| 运行监控 | `GET /admin/monitor` + 控制台面板 | 运行时长、goroutine、堆内存、GC、请求总数 / 5xx / 在途 / 平均 QPS、审计队列积压与丢弃、DB/Redis 状态 |
| Prometheus 抓取 | `GET /metrics` | 默认关闭；`METRICS_ENABLED=true` 开启，可配 `METRICS_TOKEN` 要求 Bearer 鉴权；输出 Prometheus 文本格式，无第三方依赖 |
| 站点扩展 | 后台「系统管理 → 站点扩展」 | 友链、评论（层级 + 审核）、邮件订阅；开关由 `feature_*` 配置控制，公开接口不挂 JWT |

**Prometheus 抓取**：`GET /metrics` 默认关闭（关闭时返回 404），开启后输出 `meteor_cms_requests_total`、`_requests_server_errors_total`、`_requests_in_flight`、`_uptime_seconds`、`_go_goroutines` / `_go_heap_alloc_bytes` / `_go_sys_bytes` / `_go_gc_total`、`_audit_queue_depth` / `_audit_queue_dropped_total`，以及带 `version` / `commit` / `build_time` / `schema_version` / `go_version` 标签的 `_build_info`。全部是瞬时快照，不引入客户端依赖；`make preflight` 会检查开启时的 token 配置。Prometheus 侧示例：

```yaml
scrape_configs:
  - job_name: meteor-cms
    metrics_path: /metrics
    authorization:
      type: Bearer
      credentials: <METRICS_TOKEN>
    static_configs:
      - targets: ["api:8888"]
```

不设 `METRICS_TOKEN` 时端点免鉴权，仅适合抓取端与 API 同内网、由网络层隔离的场景；但 `STRICT_SECURITY=true` 会把「开启却未设 token」升级为启动失败，因此生产严格模式下应始终配置 token。

全部手工注册路由都有 OpenAPI 文档源（`ops.doc.api`），`go test ./cmd/apidoc` 会以 Go 路由注册为基准做覆盖校验。

## 程序初始化与数据迁移

**启动链路**：`docker compose up -d` 按 `mysql 健康 → migrate 容器完成 → api 启动` 串行执行。API 进程启动时会依次完成：配置加载 → 环境变量覆盖 → `Validate()` 校验 → 数据库连接池创建与 `Ping` → schema 版本校验 → 缓存预热 → 路由与中间件注册 → 后台维护任务 → `server.Start()`。任一步失败都会明确退出，不会留下「启动成功但一请求就报 SQL 错」的实例。

**配置覆盖**：所有敏感项都可通过环境变量注入，未设置时沿用 `api/cms/v1/etc/cms-api.yaml`。配置不会自动映射环境变量，新增开关必须在 `cms.go` 里显式读取。

| 环境变量 | 用途 |
| --- | --- |
| `MYSQL_DSN` / `REDIS_HOST` / `REDIS_PASS` | 数据库与 Redis 连接 |
| `AUTH_ACCESS_SECRET` / `AUTH_REFRESH_SECRET` | JWT 签名密钥；**生产必须替换**。更换 `AccessSecret` 会导致已有密文配置无法解密 |
| `AUTH_ACCESS_EXPIRE` / `AUTH_REFRESH_EXPIRE` / `AUTH_REFRESH_GRACE_SECONDS` | 令牌有效期与轮换宽限期 |
| `DB_MAX_OPEN_CONNS` / `DB_MAX_IDLE_CONNS` / `DB_CONN_MAX_LIFETIME` | 连接池参数（默认 20 / 10 / 3600s） |
| `SQL_DEBUG` | 默认关闭；打开会把每条 SQL 及参数写入日志 |
| `STRICT_SECURITY` | `true` 时把「示例密钥 / 关闭限流」等告警升级为启动失败 |
| `SCHEMA_CHECK_STRICT` | 默认 `true`，启动时校验 `schema_migrations`；紧急回滚可临时设 `false` |
| `METRICS_ENABLED` / `METRICS_TOKEN` | Prometheus `/metrics` 默认关闭；开启后建议设置长度 ≥16 位的 token，否则严格模式拒绝启动 |
| `MAIL_*` / `WEB_BASE_URL` / `MAINTENANCE_*` | 邮件、站点地址与维护策略 |
| `LOGIN_*` / `PUBLIC_RATE_LIMIT_*` | 登录与公开接口防护（完整列表见 `.env.example`） |

启动日志会打印构建版本、脱敏后的生效配置摘要与安全告警；`go run ./cmd/migrate -version` 可查看版本与期望的 schema 版本。

**迁移工具**：

```bash
make migrate              # 完整迁移：结构同步 + 单事务种子 + 版本记录（幂等）
make migrate-check        # 只读对比 information_schema 与 Ent 表定义，有差异退出码 2
make migrate-dry-run      # 输出待新增表/列与期望索引清单，不执行 DDL
make migrate-schema       # 只同步表结构（-skip-seed）
make migrate-seed         # 只重跑种子（-only-seed）
make migrate-backfill     # 回填历史上传文件的媒体元数据
go run ./cmd/migrate -require-backup              # 校验 CMS_BACKUP_FILE 备份新鲜度后才继续
go run ./cmd/migrate -timeout 600 -f /path/to.yaml
```

- `schema_migrations` 表记录 `version` / `checksum` / `duration_ms` / `applied_at`。API 启动时比对版本，`readyz` 也读取它；漏跑迁移的实例不会被接流量。
- 迁移用 MySQL `GET_LOCK('meteor_cms_schema_migrate')` 串行化，多副本或重复 `compose up` 不会并发写种子；种子整体在**单个事务**里提交，失败即回滚，不会留下半初始化状态。
- `admin_permissions` 上有 `(type, permission, method, api_path)` 唯一索引。升级前 CLI 会先清理历史重复行（同时迁移角色绑定），否则建索引会失败。`go test ./internal/seed` 会在静态层面拦截清单自身的重复键。
- `Schema.Create` 仍坚持「只增不减」（`WithDropColumn(false)`）。`make migrate-backfill` 为媒体库上线前已上传的文件补齐 `media_files` 记录；`InitConfigs` / `InitDicts` 同时补齐历史行的 `config_group` / `value_type` / `dict_group` 等新增列，且**不覆盖运营已改的值**。
- `cmd/migrate` 与 `cmd/backup` 共用 `internal/config.ResolveDSN`，优先级 `MYSQL_DSN > -f > CMS_CONFIG > 容器内 /app/etc > 仓库相对路径`；配置加载失败会明确报错，不再静默回退到硬编码弱口令 DSN。
- 旧库升级路径由 `make test-integration`（即 `go test -tags=integration ./test/migration/...`，需 `MYSQL_DSN`）覆盖：构造缺列的上一版 `sys_configs` 与历史显示名，依次验证漂移检查能发现缺列 → Ent 补列 → 种子把站名 / 页脚迁移到 `Meteor CMS` / `© {year} Meteor CMS` 且不覆盖运营自定义值 → 重跑幂等 → `schema_migrations` 记录后启动校验放行。用例在独立库 `meteor_cms_upgrade_test` 中运行并自动清理，CI 的 `migration` job 每轮都会执行。

**探针与关闭**：`/healthz` 是存活探活（DB + Redis 连通性），`/readyz` 额外校验 schema 版本并返回构建信息。服务停止时按「后台维护任务（WaitGroup，最多等 10s）→ 审计写入队列（5s）→ 数据库连接」的顺序收尾，每步都有超时兜底。

**生产检查清单**：复制 `.env.production.example` 为 `.env` 并逐项填写（`CHANGE_ME` / `REPLACE_WITH` 未替换会被判定为失败）：

```bash
cp .env.production.example .env
openssl rand -base64 48   # 生成 AccessSecret
openssl rand -base64 48   # 再生成一次作为 RefreshSecret，两者必须不同
# 编辑 .env：填入两个密钥、数据库强口令（同步修改 MYSQL_DSN）、CMS_IMAGE 的版本号
```

然后按顺序执行：

1. `make backup` —— 对当前生产库导出内容快照，把生成路径填入 `.env` 的 `CMS_BACKUP_FILE`；
2. `make preflight` —— 自动校验两个 JWT 密钥、`STRICT_SECURITY`、安全开关、DSN/口令、`CMS_IMAGE` 版本、备份新鲜度（24 小时）与 GHCR 拉取凭证，任一 FAIL 则退出码 1；
3. `make migrate-check` 确认表结构无漂移 → `make migrate-safe`（带 `-require-backup`）执行迁移；
4. `docker compose pull && docker compose up -d`（**不要加 `--build`**）→ 确认 `/readyz` 返回 200，也可用 `READY_URL=http://127.0.0.1:8888/readyz make preflight` 复查；
5. 首次登录立即修改 `admin/123456`。

`preflight` 可用 `ENV_FILE=` 指定其它 `.env` 路径，`SKIP_IMAGE_CHECK=1` 跳过镜像可达性检查。它的检查项与 `STRICT_SECURITY=true` 时 API 的启动校验逐项对齐，避免「检查通过但服务拒绝启动」。`make` 入口面向 Linux 生产机；Windows 本地未装 WSL 时可用 Git Bash 直接执行 `bash scripts/preflight.sh`。

> **首次部署**（仓库还没有 tag、GHCR 上还没有镜像、数据库也尚未建立）：备份与镜像两项检查此时都没有对象，可先执行
> `SKIP_IMAGE_CHECK=1 SKIP_BACKUP_CHECK=1 make preflight` 只校验配置；随后按「版本发布与回滚」打 `vX.Y.Z` 触发首次发版，
> 等镜像可拉取、`migrate` 建好库表并 `make backup` 导出首份快照后，再执行完整 `make preflight` 和上面的部署流程。
> 两个跳过开关**仅供首次部署**，常规升级不得跳过。

### 关于项目命名与兼容性

项目已从 `go-meteor-cms` 更名为 `meteor-cms`：Go module（`github.com/yuwen002/meteor-cms`）、Docker 镜像 / 容器名、npm 包名、OpenAPI 标题、站点显示名 `Meteor CMS`、文档与代码生成模板都已统一。以下几项**刻意保留旧名**，改动会直接破坏已有数据，请勿「顺手清理」：

| 保留项 | 原因 |
| --- | --- |
| 数据库名 `go_meteor_cms` | 现有数据卷里的库名；改 DSN 会连到一个空库，看起来像数据丢失。如需改名，先 `make backup` 导出内容包，改 `MYSQL_DATABASE` + `MYSQL_DSN` 后新建库并恢复 |
| 数据卷 `go-meteor-cms_mysql_data` | `docker-compose.yml` 已显式声明卷名，因此重命名仓库目录 / compose 项目名后仍复用同一份数据。改卷名等于换一个空库 |
| compose 项目名 `go-meteor-cms` | 默认取自仓库目录名。本地工作区目录仍是 `go-meteor-cms`（IDE 依赖该路径）；如需重命名目录，建议先 `docker compose down` 并确认卷名仍指向上面那一个 |

已有数据库里的站点显示名会随 `make migrate`（或 compose 的 migrate 容器）自动更新：种子只在 `site_name` / `site_footer` 仍精确等于历史默认值（`Go Meteor CMS` / `© 2026 Go Meteor CMS`，以及更名后、年份模板化前写入的 `© 2026 Meteor CMS`）时改成 `Meteor CMS` / `© {year} Meteor CMS`；运营自定义过的站名与页脚不会被覆盖。更新后 `settings` 服务的进程内缓存会在 API 启动时预热，前台标题、RSS、页脚随即生效。

**采用方案 A：GitHub 仓库随之改名**。源码里的 module path、badge、`git clone` 地址与 GHCR 路径都已经指向新名，因此仓库外部状态必须同步，否则会停留在「代码指向 `meteor-cms`、远端仍是 `go-meteor-cms`」的半改名状态。以下四步无法由代码自动完成，请按顺序执行：

1. 在 GitHub 把仓库 `yuwen002/go-meteor-cms` 改名为 `yuwen002/meteor-cms`（GitHub 会对旧地址保留一段时间的重定向）。
2. 本地更新远端地址并确认：

   ```bash
   git remote set-url origin git@github.com:yuwen002/meteor-cms.git
   git remote -v
   ```

3. 在已验证的 commit 上推新标签 `git tag -a vX.Y.Z -m "..." && git push origin vX.Y.Z` 触发发版。`.github/workflows/release.yml` 在发版前会校验 `github.repository == yuwen002/meteor-cms` 且 `go.mod` 的 module path 与仓库路径一致；仓库未改名会直接失败，避免把镜像推到旧路径 `ghcr.io/yuwen002/go-meteor-cms`。
4. 生产 `.env` 把 `CMS_IMAGE` 指向新发布的 `ghcr.io/yuwen002/meteor-cms:<版本>`，执行 `docker compose pull && docker compose up -d`。仍在使用旧 tag 的环境保留一段时间，或在新仓库重新发布一份等价 tag 作为过渡。

> 在步骤 1 完成前，README 顶部的 Go Report Card badge、`git clone` 地址与 ghcr 示例都会 404，`git push` 也会失败——这是预期的过渡状态，不要因此把源码里的新名回退。若最终决定不改仓库名，则必须改走方案 B：把 module path、badge、clone、ghcr 路径全部改回 `go-meteor-cms`，只保留显示名 `Meteor CMS`。

`.idea/`（IDE 本地配置，含绝对路径与旧 module 名）不随源码提交，IDE 重新读取 `go.mod` 即可；`release.yml` 的 `IMAGE_NAME` 取自 `github.repository`，无需在代码里改镜像名。

## 性能基准

`make bench` 以 `-run=^$ -bench=. -benchmem -benchtime=200x` 跑全部基准（跳过单测），默认只迭代 200 次，便于在 CI 里快速冒烟；需要稳定数值时用 `make bench BENCHTIME=2s`，并固定机器负载再对比。CI 会用 `-benchtime=1x` 编译并执行一遍所有基准，防止基准代码随重构腐坏。

| 基准 | 位置 | 覆盖路径 |
| --- | --- | --- |
| `BenchmarkExpandSiteText` | `internal/common` | 公开 `GET /api/site` 每次请求展开页脚模板 |
| `BenchmarkNormalizeSlug` / `BenchmarkGenerateSlug` | `api/cms/v1/internal/logic` | 文章发布与导入时的 slug 规范化 |
| `BenchmarkValidatePasswordComplexity` | `internal/utils` | 创建 / 注册 / 改密入口的逐字符校验 |
| `BenchmarkCommentRateLimitMemoryFallback` | `api/cms/v1/internal/middleware` | Redis 故障时公开评论写入的内存限流兜底 |

bcrypt 哈希与 Redis 网络路径刻意不纳入基准：前者是刻意慢哈希，后者测的是外部服务，数值不可复现。数据库与缓存的读写压测请用真实栈跑 `go test -tags=e2e ./test/e2e/...` 或外部压测工具，不要写成依赖本机端口的单测。

## 贡献指南

欢迎提交 Issue 和 Pull Request。

1. Fork 项目
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 提交 Pull Request

## 许可证

[MIT](LICENSE) © 2023 Your Name

## 致谢

- [Go-Zero](https://go-zero.dev/)
- [Ent](https://entgo.io/)
- [Go](https://golang.org/)