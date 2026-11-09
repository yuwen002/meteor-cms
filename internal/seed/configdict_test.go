package seed

import "testing"

const (
	newSiteName   = "Meteor CMS"
	newSiteFooter = "© {year} Meteor CMS"
)

func TestRenamedDisplayValue(t *testing.T) {
	for _, tc := range []struct {
		name    string
		key     string
		current string
		want    string
	}{
		{"更名前站名", "site_name", "Go Meteor CMS", newSiteName},
		{"更名前页脚", "site_footer", "© 2026 Go Meteor CMS", newSiteFooter},
		{"已更名但未模板化的页脚", "site_footer", "© 2026 Meteor CMS", newSiteFooter},
	} {
		t.Run(tc.name, func(t *testing.T) {
			seedValue := newSiteName
			if tc.key == "site_footer" {
				seedValue = newSiteFooter
			}
			if v, ok := renamedDisplayValue(tc.key, tc.current, seedValue); !ok || v != tc.want {
				t.Fatalf("历史默认值应迁移为 %q: v=%q ok=%v", tc.want, v, ok)
			}
		})
	}
	for _, tc := range []struct {
		name    string
		key     string
		current string
		seed    string
	}{
		{"运营自定义站名", "site_name", "我的博客", newSiteName},
		{"已是新站名", "site_name", newSiteName, newSiteName},
		{"运营自定义页脚", "site_footer", "© 2026 我的博客", newSiteFooter},
		{"已是年份模板", "site_footer", newSiteFooter, newSiteFooter},
		{"非显示类配置", "site_description", "Go Meteor CMS", newSiteName},
		{"空值", "site_name", "", newSiteName},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, ok := renamedDisplayValue(tc.key, tc.current, tc.seed); ok {
				t.Fatal("不应触发迁移")
			}
		})
	}
}

// 迁移目标必须始终等于当前种子默认值，否则每次迁移都会写入一个过期名字。
func TestDisplaySeedValuesMatchMigrationTarget(t *testing.T) {
	want := map[string]string{
		"site_name":   newSiteName,
		"site_footer": newSiteFooter,
	}
	seen := 0
	for _, c := range defaultConfigs {
		if expected, ok := want[c.key]; ok {
			seen++
			if c.value != expected {
				t.Errorf("%s 种子值为 %q，与迁移目标 %q 不一致", c.key, c.value, expected)
			}
		}
	}
	if seen != len(want) {
		t.Fatalf("种子清单缺少显示类配置：期望 %d 项，实际 %d 项", len(want), seen)
	}
}

// 历史值清单的键必须仍在种子清单里，否则迁移会写入一个永远不会被读取的配置项。
func TestLegacyDisplayKeysStillSeeded(t *testing.T) {
	seeded := make(map[string]bool, len(defaultConfigs))
	for _, c := range defaultConfigs {
		seeded[c.key] = true
	}
	for key := range legacyDisplayValues {
		if !seeded[key] {
			t.Errorf("历史值清单的 %q 已不在种子清单中", key)
		}
	}
}
