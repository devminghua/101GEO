package sms

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// AliyunSender 阿里云短信发送器（dysmsapi SendSms，RPC 风格 + HMAC-SHA1 签名）。
// 无第三方 SDK 依赖，直接拼装签名请求，便于无 CGO 交叉编译。
type AliyunSender struct {
	AccessKeyID     string
	AccessKeySecret string
	SignName        string
	TemplateCode    string
}

// Send 调用阿里云短信 API 发送验证码。
func (s *AliyunSender) Send(phone, code string) error {
	biz := map[string]string{
		"PhoneNumbers":  phone,
		"SignName":      s.SignName,
		"TemplateCode":  s.TemplateCode,
		"TemplateParam": fmt.Sprintf(`{"code":"%s"}`, code),
	}
	return callAliyun(s.AccessKeyID, s.AccessKeySecret, "SendSms", "2017-05-25", biz)
}

// callAliyun 阿里云 RPC 通用调用（GET + HMAC-SHA1 签名，V1.0 规范）。
func callAliyun(akID, akSecret, action, version string, biz map[string]string) error {
	body, err := callAliyunRaw(akID, akSecret, action, version, biz)
	if err != nil {
		return err
	}
	var r struct {
		Code      string `json:"Code"`
		Message   string `json:"Message"`
		RequestID string `json:"RequestId"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return fmt.Errorf("短信网关响应异常: %s", string(body))
	}
	if r.Code != "OK" {
		return fmt.Errorf("短信发送失败: %s (%s)", r.Message, r.Code)
	}
	return nil
}

// callAliyunRaw 返回原始响应体（诊断/查询用）。
func callAliyunRaw(akID, akSecret, action, version string, biz map[string]string) ([]byte, error) {
	params := map[string]string{
		"Action":           action,
		"Version":          version,
		"Format":           "JSON",
		"RegionId":         "cn-hangzhou",
		"AccessKeyId":      akID,
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureVersion": "1.0",
		"SignatureNonce":   nonce(),
		"Timestamp":        time.Now().UTC().Format("2006-01-02T15:04:05Z"),
	}
	for k, v := range biz {
		params[k] = v
	}

	// 1) 规范化查询串：key 字典序 + percentEncode
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	canonical := ""
	for i, k := range keys {
		if i > 0 {
			canonical += "&"
		}
		canonical += percentEncode(k) + "=" + percentEncode(params[k])
	}

	// 2) 签名：HMAC-SHA1(secret&, "GET&%2F&"+percentEncode(canonical))
	stringToSign := "GET&%2F&" + percentEncode(canonical)
	mac := hmac.New(sha1.New, []byte(akSecret+"&"))
	mac.Write([]byte(stringToSign))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	// 3) 请求
	reqURL := "https://dysmsapi.aliyuncs.com/?" + canonical + "&Signature=" + percentEncode(sig)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("短信网关请求失败: %w", err)
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// QueryTemplateContent 查询短信模板内容（诊断用），返回模板详情 JSON。
func (s *AliyunSender) QueryTemplateContent() (string, error) {
	biz := map[string]string{"TemplateCode": s.TemplateCode}
	body, err := callAliyunRaw(s.AccessKeyID, s.AccessKeySecret, "QuerySmsTemplate", "2017-05-25", biz)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// percentEncode 阿里云签名专用 URL 编码：A-Z a-z 0-9 - _ . ~ 不编码，其余 → %XX（大写）。
func percentEncode(s string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.' || c == '~' {
			b.WriteByte(c)
		} else {
			b.WriteByte('%')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&0x0F])
		}
	}
	return b.String()
}

// nonce 生成签名随机数（UUID v4 风格）。
func nonce() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
