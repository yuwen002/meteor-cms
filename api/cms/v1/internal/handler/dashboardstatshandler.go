package handler

import (
	"net/http"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/logic"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func dashboardStatsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := logic.NewDashboardStatsLogic(r.Context(), svcCtx)
		resp, err := l.DashboardStats()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}
