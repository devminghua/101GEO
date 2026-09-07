package card

import (
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"
)

func TestGenerateAndVerify(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	code, err := Generate(priv, 1001, Tier12Month)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(code, prefix) {
		t.Fatalf("卡密应以 %s 开头: %s", prefix, code)
	}
	id, tier, err := Verify(pub, code)
	if err != nil {
		t.Fatalf("验签失败: %v", err)
	}
	if id != 1001 || tier != Tier12Month {
		t.Fatalf("解析错误: id=%d tier=%d", id, tier)
	}
}

func TestVerifyTampered(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	code, _ := Generate(priv, 1001, Tier12Month)
	// 篡改中间某个字符（末字符只含 1 个有效位，篡改它不可靠）
	mid := len(code) / 2
	c := byte('0')
	if code[mid] == '0' {
		c = '1'
	}
	tampered := code[:mid] + string(c) + code[mid+1:]
	if _, _, err := Verify(pub, tampered); err == nil {
		t.Fatal("篡改后的卡密应验签失败")
	}
}

func TestVerifyWrongKey(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	otherPub, _, _ := ed25519.GenerateKey(rand.Reader)
	code, _ := Generate(priv, 1001, Tier12Month)
	if _, _, err := Verify(otherPub, code); err == nil {
		t.Fatal("用错误公钥应验签失败")
	}
}

func TestDecodeToleratesFormatting(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	code, _ := Generate(priv, 7, TierTrial3Day)
	// 小写 + 去掉分隔符
	normalized := strings.ToLower(strings.ReplaceAll(code, "-", ""))
	id, tier, err := Verify(pub, normalized)
	if err != nil {
		t.Fatalf("容错解析失败: %v", err)
	}
	if id != 7 || tier != TierTrial3Day {
		t.Fatalf("解析错误: id=%d tier=%d", id, tier)
	}
}

func TestTierDays(t *testing.T) {
	if TierDays(TierTrial3Day) != 3 {
		t.Fatal("3 天试用应返回 3")
	}
	if TierDays(Tier12Month) != 365 {
		t.Fatal("12 个月应返回 365")
	}
	if TierDays(99) != 0 {
		t.Fatal("未知类型应返回 0")
	}
}

func TestVerifyEmpty(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	if _, _, err := Verify(pub, ""); err == nil {
		t.Fatal("空卡密应报错")
	}
	if _, _, err := Verify(pub, "LG-"); err == nil {
		t.Fatal("空 payload 应报错")
	}
}
