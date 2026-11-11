package password

import (
	"crypto/md5"
	"encoding/hex"
	"strings"
	"testing"
)

func TestHashAndVerify(t *testing.T) {
	plain := "S3cret!密码"
	hash := Hash(plain)
	if !strings.HasPrefix(hash, bcryptPrefix) {
		t.Fatalf("hash should be bcrypt, got %s", hash)
	}
	if !Verify(hash, plain) {
		t.Fatal("Verify should match Hash output")
	}
	if Verify(hash, plain+"x") {
		t.Fatal("Verify should reject wrong password")
	}
	if NeedsUpgrade(hash) != false {
		t.Fatal("bcrypt hash should not need upgrade")
	}
}

func TestLegacyVerifyAndUpgrade(t *testing.T) {
	plain := "123456"
	// 历史双重 MD5 加盐哈希（与升级前 Encode 算法一致）
	legacy := Md5Encode(Md5Encode(plain+"pmail") + "pmail2023")
	if !NeedsUpgrade(legacy) {
		t.Fatal("legacy hash should need upgrade")
	}
	if !Verify(legacy, plain) {
		t.Fatal("Verify should accept legacy hash")
	}
	if Verify(legacy, plain+"x") {
		t.Fatal("Verify should reject wrong password for legacy hash")
	}
}

func TestLongPasswordNormalize(t *testing.T) {
	// 100 字节口令：仅前 72 字节参与 bcrypt 校验
	long := strings.Repeat("a", 71) + "b" + strings.Repeat("c", 28)
	hash := Hash(long)
	if !Verify(hash, long) {
		t.Fatal("long password should verify against its normalized hash")
	}
	if Verify(hash, strings.Repeat("a", 71)+"c"+strings.Repeat("c", 28)) {
		t.Fatal("passwords differing within first 72 bytes must not match")
	}
	if !Verify(hash, strings.Repeat("a", 71)+"b"+strings.Repeat("d", 28)) {
		// 仅 72 字节之后不同的口令按 bcrypt 规范视为等价
		t.Fatal("passwords identical within first 72 bytes should match")
	}
}

func TestMd5EncodeStable(t *testing.T) {
	// Md5Encode 用于 APOP/Token 摘要，输出必须保持稳定
	if Md5Encode("pmail") != md5Hex("pmail") {
		t.Fatal("Md5Encode output changed")
	}
}

func md5Hex(s string) string {
	h := md5.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}
