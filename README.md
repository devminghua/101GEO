---
AIGC:
    Label: "1"
    ContentProducer: 001191440300708461136T1XGW3
    ProduceID: a5a8749cde710ecb596bb42dd5a200ac_4663aa28a1fa11f192a2525400287e28
    ReservedCode1: qhKbqtdun4dXiHGJOzwTwQwkNa4r6fimjgC3E7l/1aMaGjMhCNtNFAJdytghtooYJN+df6nqiJeN0GdRwg2Mk3AYfpfSuWsnjLexFoX4eRv/D95TqeITSY74Neuu0qc//ACvUrSWJ/vkWBHz08NRAm/Fe6QF20BHNu39bbJpamIY5xa8DSaCLnDBALo=
    ContentPropagator: 001191440300708461136T1XGW3
    PropagateID: a5a8749cde710ecb596bb42dd5a200ac_4663aa28a1fa11f192a2525400287e28
    ReservedCode2: qhKbqtdun4dXiHGJOzwTwQwkNa4r6fimjgC3E7l/1aMaGjMhCNtNFAJdytghtooYJN+df6nqiJeN0GdRwg2Mk3AYfpfSuWsnjLexFoX4eRv/D95TqeITSY74Neuu0qc//ACvUrSWJ/vkWBHz08NRAm/Fe6QF20BHNu39bbJpamIY5xa8DSaCLnDBALo=
---

# LinkGeo · 生成式引擎优化平台（Generative Engine Optimization）

监控你的品牌在各类 AI 平台生成式回答中的被提及情况，衡量 GEO 优化效果。

- 后端：Go + Gin + GORM + SQLite（单文件数据库，零配置）
- 前端：React + Arco Design（artd.pro 同源组件库）+ ECharts
- AI 接入：基于 **OpenAI 兼容协议**统一适配，支持市面上所有主流 AI 平台

## 功能一览

| 模块 | 说明 |
|---|---|
| 仪表盘 | 今日查询量 / 品牌出现率 / 命中平台数 / 累计出现率、近 30 天趋势、各平台命中率、监控问题命中率排行、最近任务 |
| 关键词监控 | 监控问题 CRUD、品牌词配置、分类、启用控制、批量导入 |
| AI 平台 | 内置 10+ 平台模板（OpenAI/DeepSeek/Kimi/通义/智谱/豆包/混元/Ollama…），支持任意 OpenAI 兼容服务、连接测试 |
| 巡检任务 | 一键全量巡检、任务状态跟踪、命中/未命中/错误分 Tab 详情 |
| 查询结果 | 全量明细、按平台/结果/关键词过滤 |
| 系统设置 | 默认品牌词、定时自动巡检（默认 60 分钟一次） |
| GEO 报告 | 一键生成近 7/14/30 天 Markdown 周报（含各平台表现与优化建议） |

## 目录结构

```
geo-tool/
├── backend/               # Go 后端
│   ├── main.go            # 入口：路由 / CORS / 定时巡检调度
│   ├── config/            # 环境配置（.env）
│   ├── models/            # 数据模型
│   ├── database/          # SQLite 初始化 + 种子数据
│   ├── services/ai/       # OpenAI 兼容统一客户端
│   ├── services/geo/      # 巡检执行引擎（并发查询 + 品牌词命中检测）
│   └── handlers/          # REST API
├── frontend/              # React + Arco Design 前端
│   └── src/pages/         # 仪表盘/关键词/平台/任务/结果/设置
└── data/                  # 运行时 SQLite 数据（自动创建）
```

## 环境要求

- Go 1.22+
- Node.js 20.19+（推荐 pnpm 或 npm）

## 快速启动

### 方式一：命令行

**1. 启动后端（端口 8090）**

```bash
cd backend
go mod tidy
go build -o geotool .
GEO_PORT=8090 ./geotool
```

> 首次启动自动建库并写入内置平台模板与示例关键词。
> 若 8080 被占用，可用 `GEO_PORT=8090` 指定端口（前端代理默认指向 8090）。

**2. 启动前端（端口 3000）**

```bash
cd frontend
npm install        # 或 pnpm install
npm run dev
```

浏览器访问 http://localhost:3000

### 方式二：一键脚本

```bash
./start.sh
```

脚本会依次启动后端与前端，并打印两个访问地址。

## 配置说明（backend/.env）

| 变量 | 默认值 | 说明 |
|---|---|---|
| GEO_PORT | 8090 | 后端端口 |
| GEO_DB_PATH | ../data/geo-tool.db | SQLite 数据库路径 |
| GEO_DEFAULT_BRAND | 空 | 默认品牌词（逗号分隔），前端也可在设置页维护 |
| GEO_CRON_ENABLED | true | 是否启用定时自动巡检 |
| GEO_CRON_MINUTES | 60 | 自动巡检间隔（分钟） |

## 使用流程

1. **AI 平台页**：为平台填入各自的 API Key（在各平台官网申请），点「测试」验证连通，确认无误后保持「启用」。
2. **关键词监控页**：把你要监测的提问（如"婚恋门店管理软件哪个好"）和品牌词（如"轻媒,QINGMEI"）配好。也可批量导入。
3. **仪表盘 / 巡检任务页**：点「开始巡检」，等待 1-3 分钟。
4. **查看结果**：仪表盘看整体出现率与趋势；任务详情看每条命中详情与 AI 回答摘要；系统设置里一键生成 GEO 周报。

## 接入更多 AI 平台

任意提供 `POST {base}/chat/completions`（OpenAI 兼容）的服务均可接入：

1. 「AI 平台」→「新增平台」
2. 填写 Base URL、模型名、API Key，或点击内置模板一键填充
3. 测试连接，启用

常见平台 Base URL 参考（见代码 `handlers.PlatformTemplates`）：
- DeepSeek: `https://api.deepseek.com` · `deepseek-chat`
- Kimi: `https://api.moonshot.cn/v1` · `moonshot-v1-8k`
- 通义千问: `https://dashscope.aliyuncs.com/compatible-mode/v1` · `qwen-plus`
- 智谱: `https://open.bigmodel.cn/api/paas/v4` · `glm-4-flash`
- 豆包: `https://ark.cn-beijing.volces.com/api/v3` · `doubao-pro-32k`
- OpenAI: `https://api.openai.com` · `gpt-4o-mini`
- 本地 Ollama: `http://localhost:11434/v1` · `llama3`

> 提示：部分平台需要先在控制台创建"推理接入点/Endpoint ID"后再填入 Base URL。

## API 一览

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | /api/dashboard/summary | 仪表盘总览 |
| GET | /api/dashboard/trend?days=30 | 出现率趋势 |
| GET | /api/dashboard/platforms | 各平台命中 |
| GET | /api/dashboard/keywords | 关键词命中排行 |
| GET/POST/PUT/DELETE | /api/keywords(/:id) | 关键词 CRUD |
| POST | /api/keywords/bulk | 批量导入 |
| GET/POST/PUT/DELETE | /api/platforms(/:id) | 平台 CRUD |
| GET | /api/platforms/templates | 内置模板 |
| POST | /api/platforms/:id/test | 连接测试 |
| POST | /api/tasks/run | 启动巡检 |
| GET | /api/tasks(/:id) | 任务列表/详情 |
| GET | /api/results | 查询结果 |
| GET | /api/report?days=7 | 生成报告 |
| GET/POST | /api/settings | 系统设置 |
*（内容由AI生成，仅供参考）*
