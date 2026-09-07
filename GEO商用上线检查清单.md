---
AIGC:
    Label: "1"
    ContentProducer: 001191440300708461136T1XGW3
    ProduceID: a5a8749cde710ecb596bb42dd5a200ac_5aa847c4a36711f193c6525400f8a581
    ReservedCode1: ZNik6xDmddH248SuGeUmITx8CIbFkgF85r60QTvPvGdf/DOW7de4tC5ppWblvxsO5OSGv1BSeKANktauYtNf8RD1vWXLqtqvhtlyCHUz629m1WQzDaXTbivrDlibB+OoybIr3lyic1fp8Ww+d8gULMWWEKwYucUQDZ5pWran/0D9uhT26ChFMGqa9Ec=
    ContentPropagator: 001191440300708461136T1XGW3
    PropagateID: a5a8749cde710ecb596bb42dd5a200ac_5aa847c4a36711f193c6525400f8a581
    ReservedCode2: ZNik6xDmddH248SuGeUmITx8CIbFkgF85r60QTvPvGdf/DOW7de4tC5ppWblvxsO5OSGv1BSeKANktauYtNf8RD1vWXLqtqvhtlyCHUz629m1WQzDaXTbivrDlibB+OoybIr3lyic1fp8Ww+d8gULMWWEKwYucUQDZ5pWran/0D9uhT26ChFMGqa9Ec=
---

# LinkGeo · 商用上线检查清单

> 系统版本：GEO 智能中心（完整版） · 生成日期：2026-08-29
> 本文档覆盖「合规 / 安全 / 运维 / 成本」四大商用硬性项，逐项打勾即为可上线状态。

## 一、合规（红线，缺一不可）

- [ ] **ICP 备案**：域名解析到国内服务器前，必须完成 ICP 备案（阿里云/腾讯云控制台提交，约 7~20 个工作日）。未备案域名禁止指向国内机房。
- [ ] **网安法 / 个保法**：产品页面底部展示隐私政策与用户协议；采集的用户/企业数据遵循最小必要原则。
- [ ] **不承诺排名**：系统任何文案、报告、销售话术禁止出现"保证 XX 天 AI 排名第一"等确定性承诺。指标一律以"出现率 / 引用率"等概率口径呈现（前端已按此口径设计）。
- [ ] **不操纵 AI / 不刷量**：禁止引用农场、批量造假链接、机器人刷问答。GEO 优化手段仅限：内容供给（事实库/软文）、结构优化（llms.txt / schema.org）、站点质量提升。系统内置的风险词库与"引用率"指标用于正向监督，不用于造假。
- [ ] **内容投放人工确认**：内容投放模块的人工环节（发布）保留在官方后台操作，系统不做自动群发、不模拟登录（已按此约束实现）。

## 二、安全（上线前必须完成）

- [ ] **设置强密钥**：生产环境必须配置 `GEO_JWT_SECRET`、`GEO_SECRET_KEY`，使用随机长字符串（如 `openssl rand -hex 32`），禁止默认值。
- [ ] **改默认密码**：admin/admin123 上线后立即修改；为每个分站创建独立账号与密码。
- [ ] **HTTPS**：域名解析后配置 SSL 证书（推荐自动续期的 Let's Encrypt 或云厂商免费证书），全站强制 HTTPS。
- [ ] **数据备份**：`geo-tool.db`（SQLite）每日自动备份，异地保留至少 7 天；`uploads/` 目录一并纳入备份。
- [ ] **限流与登录保护**：确认登录接口存在失败限流；生产环境建议加 WAF 或接入层限流，防爆破与刷接口。

## 三、部署与运维

- [ ] **服务器规格**：2C4G 起步，磁盘 40G+；SQLite 单机性能足够本系统量级，暂无需升级 PostgreSQL。
- [ ] **进程守护**：Docker 容器设置 `--restart=always`（当前 geotool 容器建议确认），或使用 systemd 守护。
- [ ] **监控告警**：关注 `geotool.log`；建议对接云监控，磁盘使用率 > 85% 告警（SQLite 需预留空间）。
- [ ] **多租户校验**：所有 GEO 智能中心接口均已按 tenant_id 隔离（事实库/竞品/风险词/引用/指标均带租户维度），上线前用两个分站账号互相验证数据不可见。
- [ ] **Docker 发布流程**（本机已就绪）：
  1. 后端改代码 → `CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o geotool .`
  2. 前端改代码 → `cd frontend && npx vite build`（产物 dist/）
  3. 部署：`docker cp dist geotool:/app/frontend/dist`（前端免重启）+ `docker cp geotool geotool:/app/backend/geotool && docker restart geotool`（后端需重启）

## 四、成本控制（防止 API 烧钱）

- [ ] **采样策略**：巡检任务建议保持"代表性问题 + 抽样平台"组合，避免全量全平台每日跑。
- [ ] **结果缓存 / 去重**：同问题同平台在缓存周期内不重复计费（系统已实现去重与增量）。
- [ ] **分级调度**：低频（每日）/ 高频（实时）分开调度；将成本高的平台（如部分付费 API）调低频率。
- [ ] **配额告警**：对接云厂商配额/账单告警，设置月度预算阈值。

## 五、数据口径说明（对外宣传用）

- **品牌出现率** = 成功回答中提及品牌的问题占比
- **推荐率 TOP3** = 品牌出现在回答前 3 位的占比
- **引用率** = 回答附带来源链接的占比
- **事实一致率** = 回答未与品牌事实库冲突的占比（事实库需人工维护）
- **竞品声量 SOV** = 回答提及竞品的占比（竞品库需人工维护）
- **风险回答率** = 回答命中风险词（过度承诺类表述）的占比

> 以上指标均为"监测口径"，用于指导内容优化，不作为对 AI 排名的承诺。
*（内容由AI生成，仅供参考）*
