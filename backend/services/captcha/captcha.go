package captcha

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

const (
	slideRange  = 272 // 前端滑块行程：轨道宽 320 - 滑块宽 48
	tolerance   = 4   // 拖到最右端的容差
	minTrackPts = 4   // 最少轨迹采样点：真人拖动至少触发多次 mousemove
	ttl         = 5 * time.Minute
)

type entry struct {
	expire time.Time
}

var (
	store   = make(map[string]entry)
	storeMu sync.Mutex
)

// Generate 下发一次性滑动解锁凭证 id（无图像，纯滑块）。
func Generate() string {
	id := newID()
	storeMu.Lock()
	now := time.Now()
	for k, v := range store { // 惰性清理过期凭证
		if v.expire.Before(now) {
			delete(store, k)
		}
	}
	store[id] = entry{expire: now.Add(ttl)}
	storeMu.Unlock()
	return id
}

// Verify 校验滑动：拖到最右端 + 轨迹像真人拖动；凭证一次性，验证后立即作废。
func Verify(id string, slideX int, track []int) bool {
	storeMu.Lock()
	_, ok := store[id]
	if ok {
		delete(store, id)
	}
	storeMu.Unlock()
	if !ok {
		return false
	}
	if slideX < slideRange-tolerance {
		return false
	}
	return humanLikeTrack(track, slideX)
}

// humanLikeTrack 判断拖动轨迹是否像真人操作，用于挡掉「直接 POST 伪造 slide_x」的脚本：
//   - 采样点足够多（脚本通常不带轨迹或只给一两个点）
//   - 首点靠近起点、末点靠近终点
//   - 相邻位移不完全相同（排除匀速/瞬间拖动的机器行为）
func humanLikeTrack(track []int, end int) bool {
	n := len(track)
	if n < minTrackPts {
		return false
	}
	if track[0] > 30 {
		return false
	}
	if track[n-1] < end-30 {
		return false
	}
	hasChange := false
	for i := 2; i < n; i++ {
		if track[i]-track[i-1] != track[1]-track[0] {
			hasChange = true
			break
		}
	}
	return hasChange
}

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
