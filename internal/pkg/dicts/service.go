// Package dicts 提供数据字典的统一读取入口，供公开 API 与后台下拉使用。
package dicts

import (
	"context"
	"sync"
	"time"

	"github.com/yuwen002/meteor-cms/ent"
	"github.com/yuwen002/meteor-cms/ent/sysdict"
	"github.com/yuwen002/meteor-cms/ent/sysdictitem"
	"github.com/yuwen002/meteor-cms/internal/pkg/cacheversion"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

const defaultTTL = 30 * time.Second

type Item struct {
	Label     string `json:"label"`
	Value     string `json:"value"`
	IsDefault bool   `json:"is_default"`
	ParentID  int64  `json:"parent_id"`
	Sort      int    `json:"sort"`
}

type Service struct {
	client  *ent.Client
	ttl     time.Duration
	version *cacheversion.Sentinel

	mu      sync.RWMutex
	values  map[string][]Item
	expires time.Time
}

// New 的可选 rds 用于跨实例失效；不传时退化为纯进程内 TTL。
func New(client *ent.Client, rds ...*redis.Redis) *Service {
	var r *redis.Redis
	if len(rds) > 0 {
		r = rds[0]
	}
	return &Service{
		client:  client,
		ttl:     defaultTTL,
		version: cacheversion.New(r, "cms:cache:dicts:version", 0),
		values:  make(map[string][]Item),
	}
}

func (s *Service) invalidateLocal() {
	s.mu.Lock()
	s.values = make(map[string][]Item)
	s.expires = time.Time{}
	s.mu.Unlock()
}

// Invalidate 使缓存立即失效（字典写操作后调用），并广播给其它实例。
func (s *Service) Invalidate() {
	s.invalidateLocal()
	s.version.Bump()
}

// Items 只返回启用且所属字典类型启用的项，按 sort / id 排序。
func (s *Service) Items(ctx context.Context, code string) []Item {
	if s.version.Changed() {
		s.invalidateLocal()
	}
	s.mu.RLock()
	if items, ok := s.values[code]; ok && time.Now().Before(s.expires) {
		out := append([]Item(nil), items...)
		s.mu.RUnlock()
		return out
	}
	s.mu.RUnlock()

	// 字典类型被禁用或不存在时不暴露任何项
	enabled, err := s.client.SysDict.Query().
		Where(sysdict.CodeEQ(code), sysdict.StatusEQ(1)).
		Exist(ctx)
	if err != nil {
		logx.Errorf("检查字典类型 %s 失败: %v", code, err)
		return []Item{}
	}
	if !enabled {
		return []Item{}
	}

	rows, err := s.client.SysDictItem.Query().
		Where(
			sysdictitem.DictCodeEQ(code),
			sysdictitem.StatusEQ(1),
		).
		Order(ent.Asc(sysdictitem.FieldSort), ent.Asc(sysdictitem.FieldID)).
		All(ctx)
	if err != nil {
		logx.Errorf("加载字典 %s 失败: %v", code, err)
		s.mu.RLock()
		out := append([]Item(nil), s.values[code]...)
		s.mu.RUnlock()
		return out
	}

	items := make([]Item, 0, len(rows))
	for _, row := range rows {
		items = append(items, Item{
			Label: row.Label, Value: row.Value,
			IsDefault: row.IsDefault, ParentID: row.ParentID, Sort: row.Sort,
		})
	}

	s.mu.Lock()
	if s.values == nil {
		s.values = make(map[string][]Item)
	}
	s.values[code] = items
	s.expires = time.Now().Add(s.ttl)
	s.mu.Unlock()
	return append([]Item(nil), items...)
}
