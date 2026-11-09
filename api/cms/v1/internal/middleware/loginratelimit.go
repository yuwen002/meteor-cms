package middleware

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
)

// loginRateLimitCode 与统一响应约定一致的业务码（请求过于频繁）
const loginRateLimitCode = 2029

// windowLimitLua 固定窗口限流：对 key 自增，首次设置窗口过期；超过阈值返回 blocked=1 与剩余 TTL。
// 采用 Lua 保证「自增 + 设置过期」的原子性，避免并发窗口边界出现计数漂移。
var windowLimitLua = `
local current = redis.call("incr", KEYS[1])
if tonumber(current) == 1 then
  redis.call("expire", KEYS[1], tonumber(ARGV[1]))
end
if tonumber(current) > tonumber(ARGV[2]) then
  return {1, redis.call("ttl", KEYS[1])}
end
return {0, redis.call("ttl", KEYS[1])}
`

type ipCounter struct {
	mu           sync.Mutex
	count        int
	windowStart  int64
	blockedUntil int64
}

// ipRateLimiter 登录限流与公开接口限流共用的固定窗口实现，prefix 用于隔离 Redis 计数键。
type ipRateLimiter struct {
	rds          *redis.Redis
	prefix       string
	max          int
	window       int
	blockSeconds int
	mem          *sync.Map // rds==nil 或 Redis 异常时的兜底
}

// allow 返回是否限流拦截，以及建议客户端重试等待秒数（Redis 路径返回 key 剩余 TTL）。
func (l *ipRateLimiter) allow(ip string) (bool, int) {
	if l.rds != nil {
		res, err := l.rds.Eval(windowLimitLua, []string{l.prefix + ip}, l.window, l.max)
		if err == nil {
			if arr, ok := res.([]interface{}); ok && len(arr) == 2 {
				if toInt(arr[0]) == 1 {
					return true, toInt(arr[1])
				}
				return false, toInt(arr[1])
			}
		}
		// Redis 不可用：降级到内存限流，保证防护不失效
		logx.Errorf("Redis 限流失败（%s），降级为内存模式: %v", l.prefix, err)
	}
	return l.memAllow(ip)
}

// memAllow 单实例内存限流，作为 Redis 不可用时的兜底。
func (l *ipRateLimiter) memAllow(ip string) (bool, int) {
	now := time.Now().Unix()
	val, _ := l.mem.LoadOrStore(ip, &ipCounter{})
	c := val.(*ipCounter)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.blockedUntil > now {
		return true, int(c.blockedUntil - now)
	}
	if now-c.windowStart > int64(l.window) {
		c.count = 0
		c.windowStart = now
	}
	c.count++
	if c.count > l.max {
		c.blockedUntil = now + int64(l.blockSeconds)
		return true, l.blockSeconds
	}
	return false, 0
}

// NewLoginRateLimitMiddleware 按客户端 IP 对登录接口做分布式固定窗口限流（防爆破）。
// rds 为 nil 或 Redis 不可用时自动降级为单实例内存限流；未启用（enabled=false）时直接透传，零额外开销。
func NewLoginRateLimitMiddleware(rds *redis.Redis, enabled bool, max, window, blockSeconds int, onBlocked func(*http.Request)) rest.Middleware {
	if !enabled {
		return func(next http.HandlerFunc) http.HandlerFunc {
			return next
		}
	}
	limiter := &ipRateLimiter{
		rds: rds, prefix: "login_rl:", max: max, window: window,
		blockSeconds: blockSeconds, mem: &sync.Map{},
	}
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/admin/login" || r.Method != http.MethodPost {
				next(w, r)
				return
			}
			blocked, retryAfter := limiter.allow(clientIP(r))
			if blocked {
				if onBlocked != nil {
					onBlocked(r)
				}
				writeRateLimited(w, retryAfter)
				return
			}
			next(w, r)
		}
	}
}

func writeRateLimited(w http.ResponseWriter, retryAfter int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if retryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"code": loginRateLimitCode,
		"msg":  "请求过于频繁，请稍后再试",
		"data": nil,
	})
}

// clientIP 从 X-Forwarded-For 取首个 IP，否则取 RemoteAddr（去除端口）
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.IndexByte(xff, ','); idx >= 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func toInt(v interface{}) int {
	switch n := v.(type) {
	case int64:
		return int(n)
	case int:
		return n
	case float64:
		return int(n)
	case string:
		i, _ := strconv.Atoi(n)
		return i
	default:
		return 0
	}
}
