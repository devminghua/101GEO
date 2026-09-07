#!/bin/bash
# geo-tool 数据库标准备份脚本（更新/部署前必须先跑）
# 数据库为 PostgreSQL：用 pg_dump 在容器内在线备份
# 用法: bash scripts/backup-db.sh        # 备份容器库（PG）
#       bash scripts/backup-db.sh pg     # 同上
set -eu

GEO_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BACKUP_DIR="${GEO_DIR}/backups/pg"
KEEP=20          # 保留最近份数
STAMP="$(date +%Y%m%d-%H%M%S)"

mkdir -p "${BACKUP_DIR}"

# 容器名与 PG 连接参数（与 docker-compose.yml 一致）
PG_CONTAINER="${GEO_PG_CONTAINER:-geotool-postgres}"
PG_USER="${GEO_PG_USER:-postgres}"
PG_DB="${GEO_PG_DB:-geotool}"

backup_pg() {
    local dst="${BACKUP_DIR}/geotool-${STAMP}.sql"
    if ! docker ps --filter "name=^/${PG_CONTAINER}$" --format '{{.Names}}' | grep -q .; then
        echo "[错误] PostgreSQL 容器 ${PG_CONTAINER} 未运行，无法备份" >&2
        exit 1
    fi
    # pg_dump 在线备份（容器内执行，-Fc 自定义格式，便于 pg_restore）
    if docker exec "$PG_CONTAINER" pg_dump -U "$PG_USER" -d "$PG_DB" -Fc -f /tmp/geotool.dump 2>/tmp/pgdump.err; then
        docker cp "${PG_CONTAINER}:/tmp/geotool.dump" "$dst"
        docker exec "$PG_CONTAINER" rm -f /tmp/geotool.dump
        local size=$(du -h "$dst" | cut -f1)
        echo "[OK] PostgreSQL 备份: ${dst} (${size})"
    else
        echo "[失败] pg_dump 出错：" >&2
        cat /tmp/pgdump.err >&2 2>/dev/null || true
        rm -f "$dst"
        exit 1
    fi
    # 滚动清理：只保留最近 KEEP 份
    ls "${BACKUP_DIR}"/geotool-*.sql 2>/dev/null | sort -r | tail -n +$((KEEP + 1)) | while read -r old; do
        rm -f "$old"
        echo "[清理] 旧备份: ${old}"
    done
}

backup_pg

echo "备份完成: ${BACKUP_DIR}"
