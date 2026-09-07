---
AIGC:
    Label: "1"
    ContentProducer: 001191440300708461136T1XGW3
    ProduceID: a5a8749cde710ecb596bb42dd5a200ac_a21c48c1a2b411f193c6525400f8a581
    ReservedCode1: 4mk1kOgZaI4axsWps6TJQ/VVr7GZnuY5tn7Brjnui/7Jlm2caYq+EKSWqHFLyYiI7cOFuOFuaBsDM+nUWTvl1E5E09W3p4sKOdccuA4WbrnyZ37qUINUcOpiLispE07P6VdokuD78okd6PAttdrLXxW3B+rdNoKnzwaqTPyVXhB3F1q3yu2Ln/WBXAA=
    ContentPropagator: 001191440300708461136T1XGW3
    PropagateID: a5a8749cde710ecb596bb42dd5a200ac_a21c48c1a2b411f193c6525400f8a581
    ReservedCode2: 4mk1kOgZaI4axsWps6TJQ/VVr7GZnuY5tn7Brjnui/7Jlm2caYq+EKSWqHFLyYiI7cOFuOFuaBsDM+nUWTvl1E5E09W3p4sKOdccuA4WbrnyZ37qUINUcOpiLispE07P6VdokuD78okd6PAttdrLXxW3B+rdNoKnzwaqTPyVXhB3F1q3yu2Ln/WBXAA=
---

# LinkGeo · 服务器部署指南

> 架构：单二进制后端（Go + Gin + 纯 Go SQLite，零外部依赖）+ 前端静态资源由后端托管。
> 无数据库、无 Redis、无 Node 运行时，部署极简。

---

## 1. 部署产物清单

上传到服务器只需 4 样东西：

| 文件/目录 | 来源 | 说明 |
|---|---|---|
| `geotool-linux-amd64` 或 `geotool-linux-arm64` | 本机已交叉编译好 | Linux 可执行文件，静态链接，20~21MB |
| `frontend/dist/` | 本机 `npm run build` 产物 | 前端静态资源（index.html + assets/） |
| `backend/.env`（可选） | 自建 | 运行配置（端口/密钥/巡检间隔） |
| `geo-tool.db`（可选） | 本机 `backend/geo-tool.db` | 已有数据（账号/平台/关键词/获客数据） |

> 服务器架构判断：`uname -m` 输出 `x86_64` 用 amd64，`aarch64` 用 arm64。

---

## 2. 服务器目录结构

保持与开发环境一致的相对路径（代码里前端静态资源和 uploads 都按相对路径取）。

```
/opt/geo-tool/
├── backend/
│   ├── geotool            # ← 上传的 Linux 二进制（改名为 geotool）
│   ├── .env               # 运行配置
│   └── uploads/           # 自动创建（创作中心图片/视频 + Logo）
├── frontend/
│   └── dist/              # ← 前端构建产物
└── data/
    └── geo-tool.db        # 数据库（GEO_DB_PATH 指向）
```

创建目录：

```bash
sudo mkdir -p /opt/geo-tool/{backend,frontend/dist,data}
sudo chown -R $USER:$USER /opt/geo-tool
```

上传（本机执行）：

```bash
# 二进制（按架构二选一）
scp backend/geotool-linux-amd64  user@服务器IP:/opt/geo-tool/backend/geotool
# 前端产物
scp -r frontend/dist/*          user@服务器IP:/opt/geo-tool/frontend/dist/
# 数据（全新部署可跳过；要带已有数据再传）
scp backend/geo-tool.db         user@服务器IP:/opt/geo-tool/data/
chmod +x /opt/geo-tool/backend/geotool
```

---

## 3. 后端配置（backend/.env）

```env
GEO_PORT=8080
GEO_DB_PATH=../data/geo-tool.db
GEO_DEFAULT_BRAND=轻媒,QINGMEI
GEO_CRON_ENABLED=true
GEO_CRON_MINUTES=60
# 生产环境务必设置随机密钥（32 位以上随机串）
GEO_JWT_SECRET=<随机字符串>
GEO_SECRET_KEY=<随机字符串>
```

> 生成随机密钥：`openssl rand -hex 32`
>
> ⚠️ 若**迁移现有数据库**，`GEO_SECRET_KEY` 必须与本机开发时一致，否则已存储的密码/密钥无法解密；全新部署无此限制。

---

## 4. systemd 守护进程（推荐）

创建 `/etc/systemd/system/geotool.service`：

```ini
[Unit]
Description=GEO Tool Backend
After=network.target

[Service]
Type=simple
WorkingDirectory=/opt/geo-tool/backend
ExecStart=/opt/geo-tool/backend/geotool
Restart=always
RestartSec=3
EnvironmentFile=/opt/geo-tool/backend/.env

[Install]
WantedBy=multi-user.target
```

启动：

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now geotool
sudo systemctl status geotool        # 查看状态
sudo journalctl -u geotool -f        # 查看日志
```

---

## 5. Nginx 反向代理 + HTTPS

前端路由是 SPA，需把 `/` 指到后端根路径（后端已托管 dist），`/api` 与 `/uploads` 直接透传。

```nginx
server {
    listen 80;
    server_name geo.example.com;          # 改成你的域名

    client_max_body_size 50m;             # 创作中心上传图片/视频

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

> 静态资源由后端 `r.Static("/assets", ...)` 托管，无需单独配置 location。
> 若后端不监听 8080，改 `proxy_pass` 端口即可。

HTTPS（certbot 一键）：

```bash
sudo apt install -y certbot python3-certbot-nginx
sudo certbot --nginx -d geo.example.com
```

---

## 6. 数据说明

| 场景 | 做法 |
|---|---|
| 全新部署 | 不传 db，首次启动自动建库 + 写入内置平台模板、演示账号 |
| 迁移已有数据 | 上传本机 `backend/geo-tool.db` 到 `data/`，并保持 `GEO_SECRET_KEY` 一致 |

首次启动后登录：
- 总后台：`admin / admin123`（如被重置，用系统内置入口或重设）
- 分站演示：`demo / demo123`

> 生产上线务必修改默认密码（总后台「用户管理」里重置）。

---

## 7. 安全清单（上线前必做）

1. [ ] `.env` 设置独立 `GEO_JWT_SECRET` / `GEO_SECRET_KEY`
2. [ ] 修改默认账号 `admin/admin123`、`demo/demo123` 密码
3. [ ] Nginx 只开放 443/80，8080 端口禁止对外（仅本机回环）
4. [ ] 服务器防火墙（ufw/安全组）仅放行 80/443/22
5. [ ] 平台 API Key 是加密存储的，避免泄露 `.env` 与数据库备份

---

## 8. 日常运维

```bash
# 查看状态 / 日志
sudo systemctl status geotool
sudo journalctl -u geotool -n 100 -f

# 更新版本（覆盖二进制 + 前端 dist 后重启）
sudo systemctl restart geotool

# 备份（SQLite 单文件，直接拷贝即可）
cp /opt/geo-tool/data/geo-tool.db /opt/geo-tool/data/geo-tool.db.bak
```

---

## 9. 常见问题

| 现象 | 处理 |
|---|---|
| 启动即退出 | 检查 `journalctl -u geotool` 日志；确认 8080 未被占用（`ss -ltnp`） |
| 页面能开但接口 404/白屏 | 确认 `frontend/dist` 与后端相对路径正确（二进制必须在 `/opt/geo-tool/backend/` 下运行） |
| 上传图片后访问 404 | 确认 `uploads/` 目录可写（`chown` 给运行用户） |
| 提示端口占用 | 改 `.env` 的 `GEO_PORT`，并同步 Nginx `proxy_pass` |
| 自动巡检没触发 | 确认 `GEO_CRON_ENABLED=true`、服务器时区正确（东八区） |
*（内容由AI生成，仅供参考）*
