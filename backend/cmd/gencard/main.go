package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"

	"geo-tool/services/card"
)

// 临时工具：生成 Ed25519 密钥对 + 测试卡密（供端到端验证；正式后台管理见 handlers）。
func main() {
	privB64 := flag.String("priv", "", "私钥 base64（复用已有私钥生成卡密时传入）")
	count := flag.Int("n", 3, "生成卡密数量")
	tier := flag.Int("tier", int(card.Tier12Month), "卡密类型 1=3天 2=12个月")
	startID := flag.Int("start", 1, "卡密起始 ID")
	flag.Parse()

	var pubB64, key string
	var priv ed25519.PrivateKey

	if *privB64 != "" {
		raw, err := base64.StdEncoding.DecodeString(*privB64)
		if err != nil {
			panic(err)
		}
		priv = ed25519.PrivateKey(raw)
		pubB64 = base64.StdEncoding.EncodeToString(priv.Public().(ed25519.PublicKey))
		key = *privB64
	} else {
		pub, p, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			panic(err)
		}
		priv = p
		pubB64 = base64.StdEncoding.EncodeToString(pub)
		key = base64.StdEncoding.EncodeToString(priv)
	}

	fmt.Println("GEO_LICENSE_PUBLIC_KEY=" + pubB64)
	fmt.Println("PRIVATE_KEY_B64=" + key)
	for i := 0; i < *count; i++ {
		code, err := card.Generate(priv, uint32(*startID+i), byte(*tier))
		if err != nil {
			panic(err)
		}
		fmt.Printf("卡密[%d] tier=%d: %s\n", *startID+i, *tier, code)
	}
}
