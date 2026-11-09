package utils

import "testing"

func TestValidatePasswordComplexity(t *testing.T) {
	cases := []struct {
		pwd  string
		want bool
	}{
		{"abc", false},        // 过短
		{"Ab1!", false},       // 过短(5)，虽含3类
		{"12345678", false},   // 仅数字
		{"abcdefgh", false},   // 仅小写
		{"ABCDEFGH", false},   // 仅大写
		{"Abcdefgh", false},   // 大小写2类
		{"Abcdef12", true},    // 大小写+数字3类
		{"Admin123456", true}, // 合规密码样例（不是种子密码；种子为 admin/123456）
		{"Ab!1wxyz", true},    // 大小写+数字+特殊4类（8位）
		{"Abcdefg1", true},    // 大小写+数字3类
	}
	for _, c := range cases {
		ok, _ := ValidatePasswordComplexity(c.pwd)
		if ok != c.want {
			t.Errorf("ValidatePasswordComplexity(%q) = %v, want %v", c.pwd, ok, c.want)
		}
	}
}
