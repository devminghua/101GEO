package identicon

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"geo-tool/config"
)

// 5x5 网格，每行 5 个像素（true=实心）。中间列恒为实心，垂直镜像生成。
// 输出 5×5 共 15 个独立像素（中心列 5 个 + 左右各 5 个）。

const gridSize = 5
const pixelSize = 18 // 每个像素的尺寸
const padding = 4    // 边距
const total = gridSize*pixelSize + 2*padding // SVG 边长

// Generate 返回 SVG 字符串。输入任意非空字符串；空串自动用 "guest" 兜底。
func Generate(input string) string {
	if input == "" {
		input = "guest"
	}
	h := sha256.Sum256([]byte(input))
	// 取第 1 字节决定前景色相，第 4 字节决定背景色相；其余字节用作像素"开/关"位流
	r := int(h[0])
	_ = h[1]
	_ = h[2]
	// 柔和背景：HSL 浅色（饱和度低、明度高）
	bg := hslToHex(float64(h[3])/255*360, 0.18, 0.92)
	fg := hslToHex(float64(r)/255*360, 0.55, 0.45)
	// 中心点对称：从第 4 字节起取 12 位（5 行 × 3 列 + 中间列恒实心）
	// 用 3 字节 (24 位) 足够，但每个 bit 用 hash 决定
	var grid [gridSize][gridSize]bool
	// 中间列恒实心
	for y := 0; y < gridSize; y++ {
		grid[y][gridSize/2] = true
	}
	// 左边 2 列按 hash bit 决定（右侧自动镜像）
	idx := 4
	for y := 0; y < gridSize; y++ {
		for x := 0; x < gridSize/2; x++ {
			bit := (h[idx/8] >> uint(7-idx%8)) & 1
			grid[y][x] = bit == 1
			grid[y][gridSize-1-x] = bit == 1
			idx++
		}
	}

	// 渲染 SVG
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`+
		`<rect width="%d" height="%d" fill="#%s"/>`,
		total, total, total, total, total, total, bg)
	for y := 0; y < gridSize; y++ {
		for x := 0; x < gridSize; x++ {
			if grid[y][x] {
				svg += fmt.Sprintf(`<rect x="%d" y="%d" width="%d" height="%d" fill="#%s"/>`,
					padding+x*pixelSize, padding+y*pixelSize, pixelSize, pixelSize, fg)
			}
		}
	}
	svg += `</svg>`
	return svg
}

// hslToHex HSL 转 hex (#RRGGBB)。h∈[0,360) s∈[0,1] l∈[0,1]
func hslToHex(h, s, l float64) string {
	c := (1 - abs(2*l-1)) * s
	x := c * (1 - abs(fmod(h/60, 2)-1))
	m := l - c/2
	var r1, g1, b1 float64
	switch {
	case h < 60:
		r1, g1, b1 = c, x, 0
	case h < 120:
		r1, g1, b1 = x, c, 0
	case h < 180:
		r1, g1, b1 = 0, c, x
	case h < 240:
		r1, g1, b1 = 0, x, c
	case h < 300:
		r1, g1, b1 = x, 0, c
	default:
		r1, g1, b1 = c, 0, x
	}
	r := int((r1 + m) * 255)
	g := int((g1 + m) * 255)
	b := int((b1 + m) * 255)
	return fmt.Sprintf("%02X%02X%02X", r, g, b)
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func fmod(a, b float64) float64 {
	return a - float64(int(a/b))*b
}

// Save 基于 seed（用户名/手机号）生成 SVG，写入 uploads/avatars/{userID}.svg，
// 返回前端可访问的 URL（/uploads/avatars/{userID}.svg）。失败返回空串 + 错误，
// 调用方应回退到 initials 默认头像。
func Save(userID uint, seed string) (string, error) {
	svg := Generate(seed)
	dir := filepath.Join(config.Load().UploadsDir, "avatars")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("%d.svg", userID))
	if err := os.WriteFile(path, []byte(svg), 0o644); err != nil {
		return "", err
	}
	return "/uploads/avatars/" + fmt.Sprintf("%d.svg", userID), nil
}
