# 集成冒烟测试与接口权限审计

本文档记录一次全链路联调冒烟与 API 权限一致性审计的**过程、命令与结论**，可作为回归测试清单复跑。

- 关联提交：`e4998ad`（动态菜单多级层级 + 子菜单丢失修复）、`d8643a8`（统一响应信封，修复 dashboard/refresh 解析失败）
- 运行环境：Windows + PowerShell，后端 8888 / 前端 5173（Vite 代理 `/admin`、`/api`、`/uploads` → `:8888`）

---

## 1. 冒烟目标

1. 后端 + 前端真实跑起来，逐菜单模块接口验证可用。
2. 导出真实渲染的侧边栏 DOM，确认动态菜单（图标/排序/层级）在前端正确渲染。
3. 审计种子里 `Type=3` 的 API 权限 `api_path` 与后端真实路由是否一致（README 明确：不一致会导致非超管 403）。

---

## 2. 前置准备

```bash
# 1) 数据库迁移（幂等，输出权限统计与 schema 版本）
go run cmd/migrate/main.go
#   期望输出：✅ schema 版本已记录 ... 与 📊 权限统计：菜单 / 按钮 / API 三项（数量以实际种子为准）
#   只看「有没有报错」和 schema 版本行，不要用硬编码数量做断言

# 2) 启动后端
go run ./api/cms/v1 -f api/cms/v1/etc/cms-api.yaml
#   默认监听 http://127.0.0.1:8888

# 3) 启动前端（另开终端）
cd web && npm install && npm run dev
#   默认 http://localhost:5173
```

### 2.1 获取测试令牌

- **方式 A（浏览器）**：用超管 `admin / 123456` 登录。注意 `/admin/login` 需要图形验证码（先 `GET /api/captcha` 取 `captcha_id` + `captcha_base64`）。
- **方式 B（自动化，跳过验证码）**：用配置里的 `AccessSecret`（在 `api/cms/v1/etc/cms-api.yaml`）直接签发 JWT，claims 为：

  | 字段 | 类型 | 说明 |
  | --- | --- | --- |
  | `user_id` | `float64` | 用户 ID（JWT 标准库解码为 float64，如 `1`） |
  | `is_super` | `bool` | 是否超管，`true` 跳过全部鉴权与数据过滤 |
  | `exp` | `int64` | 过期时间 |

  示意签发程序（仅本地调试用，密钥取自 `cms-api.yaml` 的 `AccessSecret`）：

  ```go
  // cmd/smoketoken/main.go
  package main

  import (
      "fmt"
      "os"
      "time"

      "github.com/golang-jwt/jwt/v5"
  )

  func main() {
      secret := os.Getenv("ACCESS_SECRET") // 与 cms-api.yaml 的 AccessSecret 一致
      claims := jwt.MapClaims{
          "user_id":  float64(1),
          "is_super": true,
          "exp":      time.Now().Add(24 * time.Hour).Unix(),
      }
      t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
      s, _ := t.SignedString([]byte(secret))
      fmt.Println(s)
  }
  ```

  之后对所有请求头带 `Authorization: Bearer <token>` 即可。

---

## 3. 接口冒烟清单

所有接口统一返回 `{ code, msg, data }` 信封（HTTP 恒 200），`code=0` 表示成功。以下为本次实测结果（超管令牌）：

| 模块 | 接口 | 预期 | 实测 |
| --- | --- | --- | --- |
| 个人/菜单 | `GET /admin/profile/permissions` | code=0，含 `menus`(树) + `permissions` | ✅ 菜单树含 `系统管理`(child=2) |
| 仪表盘 | `GET /admin/dashboard/stats` | code=0 + `data` | ✅（修复后） |
| 管理员 | `GET /admin/admin-users?page=1&page_size=10` | code=0 + 分页 | ✅ |
| 部门 | `GET /admin/departments/tree` | code=0 + 树 | ✅ |
| 角色 | `GET /admin/roles?page=1&page_size=10` | code=0 | ✅ |
| 权限 | `GET /admin/permissions?page=1&page_size=100` | code=0 | ✅ |
| 操作日志 | `GET /admin/logs/operations?page=1&page_size=10` | code=0 | ✅ |
| 登录日志 | `GET /admin/logs/logins?page=1&page_size=10` | code=0 | ✅ |
| 文件 | `GET /admin/files` | code=0 | ✅ |
| 字典 | `GET /admin/dicts?page=1&page_size=10` | code=0 | ✅ |
| 字典项 | `GET /admin/dicts/items?dict_code=<code>&page=1&page_size=10` | code=0 | ✅ |
| 配置 | `GET /admin/configs?page=1&page_size=10` | code=0 | ✅ |

### 3.1 真实 CRUD 复测（带数据）

列表为空往往是"库里没有种子数据"，不是 bug。本次用超管令牌做了真实增查删验证：

```powershell
# 字典
POST /admin/dicts            body: {code:"smoke_gender", name:"冒烟-性别", status:1, sort:1}   -> code=0
GET  /admin/dicts?page=1&page_size=10                                                  -> 能查到 smoke_gender
# 字典项（dict_code 为必填，设计如此：查询指定字典下的项）
POST /admin/dicts/items      body: {dict_code:"smoke_gender", label:"男", value:"1", status:1} -> code=0
GET  /admin/dicts/items?dict_code=smoke_gender&page=1&page_size=10                    -> count=1
# 配置
POST /admin/configs          body: {config_key:"smoke_key", config_value:"v1", name:"冒烟配置"} -> code=0
GET  /admin/configs?page=1&page_size=10                                               -> 能查到 smoke_key
# 清理
DELETE /admin/dicts/<id>     -> 级联删除其字典项，code=0
DELETE /admin/configs/<id>   -> code=0
```

> 注意：`/admin/dicts/items` 不带 `dict_code` 列表会返回参数错误——这是**设计行为**（"查询某个字典下的项"），不是缺陷。

---

## 4. 发现的 Bug 与修复

### 4.1 响应信封缺失（已修，提交 `d8643a8`）

`dashboardstatshandler.go`、`refreshtokenhandler.go`、`cmshandler.go` 原先使用 `httpx.OkJsonCtx`，**直接写出 `resp`、缺少统一的 `{code,msg,data}` 信封**；其余接口使用 `common.Ok`。前端拦截器契约（`web/src/utils/request.ts`）是 `if (res.code === 0) return res.data`，缺少信封会走错误分支。

后果：

- 仪表盘页无法解析数据（空白）。
- `/admin/refresh` 同样缺少信封，导致 **access token 过期时无法无感刷新**（刷新逻辑 `doRefresh` 依赖信封内的 `access_token`/`refresh_token`）。
- `cmshandler.go` 虽未注册路由（死代码），当时也一并改为 `common.Ok` 保持一致；该 goctl 占位 handler 与其 `cmslogic.go`（返回 `"your-jwt-token-here"` 等假令牌）已在后续清理中删除，避免被误当成可用的登录入口。

修复：三处统一改为 `common.Ok(w, resp)`。修复后复测 `dashboard/stats` 返回 `code=0` + `data`，刷新逻辑恢复正常。

### 4.2 子菜单整体丢失（历史，已修 `e4998ad`）

`buildMenuTree` 原实现边收集根节点边挂载子节点，而 `menus` 按 `sort` 升序（父菜单在前、子菜单在后），父节点被**值拷贝**进根列表时 `Children` 仍为空，后续子节点 `append` 只改了 map 中的指针，副本不更新，结果父菜单 `children` 为空、子菜单被 `continue` 后**整体丢失**。已改为"先挂载子节点、再收集根节点"两趟处理。该 Bug 在菜单全平铺时不可见，一旦加父菜单即触发。

---

## 5. 前端渲染验证（侧边栏 DOM 导出）

用无头 Chrome 注入超管令牌并导出真实渲染 DOM：

```powershell
# 临时跳板页 web/public/dev-login.html：写入 token 后跳 /dashboard
& "C:\Program Files\Google\Chrome\Application\chrome.exe" `
  --headless=new --disable-gpu --no-sandbox --virtual-time-budget=9000 `
  --dump-dom "http://localhost:5173/dev-login.html" > dom.html
```

DOM 结构确认：

```html
<li class="el-sub-menu">…<span>系统管理</span><i class="el-sub-menu__icon-arrow">…</i>
  <ul class="el-menu el-menu--inline" style="…--el-menu-level:1…">
    <li class="el-menu-item"><i class="el-icon"><svg…/></i>字典管理</li>
    <li class="el-menu-item"><i class="el-icon"><svg…/></i>系统配置</li>
  </ul>
```

- 父级带展开箭头、子菜单位于 `--el-menu-level:1` 内联 `<ul>` 中。
- 图标为内联 SVG，**未出现 X 元素**。
- 未配置的 icon 语义键会回落为 `Menu` 图标（仅控制台提示一次），不会显示 X。

---

## 6. API 权限一致性审计（②）

### 6.1 方法

自动比对两份清单（含 `:id` 路径参数归一化与 method 匹配）：

- **受保护路由**：`api/cms/v1/internal/handler/` 下所有 `*routes.go` 中注册的全部路由。
- **种子 API 权限**：`internal/seed/permissions.go` 中所有 `Type: PermissionTypeAPI`（即 `Type=3`）记录，取 `Method + APIPath`。

> 两侧条数都会随功能扩展变化（本次迁移输出为菜单 17 / 按钮 86 / API 104），
> 审计时以**当次** `go run ./cmd/migrate` 输出的权限统计和路由注册文件为准，
> 不要沿用本文件里写过的历史数字。

比对口径：

- 路由侧：`/admin/departments/:id` 与权限侧 `/admin/departments/:id` 视为一致；method（`GET/POST/PUT/DELETE`）需匹配。
- 方向一：每个受保护路由是否都能在种子里找到对应 `api_path`（找不到 → 非超管访问会被 `PermissionMiddleware` **拒绝 403**，fail-closed）。
- 方向二：每条种子 API 权限是否都对应真实路由（找不到 → 死权限，无人能触发，但无害）。

### 6.2 结论

**52 = 52，双向零不一致**：

- 受保护路由缺少对应权限记录（非超管会误 403）：**0 条**
- 种子 API 权限无对应真实路由（死权限）：**0 条**

当前项目无"路由无权限记录"导致非超管 403 的风险。

### 6.3 维护约定

新增任何受保护接口后，必须在种子里同步追加一条 `Type=3` 权限（同 `Method` + `api_path`），否则非超管调用必 403。建议每次改动 `routes.go` 后复跑本审计（见 `docs/runbook.md` 的排查矩阵）。

---

## 7. 一键复跑（PowerShell 示例）

```powershell
$t = (go run ./cmd/smoketoken | Select-Object -Last 1).Trim()   # 见 2.1
$h = @{ Authorization = "Bearer $t" }
$base = "http://127.0.0.1:8888"
$paths = @(
  "/admin/profile/permissions", "/admin/dashboard/stats",
  "/admin/admin-users?page=1&page_size=10", "/admin/departments/tree",
  "/admin/roles?page=1&page_size=10", "/admin/permissions?page=1&page_size=100",
  "/admin/logs/operations?page=1&page_size=10", "/admin/logs/logins?page=1&page_size=10",
  "/admin/files", "/admin/dicts?page=1&page_size=10",
  "/admin/dicts/items?dict_code=sys_gender&page=1&page_size=10",
  "/admin/configs?page=1&page_size=10"
)
foreach ($p in $paths) {
  $r = Invoke-RestMethod -Uri ($base+$p) -Headers $h
  "$p -> code=$($r.code) msg=$($r.msg)"
}
```
