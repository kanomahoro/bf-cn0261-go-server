#!/usr/bin/env bash
# 老库升级：按 database/upgrades 里的 target-revision 依次升级，升级前自动备份。
#
# 用法：
#   BF_DB_USER=bfserver BF_DB_PASSWORD=*** [BF_DB_NAME=bubblefighter_cn0261] \
#     ./database/upgrade.sh [host] [port] [--no-backup] [--dry-run]
#
# 新装请用 database/migrate.sh（导 baseline）。本脚本只做增量，不 DROP 任何表。
set -euo pipefail

host=""
port=""
no_backup=0
dry_run=0
for arg in "$@"; do
  case "$arg" in
    --no-backup) no_backup=1 ;;
    --dry-run)   dry_run=1 ;;
    *) if [ -z "$host" ]; then host="$arg"; elif [ -z "$port" ]; then port="$arg"; fi ;;
  esac
done
host="${host:-127.0.0.1}"
port="${port:-3306}"

user="${BF_DB_USER:-bfserver}"
password="${BF_DB_PASSWORD:?BF_DB_PASSWORD is required}"
db="${BF_DB_NAME:-bubblefighter_cn0261}"
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
upgrades="$here/upgrades"
backups="$here/backups"

client="mysql"
command -v mysql >/dev/null 2>&1 || client="mariadb"
dump="mysqldump"
command -v mysqldump >/dev/null 2>&1 || dump="mariadb-dump"

q() { "$client" -h "$host" -P "$port" -u "$user" -p"$password" -N -B "$@"; }

[ -d "$upgrades" ] || { echo "missing $upgrades" >&2; exit 1; }

# ---- 当前版本：没有 schema_version 视为 revision 1（14 份历史迁移的库）----
have_table="$(q -e "SELECT COUNT(*) FROM information_schema.tables
                    WHERE table_schema='${db}' AND table_name='schema_version'" 2>/dev/null || echo 0)"
if [ "$have_table" = "0" ]; then
  current=1
  current_note="无 schema_version 表（历史迁移库）"
else
  current="$(q -e "SELECT COALESCE(MAX(revision),1) FROM \`${db}\`.schema_version" 2>/dev/null || echo 1)"
  current_note="schema_version.revision=${current}"
fi

# ---- 升级清单 ----
steps=()
target=0
while IFS= read -r file; do
  step_target="$(grep -m1 -oP '^--\s*target-revision:\s*\K[0-9]+' "$file" || true)"
  if [ -z "$step_target" ]; then
    echo "$(basename "$file") 缺少 \"-- target-revision: N\" 标记" >&2
    exit 1
  fi
  steps+=("${step_target}|${file}")
  if [ "$step_target" -gt "$target" ]; then
    target="$step_target"
  fi
done < <(find "$upgrades" -maxdepth 1 -name '*.sql' | sort)

echo "目标库   ：${db}@${host}:${port}"
echo "当前版本 ：revision ${current}（${current_note}）"
echo "脚本版本 ：revision ${target}（${#steps[@]} 个升级脚本）"

if [ "$current" -gt "$target" ]; then
  echo "库的 revision 比脚本还新，请升级服务端/脚本，别用旧脚本改库" >&2
  exit 1
fi
if [ "$current" -eq "$target" ]; then
  echo "已经是最新，无需升级"
  exit 0
fi

pending=()
for item in "${steps[@]}"; do
  step_target="${item%%|*}"
  file="${item#*|}"
  if [ "$step_target" -gt "$current" ]; then
    pending+=("$file|$step_target")
  fi
done

echo
echo "计划："
shown="$current"
for item in "${pending[@]}"; do
  file="${item%%|*}"
  step_target="${item##*|}"
  echo "  revision ${shown} → ${step_target}：$(basename "$file")"
  shown="$step_target"
done

if [ "$dry_run" = "1" ]; then
  echo
  echo "--dry-run：未执行任何语句"
  exit 0
fi

echo
echo "提示：升级期间请先停掉服务端，避免并发写入。"

# ---- 备份 ----
if [ "$have_table" = "0" ]; then
  current_saved=1
else
  current_saved="$current"
fi
if [ "$no_backup" = "0" ]; then
  mkdir -p "$backups"
  stamp="$(date +%Y%m%d-%H%M%S)"
  file="$backups/${db}-pre-upgrade-r${current_saved}-${stamp}.sql"
  echo "备份中 → ${file}"
  if ! "$dump" -h "$host" -P "$port" -u "$user" -p"$password" \
        --single-transaction --no-tablespaces --default-character-set=utf8mb4 \
        "$db" > "$file"; then
    echo "备份失败，已中止（确认磁盘空间/权限，或加 --no-backup 自行承担风险）" >&2
    exit 1
  fi
  if [ ! -s "$file" ]; then
    echo "备份文件为空，已中止" >&2
    exit 1
  fi
  echo "备份完成（$(du -h "$file" | cut -f1)）"
else
  echo "⚠ --no-backup：跳过备份"
fi

# ---- 逐个执行 ----
for item in "${pending[@]}"; do
  file="${item%%|*}"
  step_target="${item##*|}"
  echo
  echo "==> $(basename "$file")（目标 revision ${step_target}）"
  if ! "$client" -h "$host" -P "$port" -u "$user" -p"$password" --default-character-set=utf8mb4 \
        "$db" < "$file" >/dev/null; then
    echo "升级失败：$(basename "$file")；修好后可重跑本脚本（每个脚本都是幂等的）" >&2
    exit 1
  fi
  now="$(q -e "SELECT COALESCE(MAX(revision),0) FROM \`${db}\`.schema_version" 2>/dev/null || echo 0)"
  if [ "$now" != "$step_target" ]; then
    echo "执行完成但 revision 是 ${now}，预期 ${step_target}" >&2
    exit 1
  fi
  echo "    revision → ${now}"
done

echo
echo "升级完成：revision ${target}。现在可以启动服务端了。"
