package authsession

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

func newTestStore(t *testing.T) (*Store, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("启动 miniredis 失败: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb, err := redis.NewRedis(redis.RedisConf{Host: mr.Addr(), Type: "node"})
	if err != nil {
		t.Fatalf("初始化 Redis 客户端失败: %v", err)
	}
	return New(rdb, time.Hour), mr
}

func TestCreateListAndRevoke(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	sid := NewSID()

	if err := s.Create(ctx, 7, sid, "10.0.0.1", "test-agent", "hash-1"); err != nil {
		t.Fatalf("登记会话失败: %v", err)
	}
	list, err := s.List(ctx, 7)
	if err != nil || len(list) != 1 {
		t.Fatalf("会话列表 = %v, err=%v", list, err)
	}
	if list[0].SID != sid || list[0].IP != "10.0.0.1" || list[0].AccessHash != "hash-1" {
		t.Fatalf("会话内容不符: %+v", list[0])
	}

	if revoked, err := IsRevoked(s.rdb, sid); err != nil || revoked {
		t.Fatalf("新会话不应被标记为吊销: revoked=%v err=%v", revoked, err)
	}
	got, err := s.Revoke(ctx, 7, sid)
	if err != nil || got == nil || got.AccessHash != "hash-1" {
		t.Fatalf("吊销应返回会话快照: %+v err=%v", got, err)
	}
	if revoked, err := IsRevoked(s.rdb, sid); err != nil || !revoked {
		t.Fatalf("吊销后标记应生效: revoked=%v err=%v", revoked, err)
	}
	list, err = s.List(ctx, 7)
	if err != nil || len(list) != 0 {
		t.Fatalf("吊销后列表应为空: %v err=%v", list, err)
	}
}

func TestTouchAndUserIsolation(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	sidA, sidB := NewSID(), NewSID()

	if err := s.Create(ctx, 1, sidA, "1.1.1.1", "agent-a", "hash-a"); err != nil {
		t.Fatal(err)
	}
	if err := s.Create(ctx, 2, sidB, "2.2.2.2", "agent-b", "hash-b"); err != nil {
		t.Fatal(err)
	}
	if err := s.Touch(ctx, 1, sidA, "hash-a2"); err != nil {
		t.Fatalf("更新会话失败: %v", err)
	}

	listA, _ := s.List(ctx, 1)
	listB, _ := s.List(ctx, 2)
	if len(listA) != 1 || len(listB) != 1 {
		t.Fatalf("用户会话应隔离: A=%d B=%d", len(listA), len(listB))
	}
	if listA[0].AccessHash != "hash-a2" || listB[0].AccessHash != "hash-b" {
		t.Fatalf("Touch 影响了其它会话: A=%s B=%s", listA[0].AccessHash, listB[0].AccessHash)
	}
	if _, err := s.Revoke(ctx, 2, sidA); !errors.Is(err, ErrNotFound) {
		t.Fatalf("不能吊销其它用户的会话: %v", err)
	}
}

func TestExpiresAndUnavailable(t *testing.T) {
	s, mr := newTestStore(t)
	ctx := context.Background()
	sid := NewSID()
	if err := s.Create(ctx, 9, sid, "1.1.1.1", "agent", "hash"); err != nil {
		t.Fatal(err)
	}
	mr.FastForward(2 * time.Hour)
	list, err := s.List(ctx, 9)
	if err != nil || len(list) != 0 {
		t.Fatalf("过期会话不应出现在列表: %v err=%v", list, err)
	}

	nilStore := New(nil, time.Hour)
	if err := nilStore.Create(ctx, 9, NewSID(), "", "", ""); err != nil {
		t.Fatalf("无 Redis 时登记应降级: %v", err)
	}
	if _, err := nilStore.List(ctx, 9); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("无 Redis 时列表应明确报错: %v", err)
	}
	if _, err := nilStore.Revoke(ctx, 9, sid); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("无 Redis 时吊销应明确报错: %v", err)
	}
	if revoked, err := IsRevoked(nil, sid); err != nil || revoked {
		t.Fatalf("无 Redis 时中间件校验应放行: revoked=%v err=%v", revoked, err)
	}
}

func TestCheckUsable(t *testing.T) {
	s, mr := newTestStore(t)
	ctx := context.Background()
	sid := NewSID()
	if err := s.Create(ctx, 3, sid, "10.0.0.1", "agent", "hash"); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		rdb  *redis.Redis
		sid  string
	}{
		{"正常会话", s.rdb, sid},
		{"无 Redis 降级", nil, sid},
		{"旧令牌无 sid", s.rdb, ""},
	} {
		if err := CheckUsable(tc.rdb, tc.sid); err != nil {
			t.Fatalf("%s: 应允许轮换, 实际 %v", tc.name, err)
		}
	}

	if _, err := s.Revoke(ctx, 3, sid); err != nil {
		t.Fatal(err)
	}
	if err := CheckUsable(s.rdb, sid); !errors.Is(err, ErrRevoked) {
		t.Fatalf("已吊销会话应拒绝轮换, 实际 %v", err)
	}

	// Redis 不可用时降级放行，与 JWT 中间件一致。
	mr.Close()
	if err := CheckUsable(s.rdb, sid); err != nil {
		t.Fatalf("Redis 异常时应降级放行, 实际 %v", err)
	}
}
