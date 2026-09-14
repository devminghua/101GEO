package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"geo-tool/services/portscan"
	svcsqlmap "geo-tool/services/sqlmap"
)

/* ================================================================
 * 站点体检 · SQL 注入检测（sqlmap 图形界面）
 *  - 代理封装 sqlmap REST API（容器内 127.0.0.1:8775）
 *  - 安全约束：目标仅限域名（复用 portscan.ResolveHost 的 SSRF 防护）、
 *    每日 3 次独立配额、全任务留痕
 *  - 合规提示：前端必须勾选「仅对有权测试的站点使用」才可发起
 * ================================================================ */

const quotaModuleSqlmap = "sqlmap"
const sqlmapDailyLimit = 3

// sqlmap 任务状态（内存缓存 taskid → tenant，用于隔离校验）
var sqlmapTasks = map[string]sqlmapTaskMeta{}

type sqlmapTaskMeta struct {
	TenantID uint
	Target   string
	Created  time.Time
}

// SQLMapStart POST /api/site-audit/sqlmap/start —— 创建并启动 sqlmap 扫描任务
func SQLMapStart(c *gin.Context) {
	var req struct {
		URL     string                 `json:"url"`
		Options map[string]interface{} `json:"options"` // sqlmap 选项（白名单过滤见下）
	}
	if !jsonBody(c, &req) {
		return
	}
	// 目标安全校验（域名 + 公网，防 SSRF）
	host, _, err := portscan.ResolveHost(req.URL)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "目标校验失败：" + err.Error()})
		return
	}
	// 独立配额（先校验后扣费）
	if ok, used, limit := CheckQueryQuotaModule(c, quotaModuleSqlmap); !ok {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "今日 SQL 注入检测次数已用完（" + itoa(used) + "/" + itoa(limit) + "），明天 0 点自动重置"})
		return
	}
	// 创建任务
	taskID, err := svcsqlmap.NewTask()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "sqlmap 服务不可用：" + err.Error()})
		return
	}
	// 选项白名单过滤（防注入任意参数；「全部功能」由前端高级参数走 risk/level/technique 等安全项）
	opts := sanitizeOptions(req.Options)
	if len(opts) > 0 {
		if err := svcsqlmap.SetOptions(taskID, opts); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "选项设置失败：" + err.Error()})
			return
		}
	}
	// 启动扫描
	if err := svcsqlmap.Start(taskID, host); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "扫描启动失败：" + err.Error()})
		return
	}
	tid := TenantID(c)
	sqlmapTasks[taskID] = sqlmapTaskMeta{TenantID: tid, Target: host, Created: time.Now()}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"task": taskID, "target": host}})
}

// sqlmap 选项白名单（安全可控项；值类型强制转换）
var sqlmapOptWhitelist = map[string]bool{
	"technique": true, "level": true, "risk": true, "threads": true,
	"batch": true, "forms": true, "cookie": true, "userAgent": true,
	"referer": true, "proxy": true, "delay": true, "timeout": true,
	"getBanner": true, "getCurrentUser": true, "getCurrentDb": true,
	"getDbs": true, "getTables": true, "getColumns": true, "dumpTable": true,
	"getPrivileges": true, "getHostname": true, "getUsers": true,
	"osShell": false, "sqlShell": false, "fileRead": false, "fileWrite": false, // 高危：禁用
	"prefix": true, "suffix": true, "randomAgent": true, "hexConvert": true,
	"smart": true, "mobile": true, "checkTor": false,
}

// sanitizeOptions 白名单过滤 + 类型收敛（防注入高危选项）
func sanitizeOptions(in map[string]interface{}) map[string]interface{} {
	if in == nil {
		return nil
	}
	out := map[string]interface{}{}
	for k, v := range in {
		if !sqlmapOptWhitelist[k] {
			continue // 白名单外一律丢弃（含高危项）
		}
		switch k {
		case "technique", "prefix", "suffix":
			if s, ok := v.(string); ok && len(s) <= 16 {
				out[k] = s
			}
		case "cookie", "userAgent", "referer", "proxy":
			if s, ok := v.(string); ok && len(s) <= 512 {
				out[k] = s
			}
		case "dumpTable":
			if s, ok := v.(string); ok && len(s) <= 128 {
				out[k] = s
			}
		case "level", "risk", "threads", "delay", "timeout":
			if f, ok := v.(float64); ok {
				out[k] = int(f)
			}
		default: // bool 类
			if b, ok := v.(bool); ok {
				out[k] = b
			}
		}
	}
	return out
}

// SQLMapStatus GET /api/site-audit/sqlmap/status?task=xxx
func SQLMapStatus(c *gin.Context) {
	task := c.Query("task")
	if !checkTask(c, task) {
		return
	}
	status, err := svcsqlmap.Status(task)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": err.Error()})
		return
	}
	dyOK(c, gin.H{"status": status})
}

// SQLMapData GET /api/site-audit/sqlmap/data?task=xxx
func SQLMapData(c *gin.Context) {
	task := c.Query("task")
	if !checkTask(c, task) {
		return
	}
	data, err := svcsqlmap.Data(task)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": err.Error()})
		return
	}
	dyOK(c, data)
}

// SQLMapLog GET /api/site-audit/sqlmap/log?task=xxx
func SQLMapLog(c *gin.Context) {
	task := c.Query("task")
	if !checkTask(c, task) {
		return
	}
	log, err := svcsqlmap.Log(task)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": err.Error()})
		return
	}
	dyOK(c, log)
}

// SQLMapDelete DELETE /api/site-audit/sqlmap/task?task=xxx
func SQLMapDelete(c *gin.Context) {
	task := c.Query("task")
	if !checkTask(c, task) {
		return
	}
	_ = svcsqlmap.DeleteTask(task)
	delete(sqlmapTasks, task)
	dyOK(c, gin.H{"deleted": true})
}

// SQLMapOptions GET /api/site-audit/sqlmap/options —— 前端渲染的选项清单（白名单+说明）
func SQLMapOptions(c *gin.Context) {
	dyOK(c, gin.H{
		"techniques": []gin.H{
			{"key": "B", "label": "布尔盲注"},
			{"key": "E", "label": "报错注入"},
			{"key": "U", "label": "联合查询"},
			{"key": "S", "label": "堆叠注入"},
			{"key": "T", "label": "时间盲注"},
			{"key": "Q", "label": "内联查询"},
		},
		"levels":      []int{1, 2, 3, 4, 5},
		"risks":       []int{1, 2, 3},
		"max_threads": 10,
	})
}

// checkTask 任务归属校验（租户隔离）
func checkTask(c *gin.Context, task string) bool {
	if task == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "缺少任务 ID"})
		return false
	}
	meta, ok := sqlmapTasks[task]
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "任务不存在或已过期"})
		return false
	}
	if meta.TenantID != TenantID(c) {
		c.JSON(http.StatusForbidden, gin.H{"code": 1, "msg": "无权访问该任务"})
		return false
	}
	return true
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
