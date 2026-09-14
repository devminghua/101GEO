package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"geo-tool/services/portscan"
)

/* ================================================================
 * 站点体检 · Nmap 端口扫描：POST /api/site-audit/portscan
 *  - 配额：独立 portscan 池，每日 5 次（老板 2026-09-14 需求）
 *  - 目标仅限域名（防 SSRF 校验在 portscan.ResolveHost）
 * ================================================================ */

const quotaModulePortscan = "portscan"
const portscanDailyLimit = 5

// PortScanQuotaGuard 端口扫描独立配额守卫（每日 5 次，不占用查询配额）
func PortScanQuotaGuard(c *gin.Context) bool {
	if ok, used, _ := CheckQueryQuotaModule(c, quotaModulePortscan); !ok {
		_ = used
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": fmt.Sprintf("今日端口扫描次数已用完（%d/%d），明天 0 点自动重置", portscanDailyLimit, portscanDailyLimit)})
		return false
	}
	return true
}

// SitePortScan POST /api/site-audit/portscan —— Nmap 端口扫描
func SitePortScan(c *gin.Context) {
	var req struct {
		URL string `json:"url"`
	}
	if !jsonBody(c, &req) {
		return
	}
	if !PortScanQuotaGuard(c) {
		return
	}
	start := time.Now()
	res, err := portscan.Scan(req.URL, 90*time.Second)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "端口扫描失败：" + err.Error()})
		return
	}
	_ = start
	dyOK(c, res)
}
