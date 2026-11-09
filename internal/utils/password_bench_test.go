package utils

import "testing"

// 密码复杂度校验是创建/改密入口的同步开销，热点在于逐 rune 分类判断。
// bcrypt 哈希故意不纳入基准：它本身是刻意慢哈希，数值只反映 cost 参数。
func BenchmarkValidatePasswordComplexity(b *testing.B) {
	const password = "Str0ng!Passw0rd"
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if ok, _ := ValidatePasswordComplexity(password); !ok {
			b.Fatal("基准用例密码应通过复杂度校验")
		}
	}
}
