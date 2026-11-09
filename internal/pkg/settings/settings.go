// Package settings 是系统配置的统一读取入口。
//
// 后台改配置后调用 Invalidate 立即生效：清空本实例缓存并通过 Redis 版本哨兵
// 广播给其它实例；进程内缓存带短 TTL 作为 Redis 不可用时的兜底。
package settings

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/sysconfig"
	"github.com/yuwen002/meteor-cms/internal/pkg/cacheversion"
	"github.com/yuwen002/meteor-cms/internal/pkg/secretbox"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

const defaultTTL = 30 * time.Second

type Service struct {
	client  *ent.Client
	secret  string
	ttl     time.Duration
	version *cacheversion.Sentinel

	mu      sync.RWMutex
	values  map[string]string
	expires time.Time
}

// New 的可选 rds 用于跨实例失效；不传时退化为纯进程内 TTL（单实例 / 本地开发）。
func New(client *ent.Client, secret string, rds ...*redis.Redis) *Service {
	var r *redis.Redis
	if len(rds) > 0 {
		r = rds[0]
	}
	return &Service{
		client:  client,
		secret:  secret,
		ttl:     defaultTTL,
		version: cacheversion.New(r, "cms:cache:settings:version", 0),
	}
}

func (s *Service) invalidateLocal() {
	s.mu.Lock()
	s.values = nil
	s.expires = time.Time{}
	s.mu.Unlock()
}

// Invalidate 使缓存立即失效（配置写操作后调用），并广播给其它实例。
func (s *Service) Invalidate() {
	s.invalidateLocal()
	s.version.Bump()
}

// All 返回所有启用配置的 key -> 明文值快照。
func (s *Service) All(ctx context.Context) map[string]string {
	if s.version.Changed() {
		s.invalidateLocal()
	}
	s.mu.RLock()
	if s.values != nil && time.Now().Before(s.expires) {
		out := make(map[string]string, len(s.values))
		for k, v := range s.values {
			out[k] = v
		}
		s.mu.RUnlock()
		return out
	}
	s.mu.RUnlock()

	rows, err := s.client.SysConfig.Query().
		Where(sysconfig.StatusEQ(1)).
		All(ctx)
	if err != nil {
		logx.Errorf("加载系统配置失败: %v", err)
		// DB 异常时返回上一次快照（若存在），避免站点信息突变
		s.mu.RLock()
		out := make(map[string]string, len(s.values))
		for k, v := range s.values {
			out[k] = v
		}
		s.mu.RUnlock()
		return out
	}

	next := make(map[string]string, len(rows))
	for _, row := range rows {
		value := row.ConfigValue
		if row.IsSecret {
			plain, err := secretbox.Decrypt(value, s.secret)
			if err != nil {
				logx.Errorf("解密配置 %s 失败: %v", row.ConfigKey, err)
				continue
			}
			value = plain
		}
		next[row.ConfigKey] = value
	}

	s.mu.Lock()
	s.values = next
	s.expires = time.Now().Add(s.ttl)
	s.mu.Unlock()

	out := make(map[string]string, len(next))
	for k, v := range next {
		out[k] = v
	}
	return out
}

func (s *Service) Get(ctx context.Context, key, fallback string) string {
	if v, ok := s.All(ctx)[key]; ok && v != "" {
		return v
	}
	return fallback
}

func (s *Service) Bool(ctx context.Context, key string, fallback bool) bool {
	v, ok := s.All(ctx)[key]
	if !ok || v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func (s *Service) Int(ctx context.Context, key string, fallback int) int {
	v, ok := s.All(ctx)[key]
	if !ok || v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
