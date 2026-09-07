package handlers

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"geo-tool/database"
	"geo-tool/models"
)

// OnlineTracker 在线心跳表（纯内存，不落库）。
// 每个请求经鉴权中间件顺带记录心跳（O(1)），统计时惰性清理过期项，零数据库查询。
type OnlineTracker struct {
	mu sync.RWMutex
	m  map[uint]time.Time // tenantID -> 最后活跃时间
}

var onlineTracker = &OnlineTracker{m: make(map[uint]time.Time)}

// 在线窗口：最近 N 分钟内有请求即视为在线
const onlineWindow = 5 * time.Minute

// Touch 记录某租户的心跳
func (t *OnlineTracker) Touch(tenantID uint) {
	if tenantID == 0 {
		return
	}
	t.mu.Lock()
	t.m[tenantID] = time.Now()
	t.mu.Unlock()
}

// onlineIDs 统计在线租户 ID，并顺带清理过期项
func (t *OnlineTracker) onlineIDs() []uint {
	cutoff := time.Now().Add(-onlineWindow)
	out := make([]uint, 0)
	t.mu.Lock()
	for id, last := range t.m {
		if last.Before(cutoff) {
			delete(t.m, id)
		} else {
			out = append(out, id)
		}
	}
	t.mu.Unlock()
	return out
}

// SuperOnlineCount 总后台查询在线客户：GET /api/super/online-count
// 返回：在线数、启用中的客户总数、在线客户列表（id+名称）
func SuperOnlineCount(c *gin.Context) {
	ids := onlineTracker.onlineIDs()

	var total int64
	database.DB.Model(&models.Tenant{}).Where("status = ?", 1).Count(&total)

	onlineList := make([]gin.H, 0, len(ids))
	if len(ids) > 0 {
		var tenants []models.Tenant
		database.DB.Where("id IN ?", ids).Find(&tenants)
		for _, t := range tenants {
			onlineList = append(onlineList, gin.H{"id": t.ID, "name": t.Name})
		}
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"online":       len(ids),
		"total":        total,
		"online_list":  onlineList,
		"online_window": int(onlineWindow / time.Minute),
	}})
}
