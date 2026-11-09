package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/config"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/tokenblacklist"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/yuwen002/meteor-cms/internal/pkg/tokenversion"
	"github.com/yuwen002/meteor-cms/internal/utils"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
)

type JwtMiddleware struct {
	config *config.Config
	db     *ent.Client
	rdb    *redis.Redis
}

func NewJwtMiddleware(c *config.Config, db *ent.Client, rdb *redis.Redis) rest.Middleware {
	m := &JwtMiddleware{
		config: c,
		db:     db,
		rdb:    rdb,
	}
	return m.Handle
}

func (m *JwtMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			common.Fail(w, common.ErrUnauthorized, common.GetErrorMessage(common.ErrUnauthorized))
			return
		}

		// Bearer xxx
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			common.Fail(w, common.ErrInvalidToken, common.GetErrorMessage(common.ErrInvalidToken))
			return
		}

		tokenStr := parts[1]

		// 生成token的哈希值
		tokenHash := utils.GenerateTokenHash(tokenStr)

		// 检查token是否在黑名单中（使用token_hash进行查询）
		exists, err := m.db.TokenBlacklist.
			Query().
			Where(tokenblacklist.TokenHash(tokenHash)).
			Exist(r.Context())

		if err != nil {
			logx.Errorf("查询token黑名单失败: %v", err)
			common.Fail(w, common.ErrInternalServer, common.GetErrorMessage(common.ErrInternalServer))
			return
		}
		if exists {
			common.Fail(w, common.ErrInvalidToken, common.GetErrorMessage(common.ErrInvalidToken))
			return
		}

		// 解析 token（使用你自己的 utils.ParseToken）
		claims, err := utils.ParseToken(m.config.Auth.AccessSecret, tokenStr)
		if err != nil {
			// 过期令牌返回 2001，前端据此发起无感刷新；其它非法令牌仍返回 2009
			if errors.Is(err, utils.ErrExpiredToken) {
				common.Fail(w, common.ErrUnauthorized, common.GetErrorMessage(common.ErrUnauthorized))
				return
			}
			common.Fail(w, common.ErrInvalidToken, common.GetErrorMessage(common.ErrInvalidToken))
			return
		}

		// token 版本校验：用户被踢下线 / 修改密码后，Redis 中版本号自增，
		// 携带旧版本号的令牌立即失效（全局踢人 / 批量吊销会话）。
		// 缺失 tv 的旧令牌视为版本 0；一旦版本号 >0（发生过踢人/改密），旧令牌即被吊销。
		if m.rdb != nil {
			tvClaim := int64(0)
			if v, ok := claims["tv"].(float64); ok {
				tvClaim = int64(v)
			}
			if userID, ok := utils.ExtractUserID(claims); ok && userID > 0 {
				cur, gerr := tokenversion.Get(r.Context(), m.rdb, userID)
				if gerr != nil {
					// Redis 异常降级放行，避免误杀（与限流降级一致）
					logx.Errorf("token version check failed: user_id=%d err=%v", userID, gerr)
				} else if tvClaim != cur {
					common.Fail(w, common.ErrTokenRevoked, common.GetErrorMessage(common.ErrTokenRevoked))
					return
				}
			}
		}

		// 把用户信息和token塞进 context，后面所有 logic 都能直接拿
		ctx := utils.WithUserCtx(r.Context(), claims)
		// 存储token到context
		ctx = utils.WithTokenCtx(ctx, tokenStr)
		r = r.WithContext(ctx)

		// 放行
		next(w, r)
	}
}
