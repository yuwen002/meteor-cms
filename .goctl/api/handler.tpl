// Code scaffolded by goctl. Safe to edit.
// goctl {{.version}}

package {{.PkgName}}

import (
	"errors"
	"net/http"

	{{if .HasRequest}}{{.ImportPackages}}	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/zeromicro/go-zero/rest/httpx"{{else}}{{.ImportPackages}}
	"github.com/yuwen002/meteor-cms/internal/common"{{end}}
)

{{if .HasDoc}}{{.Doc}}{{end}}
func {{.HandlerName}}(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		{{if .HasRequest}}var req types.{{.RequestType}}
		if err := httpx.Parse(r, &req); err != nil {
			common.Fail(w, common.ErrInvalidParams, common.GetErrorMessage(common.ErrInvalidParams))
			return
		}

		{{end}}l := {{.LogicName}}.New{{.LogicType}}(r.Context(), svcCtx)
		{{if .HasResp}}resp, {{end}}err := l.{{.Call}}({{if .HasRequest}}&req{{end}})
		if err != nil {
			var bizErr *common.BizError
			if errors.As(err, &bizErr) {
				common.Fail(w, bizErr.Code, bizErr.Msg)
			} else {
				common.Fail(w, common.ErrInternalServer, err.Error())
			}
			return
		}
		{{if .HasResp}}common.Ok(w, resp){{else}}common.Ok(w, nil){{end}}
	}
}
