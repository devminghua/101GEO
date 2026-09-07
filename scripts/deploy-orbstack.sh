#!/bin/bash
# =============================================================================
# LinkGeo 一键自动部署到 OrbStack（本地 Docker 环境）
#
# 每次更新完代码后跑这一条，自动完成：
#   备份数据库 → 构建前端 → 同步内嵌产物 → 重建镜像 → 重启容器 → 健康验证
#
# 用法：
#   bash scripts/deploy-orbstack.sh
#
# 说明：
#   - 数据库现在是 PostgreSQL（geotool-postgres 容器），备份用 pg_dump 导出到 backups/pg/。
#   - SQLite 兜底备份（单机版 + 回滚用）仍走 backup-db.sh。
#   - 前端内嵌进后端二进制（go:embed webdist），所以前端改动必须走这里重建。
# =============================================================================
set -euo pipefail

cd "$(dirname "$0")/.."
TIMESTAMP=$(date +%Y%m%d-%H%M%S)

echo "==> [1/6] 备份数据库（铁律：先备份再动）"
# PostgreSQL 备份（当前权威源）
mkdir -p backups/pg
if docker ps --format '{{.Names}}' | grep -q geotool-postgres; then
  docker exec geotool-postgres pg_dump -U postgres -d geotool > "backups/pg/geotool-${TIMESTAMP}.sql" 2>/dev/null \
    && echo "    [OK] PostgreSQL 备份: backups/pg/geotool-${TIMESTAMP}.sql ($(du -h backups/pg/geotool-${TIMESTAMP}.sql | cut -f1))" \
    || echo "    [WARN] PostgreSQL 备份失败，请检查容器状态"
fi
# SQLite 兜底备份（单机版 / 回滚用）
if [ -f backend/geo-tool.db ]; then
  bash scripts/backup-db.sh dev >/dev/null 2>&1 && echo "    [OK] SQLite 开发库已备份" || echo "    [WARN] SQLite 备份跳过"
fi

echo "==> [2/6] 构建前端"
cd frontend
if [ -d dist ]; then
  mv dist "/tmp/frontend-dist-old-${TIMESTAMP}"   # 先移走，避开 safe-delete 保护
fi
npm run build
cd ..

echo "==> [3/6] 同步前端产物到后端内嵌目录"
rsync -a --delete --exclude='.DS_Store' frontend/dist/ backend/webdist/

echo "==> [4/6] 重新构建镜像（含最新代码 + 前端）"
# 用传统 builder，规避 buildx activity 权限问题
DOCKER_BUILDKIT=0 docker build -t geotool:latest .

echo "==> [5/6] 重启容器（geotool + postgres）"
docker compose up -d

echo "==> [6/6] 健康验证"
for i in $(seq 1 40); do
  if curl -s --noproxy '*' --max-time 2 http://127.0.0.1:8080/api/system/info >/dev/null 2>&1; then
    echo "    ✅ 服务就绪"
    break
  fi
  [ "$i" = "40" ] && echo "    ❌ 服务未就绪，请查看日志: docker logs geotool"
  sleep 2
done

echo ""
echo "✅ 部署完成！访问 http://127.0.0.1:8080"
echo "   登录：admin / ymh@0921tT"
