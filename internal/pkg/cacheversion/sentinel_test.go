package cacheversion

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

func newTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Redis) {
	t.Helper()
	s := miniredis.RunT(t)
	rdb, err := redis.NewRedis(redis.RedisConf{Host: s.Addr(), Type: redis.NodeType})
	if err != nil {
		t.Fatalf("NewRedis failed: %v", err)
	}
	return s, rdb
}

func TestChangedAcrossInstances(t *testing.T) {
	s, rdb := newTestRedis(t)
	short := 10 * time.Millisecond
	writer := New(rdb, "cms:cache:test:version", short)
	reader := New(rdb, "cms:cache:test:version", short)

	// 两端都还没见过版本键：首次读取只建立基线，不应触发刷新。
	if reader.Changed() {
		t.Fatal("首次读取只应建立基线")
	}
	if writer.Changed() {
		t.Fatal("首次读取只应建立基线")
	}

	writer.Bump()
	// 写方刚 Bump 过，基线已同步，不应自己误判。
	if writer.Changed() {
		t.Fatal("写方不应因自己的 Bump 触发刷新")
	}
	// 读方下一次检查必须看到版本变化。
	time.Sleep(short + time.Millisecond)
	if !reader.Changed() {
		t.Fatal("其它实例写入后读方应检测到版本变化")
	}
	// 变化只报一次。
	time.Sleep(short + time.Millisecond)
	if reader.Changed() {
		t.Fatal("版本未再变化时不应重复触发刷新")
	}

	writer.Bump()
	time.Sleep(short + time.Millisecond)
	if !reader.Changed() {
		t.Fatal("第二次写入也应被检测到")
	}
	_ = s
}

func TestNoRedisDegradesQuietly(t *testing.T) {
	if got := (*Sentinel)(nil).Changed(); got {
		t.Fatal("nil Sentinel 不应触发刷新")
	}
	s := New(nil, "cms:cache:test:version", 0)
	s.Bump()
	if s.Changed() {
		t.Fatal("未配置 Redis 时应退化为本地 TTL")
	}
}

func TestRedisFailureDoesNotForceRefresh(t *testing.T) {
	mr, rdb := newTestRedis(t)
	s := New(rdb, "cms:cache:test:version", time.Millisecond)
	if s.Changed() {
		t.Fatal("首次读取只应建立基线")
	}
	mr.Close()
	time.Sleep(2 * time.Millisecond)
	if s.Changed() {
		t.Fatal("Redis 不可用时应保持本地快照，不应反复强制刷新")
	}
}
