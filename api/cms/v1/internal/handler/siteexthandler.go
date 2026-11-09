package handler

import (
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/logic"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/svc"
	"github.com/yuwen002/meteor-cms/api/cms/v1/internal/types"
	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/article"
	"github.com/yuwen002/meteor-cms/ent/comment"
	"github.com/yuwen002/meteor-cms/ent/friendlink"
	"github.com/yuwen002/meteor-cms/ent/subscriber"
	"github.com/yuwen002/meteor-cms/internal/common"
	"github.com/zeromicro/go-zero/rest/httpx"
)

const timeLayout = "2006-01-02 15:04:05"

var emailRe = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

func siteConfigBool(r *http.Request, svcCtx *svc.ServiceContext, key string, fallback bool) bool {
	v, ok := svcCtx.Settings.All(r.Context())[key]
	if !ok || strings.TrimSpace(v) == "" {
		return fallback
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return fallback
}

func clientIP(r *http.Request) string {
	if v := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); v != "" {
		return strings.TrimSpace(strings.Split(v, ",")[0])
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

func linkItem(m *ent.FriendLink) types.LinkItem {
	return types.LinkItem{
		ID: m.ID, Name: m.Name, URL: m.URL, Logo: m.Logo, Description: m.Description,
		LinkType: m.LinkType, Sort: m.Sort, Status: m.Status,
		CreatedAt: m.CreatedAt.Format(timeLayout),
	}
}

func commentItem(m *ent.Comment) types.CommentItem {
	return types.CommentItem{
		ID: m.ID, ArticleID: m.ArticleID, ParentID: m.ParentID, Author: m.Author,
		Email: m.Email, Content: m.Content, Status: m.Status, IP: m.IP,
		CreatedAt: m.CreatedAt.Format(timeLayout),
	}
}

func subscriberItem(m *ent.Subscriber) types.SubscriberItem {
	return types.SubscriberItem{
		ID: m.ID, Email: m.Email, Status: m.Status, Source: m.Source,
		CreatedAt: m.CreatedAt.Format(timeLayout),
	}
}

// ---------- 公开接口 ----------

// publicLinksHandler 只返回启用的友链；功能开关关闭时返回空列表。
func publicLinksHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list := []types.LinkItem{}
		if !siteConfigBool(r, svcCtx, "feature_link_enabled", true) {
			common.Ok(w, types.PublicLinkListResp{List: list})
			return
		}
		rows, err := svcCtx.EntClient.FriendLink.Query().
			Where(friendlink.StatusEQ(1)).
			Order(ent.Asc(friendlink.FieldSort), ent.Asc(friendlink.FieldID)).
			Limit(200).All(r.Context())
		if err != nil {
			common.Fail(w, common.ErrInternalServer, "读取友链失败")
			return
		}
		for _, row := range rows {
			list = append(list, linkItem(row))
		}
		common.Ok(w, types.PublicLinkListResp{List: list})
	}
}

// publicCommentsHandler 只返回已发布文章的已通过评论，邮箱/IP 不下发到前台。
func publicCommentsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.SlugPathReq
		if err := httpx.Parse(r, &req); err != nil {
			common.Fail(w, common.ErrInvalidParams, common.GetErrorMessage(common.ErrInvalidParams))
			return
		}
		list := []types.PublicCommentItem{}
		a, err := svcCtx.EntClient.Article.Query().
			Where(article.SlugEQ(req.Slug), article.StatusEQ(logic.ArticleStatusPublished)).
			Only(r.Context())
		if err != nil {
			common.Ok(w, types.PublicCommentListResp{List: list})
			return
		}
		rows, err := svcCtx.EntClient.Comment.Query().
			Where(comment.ArticleIDEQ(a.ID), comment.StatusEQ(1)).
			Order(ent.Asc(comment.FieldID)).
			Limit(500).All(r.Context())
		if err != nil {
			common.Fail(w, common.ErrInternalServer, "读取评论失败")
			return
		}
		for _, row := range rows {
			list = append(list, types.PublicCommentItem{
				ID: row.ID, ParentID: row.ParentID, Author: row.Author,
				Content: row.Content, CreatedAt: row.CreatedAt.Format(timeLayout),
			})
		}
		common.Ok(w, types.PublicCommentListResp{List: list})
	}
}

// publicCommentCreateHandler 受 feature_comment_enabled / feature_comment_review 控制。
func publicCommentCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !siteConfigBool(r, svcCtx, "feature_comment_enabled", false) {
			common.Fail(w, common.ErrInvalidParams, "评论功能未开启")
			return
		}
		var req types.CreateCommentReq
		if err := httpx.Parse(r, &req); err != nil {
			common.Fail(w, common.ErrInvalidParams, common.GetErrorMessage(common.ErrInvalidParams))
			return
		}
		req.Author = strings.TrimSpace(req.Author)
		req.Content = strings.TrimSpace(req.Content)
		req.Email = strings.TrimSpace(req.Email)
		if req.Author == "" || req.Content == "" || req.Slug == "" {
			common.Fail(w, common.ErrInvalidParams, "文章、昵称与评论内容不能为空")
			return
		}
		if len([]rune(req.Author)) > 64 || len([]rune(req.Content)) > 2000 {
			common.Fail(w, common.ErrInvalidParams, "昵称或评论内容过长")
			return
		}
		if req.Email != "" && !emailRe.MatchString(req.Email) {
			common.Fail(w, common.ErrInvalidParams, "邮箱格式不正确")
			return
		}
		a, err := svcCtx.EntClient.Article.Query().
			Where(article.SlugEQ(req.Slug), article.StatusEQ(logic.ArticleStatusPublished)).
			Only(r.Context())
		if err != nil {
			common.Fail(w, common.ErrInvalidParams, "文章不存在或未发布")
			return
		}
		parentID := int64(0)
		if req.ParentID > 0 {
			parent, err := svcCtx.EntClient.Comment.Query().
				Where(comment.IDEQ(req.ParentID), comment.ArticleIDEQ(a.ID), comment.StatusEQ(1)).
				Only(r.Context())
			if err != nil {
				common.Fail(w, common.ErrInvalidParams, "回复的评论不存在或未通过审核")
				return
			}
			parentID = parent.ID
		}
		status := 1
		if siteConfigBool(r, svcCtx, "feature_comment_review", true) {
			status = 0
		}
		created, err := svcCtx.EntClient.Comment.Create().
			SetArticleID(a.ID).SetParentID(parentID).SetAuthor(req.Author).
			SetEmail(req.Email).SetContent(req.Content).SetStatus(status).
			SetIP(clientIP(r)).Save(r.Context())
		if err != nil {
			common.Fail(w, common.ErrInternalServer, "提交评论失败")
			return
		}
		common.Ok(w, types.CreateCommentResp{
			ID: created.ID, Status: status, NeedReview: status == 0,
		})
	}
}

// publicSubscribeHandler 幂等订阅，邮箱格式做基本校验。
func publicSubscribeHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !siteConfigBool(r, svcCtx, "feature_subscription_enabled", true) {
			common.Fail(w, common.ErrInvalidParams, "邮件订阅未开启")
			return
		}
		var req types.SubscribeReq
		if err := httpx.Parse(r, &req); err != nil {
			common.Fail(w, common.ErrInvalidParams, common.GetErrorMessage(common.ErrInvalidParams))
			return
		}
		req.Email = strings.ToLower(strings.TrimSpace(req.Email))
		if !emailRe.MatchString(req.Email) || len(req.Email) > 128 {
			common.Fail(w, common.ErrInvalidParams, "邮箱格式不正确")
			return
		}
		if req.Source == "" {
			req.Source = "site"
		}
		client := svcCtx.EntClient
		existing, err := client.Subscriber.Query().Where(subscriber.EmailEQ(req.Email)).Only(r.Context())
		if err == nil {
			if _, err := client.Subscriber.UpdateOneID(existing.ID).SetStatus(1).Save(r.Context()); err != nil {
				common.Fail(w, common.ErrInternalServer, "更新订阅失败")
				return
			}
		} else if ent.IsNotFound(err) {
			if _, err := client.Subscriber.Create().
				SetEmail(req.Email).SetStatus(1).SetSource(req.Source).
				Save(r.Context()); err != nil {
				common.Fail(w, common.ErrInternalServer, "创建订阅失败")
				return
			}
		} else {
			common.Fail(w, common.ErrInternalServer, "创建订阅失败")
			return
		}
		common.Ok(w, map[string]string{"email": req.Email})
	}
}

// ---------- 后台管理 ----------

func adminLinkListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := svcCtx.EntClient.FriendLink.Query()
		if kw := strings.TrimSpace(r.URL.Query().Get("keyword")); kw != "" {
			q = q.Where(friendlink.Or(friendlink.NameContains(kw), friendlink.URLContains(kw)))
		}
		if s := r.URL.Query().Get("status"); s != "" {
			if v, err := strconv.Atoi(s); err == nil {
				q = q.Where(friendlink.StatusEQ(v))
			}
		}
		rows, err := q.Order(ent.Asc(friendlink.FieldSort), ent.Asc(friendlink.FieldID)).Limit(500).All(r.Context())
		if err != nil {
			common.Fail(w, common.ErrInternalServer, "读取友链失败")
			return
		}
		list := make([]types.LinkItem, 0, len(rows))
		for _, row := range rows {
			list = append(list, linkItem(row))
		}
		common.Ok(w, types.LinkListResp{List: list, Total: len(list)})
	}
}

func adminLinkCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.LinkWriteReq
		if err := httpx.Parse(r, &req); err != nil {
			common.Fail(w, common.ErrInvalidParams, common.GetErrorMessage(common.ErrInvalidParams))
			return
		}
		if strings.TrimSpace(req.Name) == "" || !strings.HasPrefix(req.URL, "http://") && !strings.HasPrefix(req.URL, "https://") {
			common.Fail(w, common.ErrInvalidParams, "名称与 http(s) 链接为必填项")
			return
		}
		status := 1
		if req.Status != nil {
			status = *req.Status
		}
		linkType := req.LinkType
		if linkType == "" {
			linkType = "friend"
		}
		_, err := svcCtx.EntClient.FriendLink.Create().
			SetName(strings.TrimSpace(req.Name)).SetURL(req.URL).SetLogo(req.Logo).
			SetDescription(req.Description).SetLinkType(linkType).
			SetSort(req.Sort).SetStatus(status).Save(r.Context())
		if err != nil {
			common.Fail(w, common.ErrInternalServer, "新增友链失败")
			return
		}
		common.Ok(w, nil)
	}
}

func adminLinkUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.LinkUpdateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.Fail(w, common.ErrInvalidParams, common.GetErrorMessage(common.ErrInvalidParams))
			return
		}
		if strings.TrimSpace(req.Name) == "" || !strings.HasPrefix(req.URL, "http://") && !strings.HasPrefix(req.URL, "https://") {
			common.Fail(w, common.ErrInvalidParams, "名称与 http(s) 链接为必填项")
			return
		}
		upd := svcCtx.EntClient.FriendLink.UpdateOneID(req.ID).
			SetName(strings.TrimSpace(req.Name)).SetURL(req.URL).SetLogo(req.Logo).
			SetDescription(req.Description).SetSort(req.Sort)
		if req.LinkType != "" {
			upd = upd.SetLinkType(req.LinkType)
		}
		if req.Status != nil {
			upd = upd.SetStatus(*req.Status)
		}
		if _, err := upd.Save(r.Context()); err != nil {
			common.Fail(w, common.ErrInternalServer, "更新友链失败")
			return
		}
		common.Ok(w, nil)
	}
}

func adminLinkDeleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var path types.IDPathReq
		if err := httpx.Parse(r, &path); err != nil {
			common.Fail(w, common.ErrInvalidParams, common.GetErrorMessage(common.ErrInvalidParams))
			return
		}
		if err := svcCtx.EntClient.FriendLink.DeleteOneID(path.ID).Exec(r.Context()); err != nil {
			common.Fail(w, common.ErrInternalServer, "删除友链失败")
			return
		}
		common.Ok(w, nil)
	}
}

func adminCommentListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := svcCtx.EntClient.Comment.Query()
		if s := r.URL.Query().Get("status"); s != "" {
			if v, err := strconv.Atoi(s); err == nil {
				q = q.Where(comment.StatusEQ(v))
			}
		}
		if kw := strings.TrimSpace(r.URL.Query().Get("keyword")); kw != "" {
			q = q.Where(comment.Or(comment.AuthorContains(kw), comment.ContentContains(kw)))
		}
		rows, err := q.Order(ent.Desc(comment.FieldID)).Limit(500).All(r.Context())
		if err != nil {
			common.Fail(w, common.ErrInternalServer, "读取评论失败")
			return
		}
		list := make([]types.CommentItem, 0, len(rows))
		for _, row := range rows {
			list = append(list, commentItem(row))
		}
		common.Ok(w, types.CommentListResp{List: list, Total: len(list)})
	}
}

func adminCommentStatusHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CommentStatusReq
		if err := httpx.Parse(r, &req); err != nil {
			common.Fail(w, common.ErrInvalidParams, common.GetErrorMessage(common.ErrInvalidParams))
			return
		}
		if req.Status < 0 || req.Status > 2 {
			common.Fail(w, common.ErrInvalidParams, "评论状态非法")
			return
		}
		if _, err := svcCtx.EntClient.Comment.UpdateOneID(req.ID).SetStatus(req.Status).Save(r.Context()); err != nil {
			common.Fail(w, common.ErrInternalServer, "更新评论状态失败")
			return
		}
		common.Ok(w, nil)
	}
}

func adminCommentDeleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var path types.IDPathReq
		if err := httpx.Parse(r, &path); err != nil {
			common.Fail(w, common.ErrInvalidParams, common.GetErrorMessage(common.ErrInvalidParams))
			return
		}
		if err := svcCtx.EntClient.Comment.DeleteOneID(path.ID).Exec(r.Context()); err != nil {
			common.Fail(w, common.ErrInternalServer, "删除评论失败")
			return
		}
		common.Ok(w, nil)
	}
}

func adminSubscriberListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := svcCtx.EntClient.Subscriber.Query()
		if kw := strings.TrimSpace(r.URL.Query().Get("keyword")); kw != "" {
			q = q.Where(subscriber.EmailContains(kw))
		}
		rows, err := q.Order(ent.Desc(subscriber.FieldID)).Limit(500).All(r.Context())
		if err != nil {
			common.Fail(w, common.ErrInternalServer, "读取订阅失败")
			return
		}
		list := make([]types.SubscriberItem, 0, len(rows))
		for _, row := range rows {
			list = append(list, subscriberItem(row))
		}
		common.Ok(w, types.SubscriberListResp{List: list, Total: len(list)})
	}
}

func adminSubscriberDeleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var path types.IDPathReq
		if err := httpx.Parse(r, &path); err != nil {
			common.Fail(w, common.ErrInvalidParams, common.GetErrorMessage(common.ErrInvalidParams))
			return
		}
		if err := svcCtx.EntClient.Subscriber.DeleteOneID(path.ID).Exec(r.Context()); err != nil {
			common.Fail(w, common.ErrInternalServer, "删除订阅失败")
			return
		}
		common.Ok(w, nil)
	}
}
