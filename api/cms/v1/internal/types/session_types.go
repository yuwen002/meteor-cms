package types

// SessionItem 是活跃登录会话的对外视图；不返回 access_hash 等内部字段。
type SessionItem struct {
	SID        string `json:"sid"`
	IP         string `json:"ip"`
	UserAgent  string `json:"user_agent"`
	CreatedAt  int64  `json:"created_at"`
	LastSeenAt int64  `json:"last_seen_at"`
	ExpiresAt  int64  `json:"expires_at"`
	IsCurrent  bool   `json:"is_current"`
}

type SessionListResp struct {
	List []SessionItem `json:"list"`
}

type SessionSIDReq struct {
	SID string `path:"sid"`
}
