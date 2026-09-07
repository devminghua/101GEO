#!/usr/bin/env bash
# LinkGeo 一键启动脚本
set -e
cd "$(dirname "$0")"

echo "==> 启动后端 (端口 8090)"
mkdir -p data
(
  cd backend
  export GEO_PORT=8090
  export GEO_DB_PATH=../data/geo-tool.db
  if [ ! -f geotool ]; then
    echo "    首次运行，编译后端..."
    export PATH="/opt/homebrew/bin:$PATH"
    go mod tidy
    go build -o geotool .
  fi
  ./geotool
) &
BACKEND_PID=$!

echo "==> 启动前端 (端口 3000)"
(
  cd frontend
  export PATH="/opt/homebrew/bin:$PATH"
  if [ ! -d node_modules ]; then
    echo "    首次运行，安装前端依赖..."
    npm install
  fi
  npm run dev
) &
FRONTEND_PID=$!

echo ""
echo "=============================================="
echo "  前端:  http://localhost:3000"
echo "  后端:  http://localhost:8090"
echo "  停止:  Ctrl+C"
echo "=============================================="

trap "kill $BACKEND_PID $FRONTEND_PID 2>/dev/null" EXIT
wait
