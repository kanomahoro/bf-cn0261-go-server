#!/usr/bin/env bash
# 新装 / 重建数据库：把 database/baseline.sql 导入目标库。
#
# 本项目尚未上线，没有升级路径：库版本不匹配一律重建（会 DROP 表）。
#
# 用法：
#   BF_DB_USER=bfserver BF_DB_PASSWORD=*** [BF_DB_NAME=bubblefighter_cn0261] \
#     ./database/migrate.sh [host] [port] [--force]
#
# 目标库非空时默认拒绝（baseline 会 DROP 同名表）；确认整库重建才加 --force，
# 它会把目标库里的表全部清掉再导入，账号、角色、道具一并丢失。
set -euo pipefail

host=""
port=""
force=0
for arg in "$@"; do
  case "$arg" in
    --force) force=1 ;;
    -h|--help)
      sed -n '2,11p' "$0" | sed 's/^# \{0,1\}//'
      exit 0 ;;
    *)
      if [ -z "$host" ]; then
        host="$arg"
      elif [ -z "$port" ]; then
        port="$arg"
      else
        echo "多余的参数：$arg（只接受 [host] [port] [--force]）" >&2
        exit 1
      fi
      ;;
  esac
done
host="${host:-127.0.0.1}"
port="${port:-3306}"

user="${BF_DB_USER:-bfserver}"
password="${BF_DB_PASSWORD:?BF_DB_PASSWORD is required}"
db="${BF_DB_NAME:-bubblefighter_cn0261}"

# 库名会拼进标识符和字符串字面量，限制字符集而不是靠转义。
if ! printf '%s' "$db" | LC_ALL=C grep -Eq '^[A-Za-z0-9_-]+$'; then
  echo "库名只能包含字母、数字、下划线和短横线：${db}" >&2
  exit 1
fi
if ! printf '%s' "$port" | LC_ALL=C grep -Eq '^[0-9]+$'; then
  echo "端口必须是数字：${port}" >&2
  exit 1
fi

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
baseline="$here/baseline.sql"

client="mysql"
command -v mysql >/dev/null 2>&1 || client="mariadb"

[ -f "$baseline" ] || { echo "missing $baseline" >&2; exit 1; }

target="$(sed -n 's/^--[[:space:]]*schema-revision:[[:space:]]*\([0-9][0-9]*\).*/\1/p' "$baseline" | head -n1)"
[ -n "$target" ] || { echo "baseline has no \"-- schema-revision: N\" marker" >&2; exit 1; }

# 表名清单来自基线本身，导入前后的校验都以它为准。
expected_tables="$(sed -n 's/^CREATE TABLE `\([^`]*\)`.*/\1/p' "$baseline")"
[ -n "$expected_tables" ] || { echo "baseline has no CREATE TABLE statements" >&2; exit 1; }
expected_count="$(printf '%s\n' "$expected_tables" | wc -l | tr -d ' ')"

# 密码走 MYSQL_PWD，不放进命令行，避免出现在进程列表里。
export MYSQL_PWD="$password"
run() { "$client" -h "$host" -P "$port" -u "$user" "$@"; }
query() { run -N -B -e "$1"; }

echo "目标库   ：${db}@${host}:${port}"
echo "基线版本 ：schema revision ${target}"

# Fail fast with a clear message: the checks below swallow query errors on
# purpose, so a bad password must be caught here instead of looking like an
# empty database.
if ! run -e "SELECT 1" >/dev/null 2>&1; then
  echo "无法连接数据库 ${host}:${port}（检查 BF_DB_USER / BF_DB_PASSWORD 与服务是否运行）" >&2
  exit 1
fi

# 判断目标库是否非空。不看 schema_version：老版本的库（没有这张表）同样装着
# 账号和角色，只认 marker 会把它们当成新库直接覆盖掉。
tables_now="$(query "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='${db}'" 2>/dev/null || echo 0)"
tables_now="${tables_now:-0}"
if ! printf '%s' "$tables_now" | LC_ALL=C grep -Eq '^[0-9]+$'; then
  echo "无法读取 ${db} 的表数量（拿到了 '${tables_now}'），停止导入" >&2
  exit 1
fi

if [ "$tables_now" != "0" ] && [ "$force" != "1" ]; then
  cat >&2 <<EOF
库里已经有 ${tables_now} 张表，拒绝导入基线：baseline 会 DROP 同名表。
  · 确认重建：  bash database/migrate.sh ${host} ${port} --force   （数据会全部丢失）
  · 只想看现状：${client} -e "SHOW TABLES FROM \\\`${db}\\\`"
EOF
  exit 1
fi

if [ "$force" = "1" ] && [ "$tables_now" != "0" ]; then
  echo "⚠ --force：正在清空 ${db} 的 ${tables_now} 张表（数据会全部丢失）"
  leftover="$(query "SELECT table_name FROM information_schema.tables WHERE table_schema='${db}'")"
  if [ -n "$leftover" ]; then
    # 逐表删除而不是整库 DROP：库级授权的账号没有 DROP DATABASE 权限也能重建。
    {
      echo "SET FOREIGN_KEY_CHECKS=0;"
      printf '%s\n' "$leftover" | while IFS= read -r table; do
        [ -n "$table" ] && printf 'DROP TABLE IF EXISTS `%s`;\n' "$table"
      done
      echo "SET FOREIGN_KEY_CHECKS=1;"
    } | run "$db" >/dev/null
  fi
fi

# 库已存在时不再发 CREATE DATABASE：只有库级授权的账号没有建库权限，但那不影响在
# 已有库里导入。
db_exists="$(query "SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name='${db}'" 2>/dev/null || echo 0)"
if [ "${db_exists:-0}" = "0" ]; then
  if ! run -e "CREATE DATABASE \`${db}\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci" >/dev/null; then
    echo "建库失败：账号 ${user} 需要建库权限（或让管理员先建好 ${db}）" >&2
    exit 1
  fi
fi

echo "导入中 ……"
if ! run --default-character-set=utf8mb4 "$db" < "$baseline" >/dev/null; then
  echo "导入失败：账号 ${user} 需要 ${db} 的 CREATE / DROP / INSERT 权限，详见 docs/database.md" >&2
  exit 1
fi

revision="$(query "SELECT revision FROM \`${db}\`.schema_version WHERE id = 1" 2>/dev/null || true)"
if [ "$revision" != "$target" ]; then
  echo "导入完成但 revision 是 ${revision:-<空>}，预期 ${target}" >&2
  exit 1
fi

# A partial import can still carry the right revision (it is written last), so
# every table the baseline creates has to be present.
present="$(query "SELECT table_name FROM information_schema.tables WHERE table_schema='${db}'")"
missing=""
while IFS= read -r table; do
  [ -z "$table" ] && continue
  if ! printf '%s\n' "$present" | grep -Fxq "$table"; then
    missing="${missing} ${table}"
  fi
done <<< "$expected_tables"
if [ -n "$missing" ]; then
  echo "导入不完整，缺表：${missing}" >&2
  exit 1
fi

# 多余的表不影响服务端（结构校验容忍），只提示一句。
extra=""
while IFS= read -r table; do
  [ -z "$table" ] && continue
  if ! printf '%s\n' "$expected_tables" | grep -Fxq "$table"; then
    extra="${extra} ${table}"
  fi
done <<< "$present"

echo "完成：${expected_count} 张表，schema revision ${revision}"
if [ -n "$extra" ]; then
  echo "注意：库里还有基线之外的表（服务端会忽略）：${extra}"
fi
echo "下一步：启动服务端（无需再手工改库）"
