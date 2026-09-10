// 邮箱验证码发送：SMTP 真实发送 + Mock 兜底。
// 与短信验证码共用验证码存储逻辑（Service 通用，key=邮箱地址）。
package sms

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/smtp"
	"strings"
)

// EmailSender SMTP 邮箱发送器：465 端口走 SSL，其他端口（25/587）走 STARTTLS。
type EmailSender struct {
	Host string
	Port int
	User string
	Pass string
	From string // 发件人地址（须与 User 一致或获授权）
}

// Send 发送验证码邮件。
func (e EmailSender) Send(to, code string) error {
	subject := "【LinkGeo】注册验证码"
	// 正文用纯文本（兼容所有客户端，防被拦截为垃圾邮件）
	body := fmt.Sprintf("您的注册验证码是：%s（5 分钟内有效）。请勿向任何人泄露。\r\n\r\n—— LinkGeo 生成式引擎优化平台", code)
	msg := buildMail(e.From, to, subject, body)

	addr := fmt.Sprintf("%s:%d", e.Host, e.Port)
	if e.Port == 465 {
		// SSL 直连
		conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: e.Host, MinVersion: tls.VersionTLS12})
		if err != nil {
			return fmt.Errorf("SSL 连接失败: %w", err)
		}
		defer conn.Close()
		client, err := smtp.NewClient(conn, e.Host)
		if err != nil {
			return err
		}
		defer client.Close()
		if e.User != "" && e.Pass != "" {
			if err := client.Auth(smtp.PlainAuth("", e.User, e.Pass, e.Host)); err != nil {
				return fmt.Errorf("SMTP 认证失败: %w", err)
			}
		}
		return sendViaClient(client, e.From, to, msg)
	}
	// STARTTLS（25/587）
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}
	client, err := smtp.NewClient(conn, e.Host)
	if err != nil {
		conn.Close()
		return err
	}
	defer client.Close()
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: e.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("STARTTLS 失败: %w", err)
		}
	}
	if e.User != "" && e.Pass != "" {
		if err := client.Auth(smtp.PlainAuth("", e.User, e.Pass, e.Host)); err != nil {
			return fmt.Errorf("SMTP 认证失败: %w", err)
		}
	}
	return sendViaClient(client, e.From, to, msg)
}

func sendViaClient(c *smtp.Client, from, to, msg string) error {
	if err := c.Mail(from); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write([]byte(msg)); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

// buildMail 组装 RFC 5322 邮件（UTF-8 主题编码，纯文本正文）。
func buildMail(from, to, subject, body string) string {
	var b strings.Builder
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + to + "\r\n")
	b.WriteString("Subject: =?UTF-8?B?" + b64Encode(subject) + "?=\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	b.WriteString("\r\n")
	b.WriteString(body)
	return b.String()
}

func b64Encode(s string) string {
	const tbl = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var out strings.Builder
	raw := []byte(s)
	for i := 0; i < len(raw); i += 3 {
		var n uint32
		n |= uint32(raw[i]) << 16
		if i+1 < len(raw) {
			n |= uint32(raw[i+1]) << 8
		}
		if i+2 < len(raw) {
			n |= uint32(raw[i+2])
		}
		out.WriteByte(tbl[(n>>18)&0x3F])
		out.WriteByte(tbl[(n>>12)&0x3F])
		if i+1 < len(raw) {
			out.WriteByte(tbl[(n>>6)&0x3F])
		} else {
			out.WriteByte('=')
		}
		if i+2 < len(raw) {
			out.WriteByte(tbl[n&0x3F])
		} else {
			out.WriteByte('=')
		}
	}
	return out.String()
}

// MockEmailSender Mock 发送：打印日志（联调用）。
type MockEmailSender struct{}

func (MockEmailSender) Send(to, code string) error {
	log.Printf("[EMAIL][mock] 向 %s 发送验证码：%s（5 分钟内有效）", to, code)
	return nil
}

// Email 邮箱验证码服务实例（独立于短信，可单独切换发送器）。
var Email = New(MockEmailSender{})

// EmailSend 发送邮箱验证码（便捷函数）。
func EmailSend(email string) (string, error) { return Email.Send(email) }

// EmailVerify 校验邮箱验证码（便捷函数）。
func EmailVerify(email, code string) bool { return Email.Verify(email, code) }

// SetEmailSender 切换邮箱发送器（总后台配置 SMTP 后切换为真实发送）。
func SetEmailSender(sender Sender) {
	Email.mu.Lock()
	Email.sender = sender
	Email.mu.Unlock()
}

// EmailIsMock 当前邮箱发送是否 Mock。
func EmailIsMock() bool {
	Email.mu.Lock()
	defer Email.mu.Unlock()
	switch Email.sender.(type) {
	case MockSender, *MockSender, MockEmailSender, *MockEmailSender:
		return true
	default:
		return false
	}
}
