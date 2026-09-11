package config

import (
	"crypto/rand"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/joho/godotenv"
)

// Version 产品版本号：每次更新记录一次版本号（老板规则，2026-09-07 起）。
// 当前 1.0.1。发版时改这里，客户端与 SaaS 端登录页/侧栏会自动显示。
const Version = "1.0.28"

type Config struct {
	Port         string // 后端监听端口
	DBPath       string // （已弃用，保留字段兼容）数据库文件路径
	DBDriver     string // 数据库驱动：固定 postgres
	DBDSN        string // PostgreSQL DSN（必填）
	DataDir      string // 数据根目录（数据库 + uploads 等，与工作目录解耦）
	UploadsDir   string // 上传目录（Logo/头像/创作图片等）
	DefaultBrand string // 默认品牌词
	CronEnabled  bool   // 是否启用定时自动巡检
	CronMinutes  int    // 自动巡检间隔（分钟）
	JWTSecret    string // JWT 签名密钥
	SecretKey    string // 密码/敏感字段可逆加密密钥
	WxAppID      string // 微信小程序 AppID（miniapp 登录用）
	WxSecret     string // 微信小程序 Secret（code2session 用）
	LicenseMode  bool   // 是否启用卡密授权（GEO_LICENSE_MODE=on，仅 Windows 单机版）
	LicensePubKey string // 卡密验签公钥（base64，客户端内置；空则禁用卡密）
}

// fallbackTokenSecret 未配置 GEO_JWT_SECRET 时的兜底：每次启动随机生成，
// 避免固定弱密钥被攻击者伪造任意身份 JWT（安全审计修复：原为硬编码 "geo-tool-dev-secret"）。
// 代价：重启后旧登录态失效；生产环境必须显式配置 GEO_JWT_SECRET。
var fallbackTokenSecret = func() []byte {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err == nil {
		log.Printf("[config] ⚠️ GEO_JWT_SECRET 未配置：使用随机临时密钥（重启后登录态失效），生产环境必须配置 GEO_JWT_SECRET")
		return b
	}
	return []byte("geo-tool-dev-secret")
}()

func (c *Config) TokenSecret() []byte {
	if c.JWTSecret == "" {
		return fallbackTokenSecret
	}
	return []byte(c.JWTSecret)
}

// PayloadSecret 返回可逆加密密钥（AES 派生），用于密码加密存储。
func (c *Config) PayloadSecret() []byte {
	if c.SecretKey == "" {
		return []byte("geo-tool-dev-payload-secret")
	}
	return []byte(c.SecretKey)
}

// Load 返回全局配置（sync.Once 缓存，避免每次调用重复读 .env + 打印日志）。
func Load() *Config {
	once.Do(func() { cfg = loadConfig() })
	return cfg
}

var (
	once sync.Once
	cfg  *Config
)

func loadConfig() *Config {
	// 先加载工作目录下的 .env，再尝试加载可执行文件同目录的 .env（桌面安装场景：
	// 快捷方式的工作目录可能不是 exe 所在目录，二次加载兜底保证 exe 旁 .env 始终生效）。
	_ = godotenv.Load()
	if exe, err := os.Executable(); err == nil {
		_ = godotenv.Load(filepath.Join(filepath.Dir(exe), ".env"))
	}

	// 数据根目录：桌面安装版（尤其 Windows）写用户可写目录，避免 Program Files 只读 + 工作目录漂移导致崩溃。
	// 非 Windows（容器 / 开发）默认 "."，即保持原有「相对当前目录」行为，兼容 docker bind mount。
	dataDir := expandHome(getEnv("GEO_DATA_DIR", ""))
	if dataDir == "" {
		dataDir = defaultDataDir()
	}
	dbPath := expandHome(getEnv("GEO_DB_PATH", ""))
	if dbPath == "" {
		dbPath = filepath.Join(dataDir, "geo-tool.db")
	}
	uploadsDir := expandHome(getEnv("GEO_UPLOADS_DIR", ""))
	if uploadsDir == "" {
		uploadsDir = filepath.Join(dataDir, "uploads")
	}

	c := &Config{
		Port:         getEnv("GEO_PORT", "8080"),
		DBPath:       dbPath,
		DBDriver:     strings.ToLower(getEnv("GEO_DB_DRIVER", "postgres")),
		DBDSN:        getEnv("GEO_DB_DSN", ""),
		DataDir:      dataDir,
		UploadsDir:   uploadsDir,
		DefaultBrand: getEnv("GEO_DEFAULT_BRAND", ""),
		CronEnabled:  getEnvBool("GEO_CRON_ENABLED", true),
		CronMinutes:  getEnvInt("GEO_CRON_MINUTES", 60),
		JWTSecret:    getEnv("GEO_JWT_SECRET", ""),
		SecretKey:    getEnv("GEO_SECRET_KEY", ""),
		WxAppID:      getEnv("GEO_WX_APPID", ""),
		WxSecret:     getEnv("GEO_WX_SECRET", ""),
		LicenseMode:  getEnvBool("GEO_LICENSE_MODE", false),
		LicensePubKey: getEnv("GEO_LICENSE_PUBLIC_KEY", ""),
	}

	// 确保数据根目录存在（Windows 安装版首次运行即建 %LOCALAPPDATA%\LinkGeo）
	if dataDir != "" && dataDir != "." {
		if err := os.MkdirAll(dataDir, 0o755); err != nil {
			log.Printf("[config] 创建数据目录失败: %v", err)
		}
	}

	log.Printf("[config] port=%s db=%s uploads=%s cronEnabled=%v cronMinutes=%d jwtMode=%s", c.Port, c.DBDriver, c.UploadsDir, c.CronEnabled, c.CronMinutes, map[bool]string{true: "custom", false: "default"}[c.JWTSecret != ""])

	// 安全校验：服务器/容器模式（非单机版）禁止使用默认弱密钥，防止弱密钥上线被伪造 JWT / 解密敏感数据。
	// 单机版（GEO_LICENSE_MODE=true）允许默认，但打包时 .env 已注入强随机密钥。
	if c.JWTSecret == "" || c.SecretKey == "" {
		if c.LicenseMode {
			log.Printf("[config][WARN] 未配置强密钥，使用默认值（仅单机版允许，请确认 .env 已注入强随机密钥）")
		} else {
			log.Fatalf("[config] 未配置 GEO_JWT_SECRET / GEO_SECRET_KEY，服务器模式禁止使用默认弱密钥，请在 .env 或 docker-compose.yml 中配置强密钥后重启")
		}
	}
	return c
}

// defaultDataDir 返回默认数据根目录。Windows 用 %LOCALAPPDATA%\LinkGeo（每用户可写、与安装目录解耦），
// 其它平台返回 "."（保持相对当前目录，兼容既有容器/开发部署）。
func defaultDataDir() string {
	if runtime.GOOS == "windows" {
		if base := os.Getenv("LOCALAPPDATA"); base != "" {
			return filepath.Join(base, "LinkGeo")
		}
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, ".linkgeo")
		}
	}
	return "."
}

// expandHome 展开路径开头的 ~（仅 ~ 或 ~/ 前缀），供 .env 写 ~/Library/Application Support/... 等。
func expandHome(path string) string {
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
		return path
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func getEnvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return i
}
