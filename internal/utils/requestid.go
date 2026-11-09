package utils

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
)

// RequestIDHeader 请求链路标识的 HTTP 头。客户端可传入以便与上游网关串联，
// 服务端会原样回写到响应头，便于排障时按 ID 检索全部日志。
const RequestIDHeader = "X-Request-ID"

const requestIDKey reqCtxKey = "request_id"

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func RequestIDFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

// NewRequestID 生成 16 字节随机标识（32 位十六进制），不依赖外部服务。
func NewRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	return hex.EncodeToString(b[:])
}

// ResolveRequestID 优先复用客户端传入的链路标识，否则生成一个。
func ResolveRequestID(r *http.Request) string {
	if v := strings.TrimSpace(r.Header.Get(RequestIDHeader)); v != "" {
		return Truncate(v, 64)
	}
	return NewRequestID()
}
