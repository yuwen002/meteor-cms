// Package cacheversion 为进程内 TTL 缓存提供跨实例失效能力。
//
// 每个缓存服务持有一个 Sentinel：写路径调用 Bump() 递增 Redis 版本键，
// 读路径调用 Changed() 比对版本，变化时强制刷新本地快照。
// Redis 不可用或版本键尚不存在时 Changed() 返回 false，各实例退化为依赖本地 TTL，
// 不会因缓存组件故障而放大成站点不可用。
package cacheversion

import (
	"strconv"
	"sync"
	"time"

	"github.com/zeromicro/go-zero/core/stores/redis"
)

const defaultCheckEvery = time.Second

// Sentinel 记录「上次见到的版本号」。checkEvery 用来节流 Redis 往返：
// 默认每秒最多检查一次，把多实例不一致窗口从 30~60s 压到约 1s，
// 同时避免每个请求都打一次 Redis。
type Sentinel struct {
	rds        *redis.Redis
	key        string
	checkEvery time.Duration

	mu          sync.Mutex
	last        string
	initialized bool
	checked     time.Time
}

func New(rds *redis.Redis, key string, checkEvery time.Duration) *Sentinel {
	if checkEvery <= 0 {
		checkEvery = defaultCheckEvery
	}
	return &Sentinel{rds: rds, key: key, checkEvery: checkEvery}
}

// Bump 在写操作成功后调用：递增版本键，并把本实例基线同步到新值，
// 避免自己刚写完的第一次读又被判成「版本变化」而重复查库。
func (s *Sentinel) Bump() {
	if s == nil || s.rds == nil {
		return
	}
	n, err := s.rds.Incr(s.key)
	if err != nil {
		return
	}
	s.mu.Lock()
	s.last = strconv.FormatInt(n, 10)
	s.initialized = true
	s.checked = time.Now()
	s.mu.Unlock()
}

// Changed 返回其它实例（或本实例其它入口）是否已经写过。
func (s *Sentinel) Changed() bool {
	if s == nil || s.rds == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.initialized && time.Since(s.checked) < s.checkEvery {
		return false
	}
	// go-zero 的 Get 在 key 不存在时返回 ("", nil)，这里把空串当作版本 0；
	// 真正的 Redis 错误（连接失败等）返回 false，保持本地 TTL 不误判。
	v, err := s.rds.Get(s.key)
	if err != nil {
		return false
	}
	s.checked = time.Now()
	if s.initialized {
		if v == s.last {
			return false
		}
		s.last = v
		return true
	}
	s.last = v
	s.initialized = true
	return false
}
