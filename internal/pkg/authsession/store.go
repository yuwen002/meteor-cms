// Package authsession 存放活跃登录会话，用于「查看登录设备 / 吊销单个会话」。
//
// 会话是易失的运行时状态，与 tokenversion 的 auth:tv 一样放在 Redis：
// 不新增表、不改变 schema 摘要，也不影响漏跑迁移时的启动校验。
// 未配置 Redis 时按项目既有语义降级：登记与校验 no-op，列表/吊销返回明确错误。
package authsession

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

var (
	// ErrUnavailable 表示会话存储不可用（未配置 Redis 或 Redis 访问失败）。
	ErrUnavailable = errors.New("会话存储不可用，请检查 Redis 配置")
	// ErrNotFound 表示会话不存在、已过期或不属于当前用户。
	ErrNotFound = errors.New("会话不存在或已过期")
	// ErrRevoked 表示会话已被单独吊销，不能再据此轮换令牌。
	ErrRevoked = errors.New("会话已吊销")
)

// Session 是一条活跃登录会话的快照。
type Session struct {
	SID        string `json:"sid"`
	UserID     int64  `json:"user_id"`
	IP         string `json:"ip"`
	UserAgent  string `json:"user_agent"`
	CreatedAt  int64  `json:"created_at"`
	LastSeenAt int64  `json:"last_seen_at"`
	ExpiresAt  int64  `json:"expires_at"`
	// AccessHash 是该会话当前 access token 的 SHA256 哈希；吊销时据此把令牌加入黑名单，
	// 让 JWT 中间件现有的黑名单查询直接生效，无需额外查库。
	AccessHash string `json:"access_hash"`
}

type Store struct {
	rdb *redis.Redis
	ttl time.Duration
}

// New 创建会话存储；ttl 应取刷新令牌有效期。
func New(rdb *redis.Redis, ttl time.Duration) *Store {
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	return &Store{rdb: rdb, ttl: ttl}
}

func NewSID() string { return uuid.NewString() }

func sessionKey(uid int64, sid string) string {
	return fmt.Sprintf("auth:session:%d:%s", uid, sid)
}

func indexKey(uid int64) string {
	return fmt.Sprintf("auth:sessions:%d", uid)
}

func revokedKey(sid string) string {
	return "auth:session:revoked:" + sid
}

func (s *Store) seconds() int { return int(s.ttl.Seconds()) }

// Create 登记新会话。未配置 Redis 时静默降级，不影响登录主流程。
func (s *Store) Create(ctx context.Context, uid int64, sid, ip, ua, accessHash string) error {
	if s.rdb == nil {
		return nil
	}
	now := time.Now()
	sess := Session{
		SID:        sid,
		UserID:     uid,
		IP:         ip,
		UserAgent:  ua,
		CreatedAt:  now.Unix(),
		LastSeenAt: now.Unix(),
		ExpiresAt:  now.Add(s.ttl).Unix(),
		AccessHash: accessHash,
	}
	if err := s.putSession(sess); err != nil {
		return err
	}
	return s.appendIndex(uid, sid)
}

// Touch 在令牌轮换后更新会话的当前 access hash 与 last_seen。
func (s *Store) Touch(ctx context.Context, uid int64, sid, accessHash string) error {
	if s.rdb == nil {
		return nil
	}
	sess, err := s.getSession(uid, sid)
	if err != nil || sess == nil {
		return err
	}
	sess.AccessHash = accessHash
	sess.LastSeenAt = time.Now().Unix()
	return s.putSession(*sess)
}

// List 返回当前用户仍有效、未被吊销的会话。
func (s *Store) List(ctx context.Context, uid int64) ([]Session, error) {
	if s.rdb == nil {
		return nil, ErrUnavailable
	}
	sids, err := s.readIndex(uid)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	out := make([]Session, 0, len(sids))
	for _, sid := range sids {
		sess, e := s.getSession(uid, sid)
		if e != nil || sess == nil || sess.ExpiresAt <= now {
			continue
		}
		revoked, e := IsRevoked(s.rdb, sid)
		if e != nil || revoked {
			continue
		}
		out = append(out, *sess)
	}
	return out, nil
}

// Revoke 标记会话已吊销并返回快照，供调用方把当前 access token 写入黑名单。
func (s *Store) Revoke(ctx context.Context, uid int64, sid string) (*Session, error) {
	if s.rdb == nil {
		return nil, ErrUnavailable
	}
	sess, err := s.getSession(uid, sid)
	if err != nil {
		return nil, err
	}
	if sess == nil || sess.UserID != uid {
		return nil, ErrNotFound
	}
	if err := s.rdb.Setex(revokedKey(sid), "1", s.seconds()); err != nil {
		return nil, err
	}
	// 从用户索引移除；失败不影响吊销标记已生效。
	sids, _ := s.readIndex(uid)
	kept := make([]string, 0, len(sids))
	for _, v := range sids {
		if v != sid {
			kept = append(kept, v)
		}
	}
	_ = s.writeIndex(uid, kept)
	return sess, nil
}

// IsRevoked 供 JWT 中间件按 sid 判断会话是否已被单独吊销。
func IsRevoked(rdb *redis.Redis, sid string) (bool, error) {
	if rdb == nil || sid == "" {
		return false, nil
	}
	val, err := rdb.Get(revokedKey(sid))
	if err != nil {
		return false, err
	}
	return val != "", nil
}

// CheckUsable 在令牌轮换前校验会话状态：已吊销的会话不能再换发新令牌，
// 否则持续刷新会延长会话 key、却让 revoked 标记先过期，出现「吊销后复活」。
// Redis 不可用时降级放行，与 JWT 中间件的会话校验保持一致。
func CheckUsable(rdb *redis.Redis, sid string) error {
	if rdb == nil || sid == "" {
		return nil
	}
	revoked, err := IsRevoked(rdb, sid)
	if err != nil {
		return nil
	}
	if revoked {
		return ErrRevoked
	}
	return nil
}

func (s *Store) putSession(sess Session) error {
	b, err := json.Marshal(sess)
	if err != nil {
		return err
	}
	return s.rdb.Setex(sessionKey(sess.UserID, sess.SID), string(b), s.seconds())
}

func (s *Store) getSession(uid int64, sid string) (*Session, error) {
	val, err := s.rdb.Get(sessionKey(uid, sid))
	if err != nil {
		return nil, err
	}
	if val == "" {
		return nil, nil
	}
	var sess Session
	if err := json.Unmarshal([]byte(val), &sess); err != nil {
		return nil, err
	}
	return &sess, nil
}

func (s *Store) readIndex(uid int64) ([]string, error) {
	val, err := s.rdb.Get(indexKey(uid))
	if err != nil {
		return nil, err
	}
	if val == "" {
		return nil, nil
	}
	var sids []string
	err = json.Unmarshal([]byte(val), &sids)
	return sids, err
}

func (s *Store) writeIndex(uid int64, sids []string) error {
	b, err := json.Marshal(sids)
	if err != nil {
		return err
	}
	return s.rdb.Setex(indexKey(uid), string(b), s.seconds())
}

// appendIndex 的读-改-写只在登录时发生（低频）；并发登录的极端竞态最多影响列表展示，
// 不影响吊销：每条会话有独立 key 与吊销标记。
func (s *Store) appendIndex(uid int64, sid string) error {
	sids, err := s.readIndex(uid)
	if err != nil {
		return err
	}
	for _, v := range sids {
		if v == sid {
			return nil
		}
	}
	return s.writeIndex(uid, append(sids, sid))
}
