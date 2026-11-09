package secretbox

import "testing"

func TestEncryptDecryptRoundTrip(t *testing.T) {
	for _, plain := range []string{"p@ssw0rd", "中文密钥", "a-very-long-value-with-special/chars+=?"} {
		stored, err := Encrypt(plain, "test-secret")
		if err != nil {
			t.Fatalf("加密失败: %v", err)
		}
		if stored == plain {
			t.Fatal("密文不能等于明文")
		}
		got, err := Decrypt(stored, "test-secret")
		if err != nil {
			t.Fatalf("解密失败: %v", err)
		}
		if got != plain {
			t.Fatalf("往返不一致: want %q got %q", plain, got)
		}
	}
}

func TestEncryptEmpty(t *testing.T) {
	got, err := Encrypt("", "test-secret")
	if err != nil || got != "" {
		t.Fatalf("空串应原样返回: %q %v", got, err)
	}
}

func TestDecryptLegacyPlain(t *testing.T) {
	got, err := Decrypt("legacy-plain", "test-secret")
	if err != nil || got != "legacy-plain" {
		t.Fatalf("历史明文应兼容读取: %q %v", got, err)
	}
}

func TestDecryptWrongSecretFails(t *testing.T) {
	stored, _ := Encrypt("secret", "key-a")
	if _, err := Decrypt(stored, "key-b"); err == nil {
		t.Fatal("错误密钥应解密失败")
	}
}
