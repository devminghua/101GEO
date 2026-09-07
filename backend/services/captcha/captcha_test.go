package captcha

import "testing"

func TestGenerate(t *testing.T) {
	id := Generate()
	if len(id) != 32 {
		t.Fatalf("id 长度异常: %d", len(id))
	}
}

// 模拟真人轨迹：起点 0，逐步加速到最右端，位移有波动
func humanTrack() []int {
	return []int{0, 12, 27, 45, 70, 102, 140, 183, 226, 254, 268, 272}
}

func TestVerifyPassAndOneShot(t *testing.T) {
	id := Generate()
	if !Verify(id, slideRange, humanTrack()) {
		t.Fatal("拖到底 + 真人轨迹应通过")
	}
	if Verify(id, slideRange, humanTrack()) {
		t.Fatal("凭证应一次性作废")
	}
}

func TestVerifyNotReachEnd(t *testing.T) {
	id := Generate()
	if Verify(id, slideRange-tolerance-1, humanTrack()) {
		t.Fatal("未拖到最右端应拒绝")
	}
}

func TestVerifyToleranceBoundary(t *testing.T) {
	id := Generate()
	if !Verify(id, slideRange-tolerance, humanTrack()) {
		t.Fatal("容差边界内应通过")
	}
}

func TestVerifyNoTrack(t *testing.T) {
	id := Generate()
	if Verify(id, slideRange, nil) {
		t.Fatal("无轨迹（脚本直接提交 slide_x）应拒绝")
	}
	if Verify(id, slideRange, []int{272}) {
		t.Fatal("单点轨迹应拒绝")
	}
}

func TestVerifyUniformTrack(t *testing.T) {
	id := Generate()
	// 匀速轨迹：位移恒定（50），机器行为
	uniform := []int{0, 50, 100, 150, 200, 250}
	if Verify(id, slideRange, uniform) {
		t.Fatal("匀速轨迹应拒绝")
	}
}

func TestVerifyInvalidID(t *testing.T) {
	if Verify("deadbeef", slideRange, humanTrack()) {
		t.Fatal("无效凭证应拒绝")
	}
}
