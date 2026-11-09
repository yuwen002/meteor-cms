// Code scaffolded manually. Safe to edit.
// 站点扩展模块（友链 / 评论 / 订阅）的请求与响应类型。

package types

// ---------- 友链 ----------

type LinkItem struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	Logo        string `json:"logo"`
	Description string `json:"description"`
	LinkType    string `json:"link_type"`
	Sort        int    `json:"sort"`
	Status      int    `json:"status"`
	CreatedAt   string `json:"created_at"`
}

type LinkWriteReq struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	Logo        string `json:"logo,optional"`
	Description string `json:"description,optional"`
	LinkType    string `json:"link_type,optional"`
	Sort        int    `json:"sort,optional"`
	Status      *int   `json:"status,optional"`
}

// LinkUpdateReq 同时承载路径参数与 JSON 请求体，避免对同一请求重复 Parse 消耗 body。
type LinkUpdateReq struct {
	ID          int64  `path:"id"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	Logo        string `json:"logo,optional"`
	Description string `json:"description,optional"`
	LinkType    string `json:"link_type,optional"`
	Sort        int    `json:"sort,optional"`
	Status      *int   `json:"status,optional"`
}

type LinkListResp struct {
	List  []LinkItem `json:"list"`
	Total int        `json:"total"`
}

// ---------- 评论 ----------

type CommentItem struct {
	ID        int64  `json:"id"`
	ArticleID int64  `json:"article_id"`
	ParentID  int64  `json:"parent_id"`
	Author    string `json:"author"`
	Email     string `json:"email"`
	Content   string `json:"content"`
	Status    int    `json:"status"`
	IP        string `json:"ip"`
	CreatedAt string `json:"created_at"`
}

type PublicCommentItem struct {
	ID        int64  `json:"id"`
	ParentID  int64  `json:"parent_id"`
	Author    string `json:"author"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

type PublicLinkListResp struct {
	List []LinkItem `json:"list"`
}

type PublicCommentListResp struct {
	List []PublicCommentItem `json:"list"`
}

type CreateCommentReq struct {
	Slug     string `json:"slug"`
	ParentID int64  `json:"parent_id,optional"`
	Author   string `json:"author"`
	Email    string `json:"email,optional"`
	Content  string `json:"content"`
}

type CreateCommentResp struct {
	ID         int64 `json:"id"`
	Status     int   `json:"status"`
	NeedReview bool  `json:"need_review"`
}

type CommentListResp struct {
	List  []CommentItem `json:"list"`
	Total int           `json:"total"`
}

// ---------- 订阅 ----------

type SubscriberItem struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	Status    int    `json:"status"`
	Source    string `json:"source"`
	CreatedAt string `json:"created_at"`
}

type SubscribeReq struct {
	Email  string `json:"email"`
	Source string `json:"source,optional"`
}

type SubscriberListResp struct {
	List  []SubscriberItem `json:"list"`
	Total int              `json:"total"`
}

// ---------- 通用 ----------

type IDPathReq struct {
	ID int64 `path:"id"`
}

type SlugPathReq struct {
	Slug string `path:"slug"`
}

type CommentStatusReq struct {
	ID     int64 `path:"id"`
	Status int   `json:"status"`
}
