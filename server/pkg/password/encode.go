package password

import (
	"crypto/md5"
	"crypto/subtle"
	"encoding/hex"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// bcrypt 哈希前缀
const bcryptPrefix = "$2"

// bcrypt 规范限制明文最长 72 字节，超长部分统一截断以保证 Hash/Verify 一致
const bcryptMaxPlainLen = 72

// Hash 使用 bcrypt 生成口令哈希
func Hash(plain string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(normalize(plain)), bcrypt.DefaultCost)
	if err != nil {
		// normalize 后长度不会超过 72，理论上不会走到这里
		return ""
	}
	return string(hash)
}

// Verify 校验口令与存储哈希是否匹配。
// 存储哈希为 bcrypt 时使用 bcrypt 校验；
// 历史数据为双重 MD5 加盐哈希，做恒时比较以保持兼容，登录成功后由调用方透明升级。
func Verify(stored, plain string) bool {
	if strings.HasPrefix(stored, bcryptPrefix) {
		return bcrypt.CompareHashAndPassword([]byte(stored), []byte(normalize(plain))) == nil
	}
	// 历史双重 MD5 哈希兼容校验
	return subtle.ConstantTimeCompare([]byte(stored), []byte(LegacyEncode(plain))) == 1
}

// NeedsUpgrade 判断存储哈希是否仍为历史双重 MD5，需要升级为 bcrypt
func NeedsUpgrade(stored string) bool {
	return !strings.HasPrefix(stored, bcryptPrefix)
}

// LegacyEncode 历史口令哈希算法：两次 MD5 加盐。仅用于兼容存量数据校验，新哈希一律使用 Hash。
func LegacyEncode(plain string) string {
	return Md5Encode(Md5Encode(plain+"pmail") + "pmail2023")
}

// normalize bcrypt 明文长度规范
func normalize(plain string) string {
	if len(plain) > bcryptMaxPlainLen {
		return plain[:bcryptMaxPlainLen]
	}
	return plain
}

// Md5Encode 通用 MD5 摘要，用于 APOP 挑战应答（RFC 1939）与 Token 时间戳摘要等非口令存储场景
func Md5Encode(str string) string {
	h := md5.New()
	h.Write([]byte(str))
	return hex.EncodeToString(h.Sum(nil))
}
