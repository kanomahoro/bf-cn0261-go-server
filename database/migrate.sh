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

host=""
port=""
force=0
for arg in "$@"; do
  case "$arg" in
    --force) force=1 ;;
    -h|--help)
      sed -n '2,9p' "$0" | sed 's/^# \{0,1\}//'
      exit 0 ;;
    *) if [ -z "$host" ]; then host="$arg"; elif [ -z "$port" ]; then port="$arg"; fi ;;
  esac
done
host="${host:-127.0.0.1}"
port="${port:-3306}"

user="${BF_DB_USER:-bfserver}"
password="${BF_DB_PASSWORD:?BF_DB_PASSWORD is required}"
db="${BF_DB_NAME:-bubblefighter_cn0261}"
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
baseline="$here/baseline.sql"

client="mysql"
command -v mysql >/dev/null 2>&1 || client="mariadb"

[ -f "$baseline" ] || { echo "missing $baseline" >&2; exit 1; }

target="$(sed -n 's/^--[[:space:]]*schema-revision:[[:space:]]*\([0-9][0-9]*\).*/\1/p' "$baseline" | head -n1)"
[ -n "$target" ] || { echo "baseline has no \"-- schema-revision: N\" marker" >&2; exit 1; }

run() { "$client" -h "$host" -P "$port" -u "$user" -p"$password" "$@"; }

echo "目标库   ：${db}@${host}:${port}"
echo "基线版本 ：schema revision ${target}"

# Fail fast with a clear message: the checks below swallow query errors on
# purpose, so a bad password must be caught here instead of looking like an
# empty database.
if ! "$client" -h "$host" -P "$port" -u "$user" -p"$password" -e "SELECT 1" >/dev/null 2>&1; then
  echo "无法连接数据库 ${host}:${port}（检查 BF_DB_USER / BF_DB_PASSWORD 与服务是否运行）" >&2
  exit 1
fi

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

# A partial import would still show the right revision (it is written last), so
# compare the table count with the baseline's own CREATE TABLE statements.
expected_tables="$(grep -c '^CREATE TABLE ' "$baseline" || true)"
if [ "${tables}" != "${expected_tables}" ]; then
  echo "导入后表数为 ${tables}，基线里有 ${expected_tables} 张表：导入不完整" >&2
  exit 1
fi

echo "完成：${tables} 张表，schema revision ${revision}"
echo "下一步：启动服务端（无需再手工改库）"
