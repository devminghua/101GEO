#!/bin/bash
# LinkGeo 一键启动（本地直跑，不用 Docker）
# 双击本文件即可启动服务并自动打开浏览器
cd "$(dirname "$0")"

# 已在运行则直接开浏览器（--noproxy 绕过系统代理，避免误判）
if curl -s --noproxy '*' --max-time 2 http://127.0.0.1:8080/api/system/info >/dev/null 2>&1; then
  echo "LinkGeo 已在运行，直接打开浏览器"
  open http://localhost:8080
  exit 0
fi

# 后台启动后端（连接容器内 PostgreSQL，需先 docker compose up -d）
GEO_DB_DRIVER=postgres \
GEO_DB_DSN="host=127.0.0.1 port=15432 user=postgres password=a7216759faa124715f53da3f1cc88b547e1d03a2d28fffd2 dbname=geotool sslmode=disable" \
GEO_UPLOADS_DIR="$(pwd)/uploads" \
GEO_PORT=8080 \
GEO_JWT_SECRET=f576bc1106dd00fa95909deafe599ada9dcfeef51fc9663bf8ef8f792540091e \
GEO_SECRET_KEY=dfb747ea472e22c831b1c65c38af0fd752d8df8e05a97bad1ddb910602ba8263 \
nohup "$(pwd)/bin/geotool" > "$(pwd)/run.log" 2>&1 &

# 等待就绪
for i in $(seq 1 15); do
  curl -s --noproxy '*' --max-time 1 http://127.0.0.1:8080/api/system/info >/dev/null 2>&1 && break
  sleep 1
done

# 打开浏览器
open http://localhost:8080

echo ""
echo "✅ LinkGeo 已启动"
echo "   访问地址：http://localhost:8080"
echo "   登录账号：admin / ymh@0921tT"
echo "   停止服务：双击「停止LinkGeo.command」"
