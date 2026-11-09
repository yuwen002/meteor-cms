# 运维 Runbook（本地联调 / 生产部署 / 故障排查）

本文档面向开发与运维，覆盖环境搭建、健康检查与常见故障排查。与 `README.md` 的"快速开始"互补：`README` 讲特性与接口，`runbook` 讲"怎么跑起来、出问题怎么查"。

---

## 1. 架构概览

```
浏览器 ──(5173, Vite 代理 /admin,/api,/uploads)──▶ Go-Zero API(:8888) ──▶ MySQL
                                  ▲
                          前端静态资源(web/dist, 生产可反向代理)
```

- **后端**：Go-Zero，默认 `:8888`，JWT 鉴权，RBAC（菜单/按钮/API 三级）。
- **前端**：Vue3 + Element Plus，开发态 `:5173`，生产构建为静态文件。
- **数据库**：MySQL（Ent ORM），启动时由 `cmd/migrate` 自动建表 + 种子数据。

---

## 2. 本地开发

### 2.1 数据库

```sql
CREATE DATABASE go_meteor_cms CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

DSN 优先级：`MYSQL_DSN` 环境变量 > `api/cms/v1/etc/cms-api.yaml` 的 `Database.Source` > 内置默认值。

### 2.2 迁移（建表 + 种子，幂等）

```bash
go run cmd/migrate/main.go
# 期望输出：📊 权限统计（菜单 / 按钮 / API 三类数量，以实际种子为准）
```

> 种子含默认超管 `admin / 123456`，生产环境务必修改密码。

### 2.3 启动后端

```bash
go run ./api/cms/v1 -f api/cms/v1/etc/cms-api.yaml
```

### 2.4 启动前端

```bash
cd web
npm install
npm run dev          # http://localhost:5173
```

`web/vite.config.ts` 已将 `/admin`、`/api`、`/uploads` 代理到 `http://localhost:8888`，因此联调时后端须在 8888 运行。

### 2.5 验证

浏览器打开 `http://localhost:5173`，用 `admin / 123456` 登录（需输入图形验证码）。登录后应看到左侧动态菜单（控制台、管理员管理、部门管理、角色管理、权限管理、操作日志、登录日志、文件管理、系统管理【字典管理 / 系统配置】）。

---

## 3. 生产 / 容器部署

### 3.1 Docker Compose（推荐）

compose 已内置 `redis` 服务（登录限流 / 全局踢人共用），无需额外添加；容器内连接由 `.env` 的
`MYSQL_DSN`、`REDIS_HOST` 注入，配置项说明见 `README.md` 部署章节与 `.env.example`。

```bash
cp .env.example .env              # 可选：按需修改端口、口令、构建镜像源
docker compose up -d --wait        # MySQL(健康检查) → 迁移 → API
docker compose down
```

### 3.2 编译部署

```bash
go build -o bin/cms ./api/cms/v1
go build -o bin/migrate ./cmd/migrate

export MYSQL_DSN="root:123456@tcp(127.0.0.1:3306)/go_meteor_cms?parseTime=True&loc=Local"
./bin/migrate
./bin/cms -f api/cms/v1/etc/cms-api.yaml
```

### 3.3 环境变量

| 变量 | 说明 |
| --- | --- |
| `MYSQL_DSN` | 数据库连接串，覆盖配置文件 `Database.Source`，容器化部署推荐 |
| `REDIS_HOST` | Redis 地址，容器内置为 `redis:6379`；未设置时按配置文件默认 `127.0.0.1:6379` |
| `WEB_DIST` | 前端产物目录，容器内置为 `/app/web/dist`；本地留空则仅暴露 API |

compose 专用配置（`API_PORT` / `REDIS_PORT` / 镜像源构建参数等）完整清单见 `README.md` 的「环境变量」表。

### 3.4 后台维护

服务启动后自动清理过期 token 黑名单，按 `Maintenance.TokenBlacklistCleanInterval`（默认 3600s）周期执行。审计日志按 `Maintenance.AuditLogRetentionDays`（默认 90 天）在启动时清理一次、之后每 24 小时清理一次；设为 `0` 可永久保留。清理按每批 1000 条分批 DELETE、两张表独立报错，多副本用 Redis 锁 `maintenance:audit_log_cleaner` 选主；Redis 不可用时退化为单实例执行。

排查后台操作时进入「系统管理 → 操作日志 / 登录日志」：

- 按「结果」筛选时，`status=0`（成功）与不筛选是两种语义，接口已用可选指针区分；
- 结束日期填纯日期时包含当天全部记录，不必手动补 `23:59:59`；
- 每条日志带 `request_id`（响应头 `X-Request-ID`），可用它把操作日志、登录日志与运行日志串起来；详情抽屉同时展示 JSON 参数与 query 快照（敏感键递归脱敏）；
- 「操作」列由请求路径与方法推断，「统计」抽屉按天 / 模块 / 失败业务码定位异常，「导出 CSV」按当前筛选导出（上限 10000 条）；
- 只有两个列表查询与验证码获取不产生操作日志，详情 / 导出 / 统计自身会被审计；
- 数据权限三档：超管、持有 `system:log:view_all` 的审计员可看全量，其余用户只能看自己；`user_id` 仅对前两档生效；
- 日志写入走有界队列，队列满会丢弃并在服务日志留 error；服务停止会等待在途记录落库，若出现「等待审计日志写入超时」需检查数据库。

### 3.5 Redis 与 CORS 配置（鉴权增强）

登录限流默认开启（`Login.RateLimit.Enabled=true`），采用**分布式固定窗口限流**：

- **Redis（多实例/容器部署必需）**：在 `cms-api.yaml` 配置 `Redis` 块：
  ```yaml
  Redis:
    Host: "127.0.0.1:6379"
    Type: "node"
    Pass: ""
    DB: 0
  ```
  - `Host` 为空时，登录限流自动降级为**单实例内存模式**（重启即清空、多实例不共享，仅适合单机开发）。
  - Redis 连接失败/抖动时同样降级为内存模式并打 error 日志，**防护不失效**（不会因 Redis 故障导致全员无法登录）。
  - 生产建议独立 Redis 实例或 Sentinel/Cluster（`Type` 可设 `cluster`），并配置 `Pass` 与网络隔离。
- **CORS（前端独立域名部署时必需）**：`cms-api.yaml` 的 `Cors` 列表配置允许的前端源：
  ```yaml
  Cors:
    - "http://localhost:5173"      # 本地 Vite 联调
    - "https://admin.example.com"
  ```
  - 通过 go-zero 内置 `rest.WithCors` 启用，预检 `OPTIONS` 由框架统一返回 204 并带 CORS 头。
  - 列表含 `"*"` 表示允许任意源（仅适合无 cookie 凭据、纯 token 鉴权场景）。
  - **未配置的源不会收到 `Access-Control-Allow-Origin`，浏览器将拒绝跨域请求**。
  - 同源部署（本服务用 `StaticDir` 直接托管 SPA、或反向代理同源）可不配 `Cors`。

---

## 4. 健康检查

| 检查项 | 方法 | 正常表现 |
| --- | --- | --- |
| 后端进程 | `netstat`/`ss` 看 `:8888` 监听；或 `docker ps` | 端口已监听 / 容器 Up |
| 接口探活 | 带 token `GET /admin/profile/permissions` | `code=0`，返回 `menus` 与 `permissions` |
| 数据库 | 迁移日志 `✅ 数据库迁移完成`、`✅ 数据初始化完成` | 无报错 |
| Redis | 多实例部署时执行 `redis-cli -h <host> ping` 应返回 `PONG`；限流 key 形如 `login_rl:<ip>` | `PONG`；限流生效 |
| 前端 | 浏览器打开页面，能登录并看到菜单 | 菜单正常渲染，无控制台报错 |

> `GET /admin/test-token` 不参与 API 权限校验，但需要有效 token 才能解析 claims，用于验证令牌本身是否有效。

---

## 5. 故障排查矩阵

| 症状 | 可能原因 | 排查 / 修复 |
| --- | --- | --- |
| 仪表盘空白 / 数据不显示 | 该接口响应缺少 `{code,msg,data}` 信封，前端拦截器（`res.code===0`）走错误分支 | 确认 handler 用 `common.Ok` 而非 `httpx.OkJsonCtx`（已修 `d8643a8`）。所有响应必须是统一信封 |
| 非超管访问某接口返回 **403** | 该路由没有对应的 `Type=3` API 权限记录（fail-closed，见 `PermissionMiddleware`） | 在 `internal/seed/permissions.go` 补一条 `Method`+`APIPath` 匹配的 API 权限，重新 `go run cmd/migrate/main.go` |
| 侧边栏某子菜单消失 / 整组丢失 | `buildMenuTree` 父子挂载顺序问题（已修 `e4998ad`）；或菜单 `sort` 与父子关系配置错误 | 确认菜单按 `sort` 升序且父菜单 `parent_id` 正确；复跑迁移 |
| 菜单图标异常（显示默认图标而非 X） | icon 语义键未命中前端 `MenuItem.vue` 的 `iconMap` | 在 `iconMap` 补充语义键；注意未命中只会回落为 `Menu` 图标，**不会**显示 X |
| 前端接口 404 / 网络错误 | Vite 代理未生效或后端未起 | 确认后端 `:8888` 在跑；确认 `web/vite.config.ts` 代理 `/admin`、`/api`、`/uploads` 存在 |
| 登录失败 | 验证码错误/过期、账号被禁用、密码错误 | 重新 `GET /api/captcha` 取新码；检查 `admin_users.is_active` 与密码 |
| 忘记密码邮件收不到 | `Mail.Enabled=false` 时只输出到服务日志，不真发 | 查看 API 服务日志拿重置令牌；或将 `Mail` 节点配置为启用 |
| 数据库迁移失败 | DSN 错误、MySQL 用户权限不足、表锁 | 检查 `MYSQL_DSN` / `cms-api.yaml` 的 `Database.Source`；确认 MySQL 用户有建库建表权限 |
| 文件上传失败 | `uploads` 目录无写权限、超出大小/类型限制 | 检查服务运行目录的 `uploads/` 写权限与文件配置项 |
| 非超管列表数据为空 | 数据权限 fail-closed（当前仅在管理员列表生效）：角色未分配 / 分配部门缺失 / 范围解析失败 → 不返回数据 | 检查用户角色 `data_scope` 与 `dept_id`，以及角色自定义部门（`admin_role_depts`） |
| 登录接口频繁返回 `code=2029`（请求过于频繁） | 同一 IP 在窗口内尝试次数超过 `Login.RateLimit.Max`（默认 10/60s）被限流；或 Redis 未配置导致各实例独立计数 | 等待 `BlockSeconds`(默认 300s) 后重试；多实例务必配置 `Redis` 使计数集中；确认 `Login.RateLimit.Enabled` |
| 多实例部署下登录限流形同虚设 | 未配置 `Redis`，各副本内存独立计数 | 在 `cms-api.yaml` 配置 `Redis.Host` 指向共享 Redis |
| 前端跨域报错（CORS / 预检失败） | 未配置 `Cors`，或前端源不在白名单 | 在 `cms-api.yaml` 的 `Cors` 加入前端实际源（含协议与端口） |
| 带 token 的跨域请求被拦（无 `Access-Control-Allow-Origin`） | 预检/实际请求的 `Origin` 不在 `Cors` 白名单 | 将前端源加入 `Cors`；确认前端实际使用的 `http://x:port` 与配置一致 |

---

## 6. 关键约定（改代码前必读）

1. **统一响应信封**：所有接口返回 `{code,msg,data}`，HTTP 状态码恒 200，业务结果看 `code`（`0` 成功，非 `0` 见 `internal/common/error_code.go`）。handler 统一用 `common.Ok` / `common.Fail`。
2. **分页**：`page` 缺省按 1；`page_size` 缺省取接口默认、上限 100（超出截断为 100）。
3. **权限三级**：菜单(1) / 按钮(2) / API(3)。按钮与 API **共用同一权限标识**（如 `system:role:add`），授权一次即可同时控制按钮显隐与接口可达。
4. **API 权限 `api_path` 必须与 `routes.go` 真实路由一致**（含 method），否则非超管必 403。新增接口务必同步补种子权限。
5. **路由匹配**：按"静态段命中最多者优先"，`/admin/departments/tree` 不会被 `/admin/departments/:id` 抢占。
6. **数据权限**：`data_scope` 1~5（全部/本部门/本部门及子部门/仅自己/自定义部门）；多角色取并集；任一角色为"全部"则不过滤。管理员列表与详情按部门范围收敛（fail-closed：范围为空→空结果 / 越权→按不存在处理）；操作/登录日志对非超管按 `user_id` 收敛为仅本人。角色/权限/部门/字典/文件等系统配置默认全局可见。
7. **超管**：`admin_users.is_super=true` 跳过全部鉴权与数据过滤，便于调试但不应作为常规账户。
8. **动态菜单**：侧边栏由 `GET /admin/profile/permissions` 返回的菜单树驱动（图标语义键 + `sort` 排序 + `parent_id` 层级），不写死在前端。
9. **`is_super` 以数据库为准**：业务层判断超管一律查 DB 的 `admin_users.is_super`，**不信任 token claim**（claim 在权限变更后、旧 token 过期前仍携带旧值）。涉及 `adminupdatelogic` / `resetadminpasswordlogic` / `operationloglistlogic` / `loginloglistlogic`，统一经 `logic.currentUserIsSuper` 取数；权限中间件同理（见 `PermissionMiddleware`）。
10. **密码复杂度**：新密码须 ≥8 位且至少含「大写/小写/数字/特殊字符」中三类（`utils.ValidatePasswordComplexity`）；创建/注册/改密/重置（含他人重置、找回密码）全入口统一校验。**唯一例外是初始化种子**：`internal/seed/seed.go` 写入的是 `admin/123456`，它不满足该策略，因为种子直接构造 `AdminUser`、不经过业务入口校验。保留这个弱口令是为了本地开发与 E2E 开箱即用（E2E 也用该口令登录），启动日志会固定输出一条安全告警。生产环境首次登录后必须立即改密；`STRICT_SECURITY=true` 不会因这一项阻断启动，否则沿用默认种子的部署会全部起不来。
11. **全局踢人 / 会话批量吊销**：token 携带 `tv`（版本号），Redis `auth:tv:<userID>` 自增即吊销该用户全部旧令牌；触发点：禁用/删除账号、改密（含他人重置、找回密码）。未配置 Redis 时降级为不校验（与限流一致）。
12. **鉴权查库缓存**：权限中间件将「用户→角色→权限」集合与「API 路径→权限」解析按 60s TTL 内存缓存（`go-zero collection.Cache`）；`is_super`/`is_active` 仍每请求实时查库，保证权限变更即时生效。
13. **Redis 依赖**：登录限流、token 版本（全局踢人）依赖 `cms-api.yaml` 的 `Redis`；`Host` 为空则相应能力降级（限流→内存、踢人→不校验），多实例部署务必配置以保证防护一致生效。
14. **强制下线接口**：`PUT /admin/admin-users/:id/force-logout`（权限标识 `system:admin_user:force_logout`，超管专用）通过 `tokenversion.Bump` 抬高目标用户版本号，使其所有旧令牌在下次请求即被 JWT 中间件判为 `2012 登录状态已失效`。与「禁用/删除账号、改密/重置密码」触发的踢人同源；未配置 Redis 时该接口仍可达但会话不被实际吊销（与 P2-4 降级一致）。
15. **自我全设备下线**：`POST /admin/logout-all`（权限中间件已 bypass，任意登录用户可调用）抬高自身 token 版本号，注销自己全部会话（含当前令牌）。无需超管权限，与单点登出不同的是它一次清除所有设备会话。
16. **按部门强制下线**：`POST /admin/admin-users/force-logout-by-dept`（超管，`system:admin_user:force_logout_dept`）按 `dept_id` 批量抬高该部门全部管理员的 token 版本号；未配置 Redis 时降级为 no-op（仅直接隶属该 `dept_id` 的用户，不含子部门）。
17. **强制下线全部（除自己）**：`POST /admin/admin-users/force-logout-all`（超管，`system:admin_user:force_logout_all`）批量抬高除当前操作者外全部管理员的 token 版本号，避免把自己也踢下线；未配置 Redis 时降级为 no-op。

---

## 7. 回滚与定位

- **版本回滚**：在 `.env` 把 `CMS_IMAGE` 改回上一个已发布版本，执行 `docker compose pull && docker compose up -d`（发布与回滚流程详见 `README.md`「版本发布与回滚」）。`migrate` 每次启动幂等执行且只新增表/列（`WithDropColumn(false)`，不会自动 DROP COLUMN），回滚前仍须确认旧版本与当前表结构兼容；确需删列必须走显式版本化迁移并先备份数据。
- **schema 版本门禁**：新版 API 启动时会读 `schema_migrations` 并与程序内期望版本比对，不一致直接拒绝启动。回滚到旧镜像时若版本表里的版本比旧程序新，需要设 `SCHEMA_CHECK_STRICT=false` 临时放行，并人工确认旧代码不会访问新列；不要把该开关设为长期配置。`/readyz` 也会用同一份数据判定就绪，滚动发布时可据此阻断未迁移实例。
- 提交历史：`e4998ad`（动态菜单多级层级 + 子菜单丢失修复）、`d8643a8`（统一响应信封修复）。
- 接口一致性审计方法见 `docs/smoke-test.md` 第 6 节，建议改动 `routes.go` 或种子权限后复跑。

## 8. 附录：P2-4 全局踢人验证（需 Redis）

环境起 Redis 后，token 版本校验与「禁用/删除/改密/重置」触发点才会真正生效；否则自动降级为不校验（见第 13 条）。

> 该链路已有自动化测试覆盖：`go test ./internal/pkg/tokenversion` 与
> `go test ./api/cms/v1/internal/middleware -run TestJwtMiddleware_RealKick`
> （进程内 miniredis，无需真实 Redis / Docker）。以下步骤仅用于手工排查。

验证步骤（无需真实登录，用调试 token 即可）：

1. 起 Redis：`docker compose up -d redis`（见仓库 `docker-compose.yml`）。
2. 启动服务（yaml 中 `Redis.Host` 默认 `127.0.0.1:6379`）。
3. 用任意合法的用户级 token（claim 含 `user_id` 与 `tv`）访问受保护接口（如 `/admin/profile/permissions`），应正常返回。
4. 手动抬高该用户版本号（模拟「踢人」）：
   `docker exec meteor-cms-redis redis-cli SET auth:tv:<userID> 5`
5. 用同一枚 `tv` 仍为旧值（如 0）的 token 再访问 → 应返回 `2012 登录状态已失效`。
6. 用一枚 `tv=5`（与当前版本一致）的新 token 访问 → 应正常返回，证明「旧令牌失效、新令牌有效」。

说明：生产中踢人由 disable/delete/change-password/reset 自动 `INCR auth:tv:<userID>` 触发，无需手动操作；上述步骤仅用于回归验证。

---

## 9. 配置 / 内容备份 / 监控 / 站点扩展

### 9.1 系统配置与字典

- 内置配置与字典由 `internal/seed` 幂等补齐：只新增缺失键、只补 `is_system` 标记，**不会覆盖运营已改的值**。
- 配置值经 `internal/pkg/settings` 读取（30s TTL）；字典经 `internal/pkg/dicts` 读取。任何写接口成功后都会主动 `Invalidate()`，因此后台改完立即生效，无需重启。
- 密文配置用 `Auth.AccessSecret` 派生密钥加密（`enc:v1:` 前缀）。**更换 AccessSecret 会导致已有密文无法解密**，变更前先用 `GET /admin/configs/key/:key` 备份明文并重新写入。
- 配置改错可用 `GET /admin/config-versions/:id` 查历史，再 `POST /admin/config-versions/:id/rollback` 回滚；回滚本身也会写一条历史。
- 流程状态机（发布/审核/下线）不接字典，避免配置误改破坏状态约束；只有业务可配枚举走字典。

### 9.2 内容备份与恢复

```bash
make backup                         # 生成 backup/cms-content-<时间戳>.json
make backup FILE=/data/cms.json     # 指定输出路径
make restore FILE=/data/cms.json   # 幂等恢复（只补缺失）
make restore FILE=/data/cms.json OVERWRITE=true  # 覆盖同 slug 文章
```

这是**逻辑备份**，覆盖分类、标签、文章、明文配置与字典，不包含管理员密码哈希、令牌黑名单、审计日志、上传文件本体与密文配置；站点完整迁移还需单独备份数据库与 `uploads/` 目录。恢复在单事务内完成，失败自动回滚。内容包格式版本为 `1.0`，格式不兼容时导入会被拒绝。

后台「文章管理 → 导出内容 / 导入内容」使用同一份内容包，可用于跨实例迁移。

### 9.3 运行监控

`GET /admin/monitor`（`system:monitor:view`）返回 `status`（ok / degraded / unhealthy）、DB/Redis 状态、运行时长、goroutine、堆内存、GC、请求总数 / 5xx / 在途 / 平均 QPS、审计队列积压与累计丢弃。

排查建议：

- `audit_queue_depth` 持续接近 1024 或 `audit_queue_dropped` 增长 → 审计库写入变慢，检查 MySQL 负载与 `operation_logs` 索引；
- `status=degraded` → Redis 不可用，token 版本强踢与限流会降级，按第 6 节排查；
- `heap_alloc_mb` 持续上涨且 `num_gc` 不增 → 用 pprof 抓堆快照。

`/healthz` 仍只做探活，用于容器 healthcheck；监控指标不要接到探活上，避免指标抖动导致容器重启。

### 9.4 站点扩展模块功能开关

| 配置键 | 默认 | 作用 |
| --- | --- | --- |
| `feature_comment_enabled` | `false` | 关闭时 `POST /api/comments` 直接拒绝，前台不展示评论表单 |
| `feature_comment_review` | `true` | 新评论为待审；关闭后新评论直接可见 |
| `feature_link_enabled` | `true` | 关闭时 `GET /api/links` 返回空列表 |
| `feature_subscription_enabled` | `true` | 关闭时订阅接口拒绝写入 |

公开评论接口没有登录态，上线前务必确认限流策略与审核开关；`GET /api/comments/:slug` 只返回已通过评论且不暴露邮箱/IP。后台「系统管理 → 站点扩展」负责友链维护、评论审核与订阅管理。

---

## 10. 初始化与迁移排障

> **更名后的数据卷**：项目已更名为 `meteor-cms`（镜像、容器、module、站点显示名），
> 但数据库名 `go_meteor_cms` 与数据卷 `go-meteor-cms_mysql_data` 刻意保留——
> `docker-compose.yml` 已显式声明卷名，重命名仓库目录不会另建空库。
> 排障时若发现「部署成功但没有历史数据」，先执行
> `docker volume ls | grep meteor` 确认用的是哪一个卷，不要直接 `docker compose down -v`。

### 10.1 启动失败速查

| 现象 | 原因 | 处理 |
| --- | --- | --- |
| `配置校验失败: Database.Source 为空` | 容器既没设 `MYSQL_DSN` 也找不到配置文件 | 补 `MYSQL_DSN` 或 `CMS_CONFIG` |
| `数据库连通性检查失败` | DB 未就绪 / DSN 密码或主机名错 | 先看 `docker compose ps` 与 mysql healthcheck |
| `读取 schema_migrations 失败（请先执行 make migrate）` | 漏跑迁移，或版本表被删 | `make migrate`；紧急回滚可临时设 `SCHEMA_CHECK_STRICT=false` |
| `数据库 schema 版本 X 与程序期望 Y 不一致` | 换了镜像但没跑新迁移 | `make migrate` 后重启 API |
| `STRICT_SECURITY=true，存在 N 项安全配置问题` | 仍用示例密钥 / 关闭了限流 | 注入 `AUTH_*`、恢复限流开关 |
| `/healthz` 200 但 `/readyz` 503 | schema 版本不匹配或版本表读不到 | 按上面两条处理，滚动发布时该实例不应接流量 |

启动日志里有一行 `生效配置 ...` 是脱敏后的实际参数，排障时先比对它，不要直接看 yaml。

### 10.2 迁移并发与半初始化

`cmd/migrate` 用 MySQL `GET_LOCK` 串行化，等待 30 秒拿不到锁会明确报「另一个迁移任务正在执行」。若看到这条日志，先确认是否有滞留的 migrate 容器或另一个发布流程在跑，不要直接 `docker compose down -v`。

种子在单事务里提交，日志里的「初始化数据失败（事务已回滚，表结构已同步）」表示**表结构已经成功同步**，只是种子失败——此时可以用 `make migrate-seed` 单独重试，不需要重建库。旧版本在这里会让整个 API 起不来，现已修复。

### 10.3 升级到带唯一索引的版本

`admin_permissions` 新增了 `(type, permission, method, api_path)` 唯一索引。CLI 在 `Schema.Create` 之前会执行 `DedupePermissions`：保留每组最小 id、把角色绑定迁移过去、清理重复行。若旧库重复数据很多，这一步可能耗时较长，建议在维护窗口执行并先跑：

```bash
make backup
make migrate-check          # 只列出差异，退出码 2 表示需要迁移
go run ./cmd/migrate -dry-run
go run ./cmd/migrate -require-backup   # 需 CMS_BACKUP_FILE 指向 24 小时内的备份
make migrate
```

### 10.4 回填与回退

- 历史上传文件：`make migrate-backfill`，按 url 判重可重复执行；`UPLOAD_DIR=/data/uploads make migrate-backfill` 可换目录。
- 配置 / 字典新增列：每次 `make migrate` 的种子都会补齐仍为零值的 `config_group` / `value_type` / `validation` / `sort` / `is_secret` / `dict_group`，不会覆盖运营改过的分组、启停或配置值。
- 迁移工具只读预览不会写任何记录，`-check` 有差异时退出码为 2，可直接用于 CI 门禁。
