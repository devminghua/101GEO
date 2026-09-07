package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"flag"
	"fmt"
	"os"

	"geo-tool/services/card"
)

// gencards 批量生成卡密到文件（每行一张纯卡密串，无 header，便于直接分发/导入）。
// 用法：go run ./cmd/gencards -priv <私钥base64> -tier 1|2 -n 数量 -start 起始ID -out 输出文件
func main() {
	privB64 := flag.String("priv", "", "私钥 base64（必填）")
	tier := flag.Int("tier", 2, "卡密类型 1=3天试用 2=12个月")
	n := flag.Int("n", 100, "生成数量")
	start := flag.Int("start", 1, "起始卡密 ID（防重，批量时错开）")
	out := flag.String("out", "cards.txt", "输出文件（每行一张卡密）")
	flag.Parse()

	if *privB64 == "" {
		fmt.Fprintln(os.Stderr, "缺少 -priv 私钥")
		os.Exit(1)
	}
	raw, err := base64.StdEncoding.DecodeString(*privB64)
	if err != nil || len(raw) != ed25519.PrivateKeySize {
		fmt.Fprintln(os.Stderr, "私钥格式错误:", err)
		os.Exit(1)
	}
	priv := ed25519.PrivateKey(raw)

	f, err := os.Create(*out)
	if err != nil {
		fmt.Fprintln(os.Stderr, "创建输出文件失败:", err)
		os.Exit(1)
	}
	defer f.Close()

	for i := 0; i < *n; i++ {
		code, err := card.Generate(priv, uint32(*start+i), byte(*tier))
		if err != nil {
			fmt.Fprintln(os.Stderr, "生成失败:", err)
			os.Exit(1)
		}
		fmt.Fprintln(f, code)
	}
	fmt.Printf("已生成 %d 张卡密（%s，ID %d-%d）→ %s\n", *n, card.TierName(byte(*tier)), *start, *start+*n-1, *out)
}
