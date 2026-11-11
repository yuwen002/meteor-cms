package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent/tokenblacklist"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/yuwen002/meteor-cms/internal/pkg/authsession"
	"github.com/yuwen002/meteor-cms/internal/utils"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// RegisterSessionRoutes 注册活跃会话接口，挂在与其它后台接口一致的受保护链上。
func RegisterSessionRoutes(server *rest.Server, svcCtx *svc.ServiceContext) {
	protected := []rest.Middleware{
		svcCtx.ClientInfoMiddleware,
		svcCtx.JwtMiddleware,
		svcCtx.OperationLogMiddleware,
		svcCtx.PermissionMiddleware,
	}
	server.AddRoutes(rest.WithMiddlewares(protected, []rest.Route{
		{Method: http.MethodGet, Path: "/admin/sessions", Handler: sessionListHandler(svcCtx)},
		{Method: http.MethodDelete, Path: "/admin/sessions/:sid", Handler: sessionRevokeHandler(svcCtx)},
	}...))
}

func sessionStore(svcCtx *svc.ServiceContext) *authsession.Store {
	return authsession.New(svcCtx.Redis, time.Duration(svcCtx.Config.Auth.RefreshExpire)*time.Second)
}

func sessionListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := utils.GetUserFromCtx(r.Context())
		uid, ok := utils.ExtractUserID(claims)
		if !ok || uid <= 0 {
			common.Fail(w, common.ErrUnauthorized, common.GetErrorMessage(common.ErrUnauthorized))
			return
		}
		currentSID, _ := claims["sid"].(string)
		sessions, err := sessionStore(svcCtx).List(r.Context(), uid)
		if err != nil {
			common.Fail(w, common.ErrInternalServer, "会话列表暂不可用，请检查 Redis 配置")
			return
		}
		list := make([]types.SessionItem, 0, len(sessions))
		for _, s := range sessions {
			list = append(list, types.SessionItem{
				SID: s.SID, IP: s.IP, UserAgent: s.UserAgent,
				CreatedAt: s.CreatedAt, LastSeenAt: s.LastSeenAt, ExpiresAt: s.ExpiresAt,
				IsCurrent: s.SID == currentSID,
			})
		}
		common.Ok(w, types.SessionListResp{List: list})
	}
}

func sessionRevokeHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.SessionSIDReq
		if err := httpx.Parse(r, &req); err != nil || strings.TrimSpace(req.SID) == "" {
			common.Fail(w, common.ErrInvalidParams, "缺少会话 ID")
			return
		}
		claims := utils.GetUserFromCtx(r.Context())
		uid, ok := utils.ExtractUserID(claims)
		if !ok || uid <= 0 {
			common.Fail(w, common.ErrUnauthorized, common.GetErrorMessage(common.ErrUnauthorized))
			return
		}

		sess, err := sessionStore(svcCtx).Revoke(r.Context(), uid, strings.TrimSpace(req.SID))
		if errors.Is(err, authsession.ErrNotFound) {
			common.Fail(w, common.ErrInvalidParams, "会话不存在或已过期")
			return
		}
		if err != nil {
			common.Fail(w, common.ErrInternalServer, "吊销失败，请检查 Redis 配置")
			return
		}

		// revoked 标记由中间件按 sid 拦截；再把该会话当前 access token 写入黑名单，
		// 让已经签发的令牌在黑名单查询处直接失效，形成双重保险。
		if sess.AccessHash != "" {
			exists, e := svcCtx.EntClient.TokenBlacklist.Query().
				Where(tokenblacklist.TokenHashEQ(sess.AccessHash)).
				Exist(r.Context())
			if e == nil && !exists {
				if _, e2 := svcCtx.EntClient.TokenBlacklist.Create().
					SetTokenHash(sess.AccessHash).
					SetExpiredAt(time.Unix(sess.ExpiresAt, 0)).
					Save(r.Context()); e2 != nil {
					logx.Errorf("吊销会话时写入 token 黑名单失败: sid=%s err=%v", sess.SID, e2)
				}
			}
		}
		common.Ok(w, types.CommonResp{Message: "Session revoked"})
	}
}
