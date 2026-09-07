# LinkGeo 客户效果监控小程序

给分站客户在微信里查看自己品牌 GEO 优化效果的小程序（Taro + React）。

## 当前进度

- ✅ 微信登录 / 分站账号绑定
- ✅ 效果总览页（六项指标 + 近 7 天趋势 + 品牌声量 + 未读角标）
- ✅ 竞品对标（竞品声量排行 + 首推次数，复用 /geo/gaps）
- ✅ 效果归因（优化前后对比 + 逐问题变化，复用 /geo/compare）
- ✅ 消息中心（站内信列表 + 已读 + 全部已读，复用 /notifications）
- ⏳ 阶段 3：Token 余额 + 小程序支付 + 订阅消息

底部 TabBar 4 个 tab：总览 / 竞品 / 归因 / 消息。

## 目录结构

```
miniapp/
├── src/
│   ├── app.tsx / app.config.ts / app.scss
│   ├── api.ts              # 请求封装（登录/绑定/总览）
│   └── pages/
│       ├── login/          # 登录 / 绑定页
│       └── home/           # 效果总览页
├── config/                 # Taro 构建配置
├── project.config.json     # 微信开发者工具配置
└── dist/                   # 构建产物（导入微信开发者工具用）
```

## 后端接口（已实现）

| 接口 | 说明 |
|---|---|
| `POST /api/miniapp/login` | 微信登录：code → 已绑定返回 token，未绑定返回 need_bind |
| `POST /api/miniapp/bind` | 绑定：code + 分站账号密码 → 绑定 → token |
| `GET /api/miniapp/home` | 效果总览聚合（六项指标 + 趋势 + 未读） |

后端环境变量（docker-compose / .env）：

```
GEO_WX_APPID=wx...       # 微信小程序 AppID
GEO_WX_SECRET=...        # 微信小程序 Secret
```

> 未配置 AppID 时后端进入「测试模式」：用 code 生成测试 openid（`wx-test-<code>`），方便本地联调。

## 本地开发

```bash
cd miniapp
npm install
npm run dev:weapp   # 监听构建，产物在 dist/
```

然后用微信开发者工具「导入项目」，选择 `miniapp/` 目录（miniprogramRoot 已指向 dist/）。

> 本地开发后端地址是 `http://127.0.0.1:8080`（见 `src/api.ts` 的 BASE_URL），
> 微信开发者工具需勾选「不校验合法域名」（详情 → 本地设置）。

## 上线前必改

1. `src/api.ts` 的 `BASE_URL` 改成 `https://你的域名`。
2. 后端配 `GEO_WX_APPID` + `GEO_WX_SECRET`（去掉测试模式）。
3. `project.config.json` 的 `appid` 改成真实 AppID。
4. 后端必须 HTTPS（小程序强制）。
