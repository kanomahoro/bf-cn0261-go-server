#!/usr/bin/env bash
# 新装 / 重建数据库：把 database/baseline.sql 导入目标库。
#
# 本项目尚未上线，没有升级路径：库版本不匹配一律重建（会 DROP 同名表）。
#
# 用法：
#   BF_DB_USER=bfserver BF_DB_PASSWORD=*** [BF_DB_NAME=bubblefighter_cn0261] \
#     ./database/migrate.sh [host] [port] [--force]
#
# 库已存在且带 schema_version 时默认拒绝导入（baseline 会 DROP 同名表）；
# 确认要整库重建再加 --force。
set -euo pipefail

host="${1:-127.0.0.1}"
port="${2:-3306}"
force=0
for arg in "$@"; do
  case "$arg" in
    --force) force=1 ;;
  esac
done

user="${BF_DB_USER:-bfserver}"
password="${BF_DB_PASSWORD:?BF_DB_PASSWORD is required}"
db="${BF_DB_NAME:-bubblefighter_cn0261}"
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
baseline="$here/baseline.sql"

client="mysql"
command -v mysql >/dev/null 2>&1 || client="mariadb"

[ -f "$baseline" ] || { echo "missing $baseline" >&2; exit 1; }

target="$(grep -m1 -oP '^--\s*schema-revision:\s*\K[0-9]+' "$baseline" || true)"
[ -n "$target" ] || { echo "baseline has no \"-- schema-revision: N\" marker" >&2; exit 1; }

run() { "$client" -h "$host" -P "$port" -u "$user" -p"$password" "$@"; }

echo "目标库   ：${db}@${host}:${port}"
echo "基线版本 ：schema revision ${target}"

"$client" -h "$host" -P "$port" -u "$user" -p"$password" \
  -e "CREATE DATABASE IF NOT EXISTS \`${db}\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci" >/dev/null

existing="$("$client" -h "$host" -P "$port" -u "$user" -p"$password" -N -B \
  -e "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='${db}' AND table_name='schema_version'" 2>/dev/null || echo 0)"
if [ "$existing" != "0" ] && [ "$force" != "1" ]; then
  current="$("$client" -h "$host" -P "$port" -u "$user" -p"$password" -N -B \
    -e "SELECT revision FROM \`${db}\`.schema_version WHERE id = 1" 2>/dev/null || echo '?')"
  cat >&2 <<EOF
库里已经有 schema_version（revision ${current}），拒绝导入基线：baseline 会 DROP 同名表。
  · 确认重建：  bash database/migrate.sh ${host} ${port} --force   （数据会全部丢失）
  · 只想看现状：mariadb -e "SELECT * FROM ${db}.schema_version"
EOF
  exit 1
fi

if [ "$existing" != "0" ]; then
  echo "⚠ --force：正在用基线重建 ${db}（原有数据会丢失）"
fi

echo "导入中 ……"
run "$db" < "$baseline" >/dev/null

revision="$("$client" -h "$host" -P "$port" -u "$user" -p"$password" -N -B \
  -e "SELECT revision FROM \`${db}\`.schema_version WHERE id = 1")"
tables="$("$client" -h "$host" -P "$port" -u "$user" -p"$password" -N -B \
  -e "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='${db}'")"

if [ "$revision" != "$target" ]; then
  echo "导入完成但 revision 是 ${revision}，预期 ${target}" >&2
  exit 1
fi

echo "完成：${tables} 张表，schema revision ${revision}"
echo "下一步：启动服务端（无需再手工改库）"
