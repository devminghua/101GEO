package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"geo-tool/database"
	"geo-tool/models"
	"geo-tool/services/geo"
)

// RunTask 启动一次全量巡检（当前租户）
func RunTask(c *gin.Context) {
	tid := TenantID(c)
	var req struct {
		Mode string `json:"mode"`
	}
	_ = c.ShouldBindJSON(&req)
	mode := req.Mode
	if mode == "" {
		mode = "manual"
	}
	if ok := geo.RunTenantTask(tid, mode); !ok {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "已有巡检任务运行中，请稍后再试"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "巡检任务已启动"})
}

// ListTasks 任务列表（当前租户）
func ListTasks(c *gin.Context) {
	tid := TenantID(c)
	var list []models.CheckTask
	database.DB.Where("tenant_id = ?", tid).Order("id desc").Limit(200).Find(&list)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": list})
}

// TaskDetail 任务详情（含结果明细）
func TaskDetail(c *gin.Context) {
	tid := TenantID(c)
	id, _ := strconv.Atoi(c.Param("id"))
	var task models.CheckTask
	if err := database.DB.Where("id = ? AND tenant_id = ?", id, tid).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "任务不存在"})
		return
	}
	var results []models.CheckResult
	database.DB.Where("task_id = ? AND tenant_id = ?", task.ID, tid).Order("id asc").Find(&results)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"task": task, "results": results}})
}
