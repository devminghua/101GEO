package sqlmap

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

/* ================================================================
 * sqlmap REST API 客户端（sqlmapapi.py -s 服务）
 *  - 端点：http://127.0.0.1:8775（容器内）
 *  - 任务制：new → set options → start → status/data/log → delete
 * ================================================================ */

const apiBase = "http://127.0.0.1:8775"

var httpc = &http.Client{Timeout: 30 * time.Second}

// NewTask 创建扫描任务，返回 taskid
func NewTask() (string, error) {
	return call("GET", "/task/new", "")
}

// DeleteTask 删除任务（含其日志与结果）
func DeleteTask(taskID string) error {
	_, err := call("GET", "/task/"+taskID+"/delete", "")
	return err
}

// GetOptions 获取该任务当前全部选项（sqlmap 原生全量选项，前端「全部功能」据此渲染）
func GetOptions(taskID string) (map[string]interface{}, error) {
	res, err := callRaw("GET", "/option/"+taskID+"/list")
	if err != nil {
		return nil, err
	}
	return res, nil
}

// SetOptions 设置任务选项（map：选项名 → 值）
func SetOptions(taskID string, opts map[string]interface{}) error {
	body, _ := json.Marshal(opts)
	_, err := call("POST", "/option/"+taskID+"/set", string(body))
	return err
}

// Start 启动扫描
func Start(taskID string, url string) error {
	body, _ := json.Marshal(map[string]string{"url": url})
	_, err := call("POST", "/scan/"+taskID+"/start", string(body))
	return err
}

// Status 查询扫描状态（running / terminated / not running）
func Status(taskID string) (string, error) {
	return call("GET", "/scan/"+taskID+"/status", "")
}

// Data 获取扫描结果数据
func Data(taskID string) (map[string]interface{}, error) {
	return callRaw("GET", "/scan/"+taskID+"/data")
}

// Log 获取扫描日志
func Log(taskID string) (map[string]interface{}, error) {
	return callRaw("GET", "/scan/"+taskID+"/log")
}

// call 调用 API，返回 success 字段
func call(method, path, body string) (string, error) {
	req, err := http.NewRequest(method, apiBase+path, bytes.NewReader([]byte(body)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpc.Do(req)
	if err != nil {
		return "", fmt.Errorf("sqlmapapi 不可用：%v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out struct {
		Success bool   `json:"success"`
		TaskID  string `json:"taskid"`
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("sqlmapapi 响应解析失败：%v", err)
	}
	if !out.Success {
		return "", fmt.Errorf("sqlmapapi 错误：%s", out.Message)
	}
	if out.TaskID != "" {
		return out.TaskID, nil
	}
	return out.Status, nil
}

// callRaw 返回完整 JSON 对象
func callRaw(method, path string) (map[string]interface{}, error) {
	req, err := http.NewRequest(method, apiBase+path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sqlmapapi 不可用：%v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("sqlmapapi 响应解析失败：%v", err)
	}
	if ok, _ := out["success"].(bool); !ok {
		msg, _ := out["message"].(string)
		return nil, fmt.Errorf("sqlmapapi 错误：%s", msg)
	}
	return out, nil
}
