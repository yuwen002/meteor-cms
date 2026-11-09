package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/config"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/internal/pkg/audit"
	"github.com/yuwen002/meteor-cms/internal/utils"
	"github.com/zeromicro/go-zero/rest"
)

const (
	// maxLogBodyBytes 参数快照的最大长度，超出部分截断（只影响日志，不影响业务请求体）
	maxLogBodyBytes = 4096
	// writeLogTimeout 写日志的超时时间，避免拖慢请求
	writeLogTimeout = 3 * time.Second
)

// sensitiveKeys 需要脱敏的参数名。比较时忽略大小写并去掉 _ - 与空格，
// 因此 password / PassWord / accessToken / access_token 都会被命中。
var sensitiveKeys = []string{
	"password", "passwd", "pwd", "old_password", "new_password", "confirm_password",
	"captcha", "captcha_id", "token", "access_token", "refresh_token",
	"authorization", "secret", "client_secret", "api_key", "private_key", "credential",
	"phone", "mobile", "id_card", "idcard",
	// 配置值是可能含密钥的自由文本，审计快照一律脱敏；
	// 精确的值级追溯由 sys_config_versions 承担
	"config_value",
}

var sensitiveKeySet = func() map[string]struct{} {
	m := make(map[string]struct{}, len(sensitiveKeys))
	for _, k := range sensitiveKeys {
		m[normalizeKey(k)] = struct{}{}
	}
	return m
}()

func normalizeKey(key string) string {
	return strings.NewReplacer("_", "", "-", "", " ", "").Replace(strings.ToLower(key))
}

func isSensitiveKey(key string) bool {
	_, ok := sensitiveKeySet[normalizeKey(key)]
	return ok
}

// moduleByPrefix 请求路径前缀到业务模块的映射，用于日志归类
var moduleByPrefix = map[string]string{
	"/admin/admin-users": "管理员管理",
	"/admin/departments": "部门管理",
	"/admin/roles":       "角色管理",
	"/admin/permissions": "权限管理",
	"/admin/profile":     "个人中心",
	"/admin/logs":        "日志管理",
	"/admin/articles":    "文章管理",
	"/admin/categories":  "分类管理",
	"/admin/tags":        "标签管理",
	"/admin/trash":       "回收站",
	"/admin/files":       "文件管理",
	"/admin/upload":      "文件管理",
	"/admin/dicts":       "字典管理",
	"/admin/configs":     "配置管理",
	"/admin/dashboard":   "控制台",
}

// actionKeywords 路径关键词到操作描述的映射，按顺序命中，越具体的越靠前。
// 后台路由没有统一的操作元数据，这里用路径语义 + HTTP 方法推断可读的动作名。
var actionKeywords = []struct {
	keyword string
	action  string
}{
	{"force-logout-by-dept", "按部门强制下线"},
	{"force-logout-all", "强制全部下线"},
	{"force-logout", "强制下线"},
	{"logout-all", "强制全部下线"},
	{"forgot-password", "找回密码"},
	{"change-password", "修改密码"},
	{"reset-password", "重置密码"},
	{"bind-department", "绑定部门"},
	{"toggle-top", "置顶切换"},
	{"withdraw", "撤回"},
	{"submit", "提交审核"},
	{"batch-delete", "批量删除"},
	{"batch-offline", "批量下线"},
	{"enable", "启用"},
	{"disable", "停用"},
	{"purge", "彻底删除"},
	{"restore", "恢复"},
	{"approve", "审核通过"},
	{"reject", "审核驳回"},
	{"publish", "发布"},
	{"offline", "下线"},
	{"assign", "分配"},
	{"upload", "上传"},
	{"export", "导出"},
	{"import", "导入"},
	{"create", "新增"},
	{"delete", "删除"},
	{"remove", "移除"},
	{"update", "修改"},
	{"rename", "重命名"},
	{"login", "登录"},
	{"logout", "退出登录"},
}

// OperationLogMiddleware 操作日志中间件
//
// 记录受保护接口的调用情况：操作人、链路标识、方法、路径、参数快照、结果码与耗时。
// 日志写入放在独立 goroutine 中，不阻塞请求；二进制上传请求不记录参数。
type OperationLogMiddleware struct {
	config config.Config
	db     *ent.Client
}

// NewOperationLogMiddleware 创建操作日志中间件
func NewOperationLogMiddleware(c config.Config, db *ent.Client) rest.Middleware {
	m := &OperationLogMiddleware{config: c, db: db}
	return m.Handle
}

// shouldSkipAudit 判断哪些请求不应产生操作日志。
// 只跳过两个列表查询与验证码获取：前者每翻一页都会新增记录、淹没真实操作，后者高频且无审计价值。
// 精确匹配而非 /admin/logs/ 前缀，后续新增的详情/导出/统计接口仍会被正常采集。
func shouldSkipAudit(method, path string) bool {
	if method == http.MethodGet && (path == "/admin/logs/operations" || path == "/admin/logs/logins") {
		return true
	}
	return path == "/api/captcha"
}

// Handle 返回中间件处理函数
func (m *OperationLogMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if shouldSkipAudit(r.Method, r.URL.Path) {
			next(w, r)
			return
		}

		start := time.Now()
		body := readBodySnapshot(r)
		query := readQuerySnapshot(r)

		rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK, body: bytes.NewBuffer(nil)}
		next(rec, r)

		duration := time.Since(start)
		audit.Submit("operation_log", func(ctx context.Context) error {
			return m.save(ctx, r, rec, body, query, duration)
		})
	}
}

// save 写入单条操作日志，由有界队列驱动（ctx 已带超时）
func (m *OperationLogMiddleware) save(ctx context.Context, r *http.Request, rec *responseRecorder, body, query string, duration time.Duration) error {
	var userID int64
	var username string
	if id, ok := utils.CurrentUserID(r.Context()); ok {
		userID = id
	}
	if claims := utils.GetUserFromCtx(r.Context()); claims != nil {
		if name, ok := claims["username"].(string); ok {
			username = name
		}
	}
	// 认证类接口没有 JWT 身份，从脱敏后的参数快照里补账号名，保证日志可定位到人
	if username == "" && body != "" {
		username = usernameFromSnapshot(body)
	}

	requestID := utils.RequestIDFromCtx(r.Context())
	if requestID == "" {
		requestID = utils.NewRequestID()
	}
	code, msg := rec.businessResult()

	create := m.db.OperationLog.Create().
		SetUserID(userID).
		SetUsername(username).
		SetModule(resolveModule(r.URL.Path)).
		SetAction(resolveAction(r.Method, r.URL.Path)).
		SetMethod(r.Method).
		SetPath(r.URL.Path).
		SetIP(utils.ClientIPFromRequest(r)).
		SetUserAgent(utils.Truncate(r.UserAgent(), 512)).
		SetDurationMs(duration.Milliseconds()).
		SetBizCode(code).
		SetHTTPStatus(rec.status)

	if requestID != "" {
		create = create.SetRequestID(requestID)
	}
	if body != "" {
		create = create.SetParams(body)
	}
	if query != "" {
		create = create.SetQuery(query)
	}

	if code == 0 {
		create = create.SetStatus(0)
	} else {
		create = create.SetStatus(1).SetErrorMsg(utils.Truncate(msg, 512))
	}

	_, err := create.Save(ctx)
	return err
}

// readBodySnapshot 读取请求体快照。
//
// 只读取有 body 且非二进制的请求；必须读取完整 body 再原样回填，
// 否则超过快照上限的请求会把截断后的非法 JSON 交给下游 handler。
func readBodySnapshot(r *http.Request) string {
	if r.Body == nil || r.Method == http.MethodGet || r.Method == http.MethodHead {
		return ""
	}

	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/") {
		// 文件上传等二进制请求不记录内容，避免日志膨胀
		return ""
	}

	raw, err := io.ReadAll(r.Body)
	if err != nil {
		return ""
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))

	snapshot := strings.TrimSpace(string(raw))
	if snapshot == "" {
		return ""
	}
	return sanitizeParams(contentType, snapshot)
}

// readQuerySnapshot 采集 URL query，让 GET 请求的筛选条件也可回溯。
func readQuerySnapshot(r *http.Request) string {
	if r.URL.RawQuery == "" {
		return ""
	}
	return sanitizeQuery(r.URL.RawQuery)
}

// usernameFromSnapshot 从脱敏后的参数快照里提取操作人账号。
// 认证类接口没有 JWT claims，用它给审计日志补一个可定位的账号名。
func usernameFromSnapshot(snapshot string) string {
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(snapshot), &payload); err != nil {
		return ""
	}
	for _, key := range []string{"username", "email"} {
		if v, ok := payload[key].(string); ok && v != "" {
			return utils.Truncate(v, 50)
		}
	}
	return ""
}

// sanitizeParams 对 JSON 或表单请求体做递归脱敏后生成快照。
// 其他 Content-Type 无法可靠逐字段识别，直接丢弃以免泄露敏感值。
func sanitizeParams(contentType, snapshot string) string {
	switch {
	case strings.Contains(contentType, "application/json"):
		var payload interface{}
		if err := json.Unmarshal([]byte(snapshot), &payload); err != nil {
			return ""
		}
		masked, err := json.Marshal(maskSensitive(payload))
		if err != nil {
			return ""
		}
		return utils.Truncate(string(masked), maxLogBodyBytes)
	case strings.Contains(contentType, "application/x-www-form-urlencoded"):
		return sanitizeQuery(snapshot)
	default:
		return ""
	}
}

// sanitizeQuery 解析 query/表单并脱敏敏感键，保留其余筛选条件用于回溯。
func sanitizeQuery(raw string) string {
	values, err := url.ParseQuery(raw)
	if err != nil {
		return ""
	}
	for key, vals := range values {
		for i := range vals {
			if isSensitiveKey(key) {
				vals[i] = "******"
			} else {
				vals[i] = utils.Truncate(vals[i], 512)
			}
		}
		values[key] = vals
	}
	return utils.Truncate(values.Encode(), maxLogBodyBytes)
}

// maskSensitive 递归遍历 JSON 的 map / slice，命中敏感键则整值替换。
func maskSensitive(value interface{}) interface{} {
	switch node := value.(type) {
	case map[string]interface{}:
		for key, child := range node {
			if isSensitiveKey(key) {
				node[key] = "******"
			} else {
				node[key] = maskSensitive(child)
			}
		}
		return node
	case []interface{}:
		for i := range node {
			node[i] = maskSensitive(node[i])
		}
		return node
	default:
		return value
	}
}

// resolveModule 按路径前缀归类业务模块
func resolveModule(path string) string {
	for prefix, module := range moduleByPrefix {
		if strings.HasPrefix(path, prefix) {
			return module
		}
	}
	return "其他"
}

// resolveAction 按路径关键词与 HTTP 方法推断操作描述，写入 operation_logs.action。
// 路径不带语义时按方法兜底，保证「操作」列始终可读。
func resolveAction(method, path string) string {
	lower := strings.ToLower(path)
	for _, item := range actionKeywords {
		if strings.Contains(lower, item.keyword) {
			return item.action
		}
	}

	switch method {
	case http.MethodPost:
		return "新增"
	case http.MethodPut, http.MethodPatch:
		return "修改"
	case http.MethodDelete:
		return "删除"
	default:
		return "查询"
	}
}

// responseRecorder 捕获响应状态码与响应体，用于判定操作结果
type responseRecorder struct {
	http.ResponseWriter
	status int
	body   *bytes.Buffer
}

// WriteHeader 记录状态码
func (r *responseRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Write 同时写入底层响应与缓冲
func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

// businessResult 解析响应体中的业务状态码与提示信息
func (r *responseRecorder) businessResult() (int, string) {
	var out struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(r.body.Bytes(), &out); err != nil {
		// 非 JSON 响应（如静态资源）按 HTTP 状态码判定
		if r.status >= http.StatusBadRequest {
			return r.status, http.StatusText(r.status)
		}
		return 0, ""
	}
	return out.Code, out.Msg
}
