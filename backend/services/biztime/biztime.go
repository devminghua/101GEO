// Package biztime 提供**业务时区（北京时间）**的统一时间工具。
//
// 为什么需要这个包（2026-09-12 修复）：
// 容器镜像默认时区是 **UTC**（实测 `docker exec geotool date` 返回 UTC，
// docker-compose.yml 也未设置 TZ）。而 Go 的 time.Now() 返回的是**本地时区**时间，
// 在 UTC 容器里就等于 UTC 时间。这导致所有「按天」「按小时」的业务逻辑全部偏了 8 小时：
//
//  1. 自动巡检时段判断（main.go runAutoCheck）：本该「北京时间 8-22 点巡检」，
//     实际跑成了「北京时间 16:00 ~ 次日 06:00」——客户白天 6-15 点完全停摆，
//     深夜反而频繁巡检。实测数据库印证：自动巡检记录集中在本地 16-23 点与 0-5 点，
//     本地 6-15 点整段空白。
//  2. 每日查询配额（handlers/query_quota.go）：配额按 UTC 日切，
//     实际是**北京时间早上 8 点**重置，而给客户看的提示写的是「明天 0 点自动重置」，
//     文案与实际行为不符。
//  3. 签到 / 日期统计 / 日报周期：同样按 UTC 日切，北京时间 0-8 点的操作会被算进前一天。
//
// 统一收口到本包，全站任何「按业务日期/小时」的逻辑一律使用这里的函数，
// 不要再直接调用 time.Now()（延续项目「口径唯一」的工程原则）。
package biztime

import "time"

// zone 北京时间（东八区）。
//
// 用 FixedZone 而不是 LoadLocation("Asia/Shanghai")：
//   - 中国自 1991 年起已无夏令时，UTC+8 恒定，FixedZone 语义完全正确；
//   - 不依赖容器/系统是否装了 tzdata，也不受 TZ 环境变量影响，
//     服务器版（Linux 容器）与单机版（Windows/macOS 客户机）行为完全一致。
var zone = time.FixedZone("CST", 8*3600)

// Now 返回当前北京时间
func Now() time.Time {
	return time.Now().In(zone)
}

// Hour 返回当前北京时间的小时（0-23），用于业务时段判断
func Hour() int {
	return Now().Hour()
}

// Today 返回当前业务日期，格式 2006-01-02。用于每日配额、签到等「按天」逻辑
func Today() string {
	return Now().Format("2006-01-02")
}

// Month 返回当前业务月份，格式 2006-01。用于月度统计
func Month() string {
	return Now().Format("2006-01")
}

// MonthCompact 返回当前业务月份，格式 200601。用于需要紧凑格式的月度统计
func MonthCompact() string {
	return Now().Format("200601")
}

// DateCompact 返回当前业务日期，格式 20060102。用于流水号/文件名
func DateCompact() string {
	return Now().Format("20060102")
}

// Format 按业务时区格式化指定时间。
// 用于把任意时区的 time.Time（例如来自数据库的 timestamptz）转成北京时间的文本。
func Format(t time.Time, layout string) string {
	return t.In(zone).Format(layout)
}

// DateTime 返回当前业务时间的 2006-01-02 15:04 文本（报告/导出落款用）
func DateTime() string {
	return Now().Format("2006-01-02 15:04")
}

// DateTimeSec 返回当前业务时间的 2006-01-02 15:04:05 文本（报告生成时间用）
func DateTimeSec() string {
	return Now().Format("2006-01-02 15:04:05")
}

// Day 返回业务时区下「相对今天偏移 n 天」的日期，格式 2006-01-02。
//
// 这是「统计周期」「近 N 日曲线」类逻辑的统一入口：
// n=0 即今天，n=-7 即 7 天前。**不要**再写 time.Now().AddDate(0,0,n)，
// 那会按 UTC 日切，北京时间 0-8 点算出来的日期会差一天。
func Day(offset int) string {
	return Now().AddDate(0, 0, offset).Format("2006-01-02")
}

// Since 返回业务时区下 n 天前的时刻（用于把统计下界交给数据库比较）。
// 统计周期上界请用 Now()，保证上下界同处一个时区。
func Since(days int) time.Time {
	return Now().AddDate(0, 0, -days)
}

// DayStart 返回指定时刻所在**业务日**的零点（北京时间）。
// 用于「次日活跃时段开始」这类需要落到业务日边界的计算。
func DayStart(t time.Time) time.Time {
	y, m, d := t.In(zone).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, zone)
}

// In 把任意时间转换到业务时区，用于需要保留 time.Time 类型的场景
func In(t time.Time) time.Time {
	return t.In(zone)
}

// Zone 返回业务时区，供 time.Date(...) 等需要显式时区的调用使用
func Zone() *time.Location {
	return zone
}
