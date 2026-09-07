package handlers

import (
	"testing"
	"time"

	"geo-tool/models"
)

// 直接验证授权签名逻辑（不依赖数据库/WAL/进程），覆盖篡改检测的各类场景。
func TestActivationSignAndVerify(t *testing.T) {
	now := time.Now()
	expireAt := now.AddDate(0, 0, 365)

	a := models.Activation{CardID: 1, Tier: 2, ActivatedAt: now, ExpireAt: expireAt}
	a.Sig = signActivation(a.CardID, a.Tier, a.ActivatedAt, a.ExpireAt)

	// 1) 未篡改：应通过
	if !activationValid(&a) {
		t.Fatal("未篡改的签名应通过")
	}

	// 2) 篡改到期时间（白嫖最常见手段）：应失败
	b := a
	b.ExpireAt = now.AddDate(10, 0, 0)
	if activationValid(&b) {
		t.Fatal("篡改到期时间后签名应失败")
	}

	// 3) 篡改卡密序号：应失败
	c := a
	c.CardID = 9999
	if activationValid(&c) {
		t.Fatal("篡改卡密序号后签名应失败")
	}

	// 4) 篡改类型：应失败
	d := a
	d.Tier = 1
	if activationValid(&d) {
		t.Fatal("篡改类型后签名应失败")
	}

	// 5) 篡改激活时间：应失败
	e := a
	e.ActivatedAt = now.AddDate(-1, 0, 0)
	if activationValid(&e) {
		t.Fatal("篡改激活时间后签名应失败")
	}
}
