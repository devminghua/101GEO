#!/bin/bash
# =============================================================================
# LinkGeo 一键部署/更新脚本（在本地 Mac 上运行）
#
# 首次部署 和 版本更新 都用它，一条命令搞定。
# 数据（数据库 + 上传文件）存在服务器磁盘，脚本每次更新前自动备份旧数据库，数据永不丢失。
#
# 用法：
#   ./deploy.sh <服务器IP>            # 服务器是 x86_64 架构（绝大多数云服务器）
#   ./deploy.sh <服务器IP> arm64      # 服务器是 ARM 架构（如阿里云倚天/腾讯云ARM）
#   ./deploy.sh <服务器IP> amd64 root # 指定架构 + 用户名
#
# 前提：
#   1. 本地能 ssh 登录服务器（root 或有 sudo 权限的用户）
#   2. 本地有 Go 环境（用于交叉编译 Linux 二进制）
# =============================================================================
set -euo pipefail

SERVER="${1:?用法: ./deploy.sh <服务器IP> [amd64|arm64] [用户名]}"
ARCH="${2:-amd64}"
SSH_USER="${3:-root}"
REMOTE_DIR="/opt/linkgeo"
APP="geotool"

# 跳到项目根目录（scripts/ 的上一级）
cd "$(dirname "$0")/.."

# 从 docker-compose.yml 自动提取密钥（保证与本地/历史数据一致，enc:v1 可解密）
JWT_SECRET=$(grep 'GEO_JWT_SECRET:' docker-compose.yml | sed 's/.*: "\(.*\)"/\1/')
SECRET_KEY=$(grep 'GEO_SECRET_KEY:' docker-compose.yml | sed 's/.*: "\(.*\)"/\1/')

echo "==> [1/4] 编译 Linux ${ARCH} 二进制..."
cd backend
CGO_ENABLED=0 GOOS=linux GOARCH=${ARCH} go build -ldflags="-s -w" -o /tmp/${APP}-linux .
cd ..

echo "==> [2/4] 上传二进制到服务器..."
scp /tmp/${APP}-linux "${SSH_USER}@${SERVER}:/tmp/${APP}.new"

echo "==> [3/4] 远程部署（首次自动初始化 / 更新自动备份旧库）..."
ssh "${SSH_USER}@${SERVER}" "bash -s" <<REMOTE
set -e
# 首次部署：建目录
if [ ! -d ${REMOTE_DIR} ]; then
  mkdir -p ${REMOTE_DIR}/backups ${REMOTE_DIR}/uploads
  echo "  [首次部署] 创建目录 ${REMOTE_DIR}"
fi
cd ${REMOTE_DIR}

# 更新：备份旧数据库（数据永不丢失的关键）
if [ -f ${APP}.db ]; then
  cp ${APP}.db backups/${APP}-\$(date +%Y%m%d-%H%M%S).db
  echo "  [备份] 旧数据库已备份到 backups/"
fi

# 替换二进制
mv /tmp/${APP}.new ${APP}
chmod +x ${APP}

# 首次部署：生成 .env（含密钥，与本地一致）
if [ ! -f .env ]; then
  cat > .env <<ENV
GEO_PORT=8080
GEO_DB_PATH=${APP}.db
GEO_CRON_ENABLED=true
GEO_CRON_MINUTES=60
GEO_JWT_SECRET=${JWT_SECRET}
GEO_SECRET_KEY=${SECRET_KEY}
ENV
  echo "  [首次部署] 已生成 .env"
fi

# 首次部署：安装 systemd 服务（开机自启 + 崩溃自动重启）
if [ ! -f /etc/systemd/system/linkgeo.service ] && command -v systemctl >/dev/null 2>&1; then
  cat > /etc/systemd/system/linkgeo.service <<SVC
[Unit]
Description=LinkGeo GEO Platform
After=network.target

[Service]
Type=simple
WorkingDirectory=${REMOTE_DIR}
ExecStart=${REMOTE_DIR}/${APP}
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
SVC
  systemctl daemon-reload
  systemctl enable linkgeo
  echo "  [首次部署] 已安装 systemd 服务（开机自启 + 崩溃重启）"
fi

# 启动/重启
if [ -f /etc/systemd/system/linkgeo.service ]; then
  systemctl restart linkgeo
  echo "  [重启] systemd 服务已重启"
else
  pkill -f "${REMOTE_DIR}/${APP}" 2>/dev/null || true
  nohup ${REMOTE_DIR}/${APP} >> ${REMOTE_DIR}/run.log 2>&1 &
  echo "  [启动] nohup 后台启动（PID \$!）"
fi
REMOTE

echo "==> [4/4] 验证..."
sleep 2
echo -n "  服务响应: "
ssh "${SSH_USER}@${SERVER}" "curl -s --max-time 5 http://localhost:8080/api/system/info | head -c 120" && echo ""

echo ""
echo "✅ 部署完成！访问 http://${SERVER}:8080"
echo "   登录：admin / ymh@0921tT"
