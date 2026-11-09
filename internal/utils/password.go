package utils

import (
	"strconv"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

// MinPasswordLength 密码最小长度
const MinPasswordLength = 8

// ValidatePasswordComplexity 校验密码复杂度：
//   - 长度 >= MinPasswordLength
//   - 至少包含 3 类字符（小写字母、大写字母、数字、特殊字符）
//
// 返回 (是否通过, 失败原因)。仅业务入口（创建/注册/改密/重置）调用本函数；
// 初始化种子直接构造 AdminUser，写入的 admin/123456 是本策略的唯一例外，
// 保留它是为了让本地开发与 E2E 开箱即用，生产首次登录后必须立即修改。
func ValidatePasswordComplexity(password string) (bool, string) {
	if len(password) < MinPasswordLength {
		return false, "密码长度不能少于 " + strconv.Itoa(MinPasswordLength) + " 位"
	}

	var lower, upper, digit, special bool
	for _, r := range password {
		switch {
		case unicode.IsLower(r):
			lower = true
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsDigit(r):
			digit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			special = true
		}
	}

	classes := 0
	if lower {
		classes++
	}
	if upper {
		classes++
	}
	if digit {
		classes++
	}
	if special {
		classes++
	}
	if classes < 3 {
		return false, "密码需包含大写字母、小写字母、数字、特殊字符中的至少三类"
	}
	return true, ""
}

// ValidatePassword 校验密码长度是否合法（向后兼容：仅长度检查）
func ValidatePassword(password string) bool {
	return len(password) >= MinPasswordLength
}

// HashPassword 对密码进行加密
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPassword 校验明文密码与哈希是否匹配
func CheckPassword(hashedPassword, plainPassword string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(plainPassword))
	return err == nil
}
