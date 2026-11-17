#!/usr/bin/env bash
# ============================================================================
#  泡泡战士 CN0261 服务端 · Debian 12+ 一键安装脚本
#
#  用法：
#     bash bfserver-install.sh              # 交互安装（推荐）
#     bash bfserver-install.sh --yes        # 全自动，全部用默认值
#     bash bfserver-install.sh --dry-run    # 只看会做什么，不改系统
#     bash bfserver-install.sh --uninstall  # 卸载（保留数据库，会再问一次）
#
#  离线安装（国内拉不动 GitHub 时用）：
#     bash bfserver-install.sh --offline \
#          --bin ./bfserver-linux-amd64-v1.1.0.tar.gz \
#          --src ./v1.1.0.tar.gz
#     --bin <路径>   用本地二进制包代替 GitHub Release 下载
#     --src <路径>   用本地源码包代替 GitHub 归档下载
#     --offline      两个都用本地包（此时必须同时给 --bin 和 --src）
#     --no-sha256    跳过二进制包哈希校验（自己编译的包用）
#     注意：离线模式不联网，MariaDB 等系统依赖需事先用本地源装好。
#
#  已有旧版本（数据库是旧结构）时：
#     bash bfserver-install.sh --rebuild-db
#     --rebuild-db   备份现有库（/opt/bf-backup-*.sql.gz）后整库重建为新基线。
#                    会清空账号、角色、道具、战绩；不加这个参数时，脚本发现库结构与
#                    本版不符会在改任何文件之前就停下，不会留下半装的现场。
#
#  面向 Linux 新手：每一步都有中文提示，出错会告诉你下一步做什么。
# ============================================================================
set -Eeuo pipefail

VERSION="1.1.0"
REPO="kanomahoro/bf-cn0261-go-server"
BIN_ASSET="bfserver-linux-amd64-v${VERSION}.tar.gz"
BIN_SHA256="a35680ef125d3dcba4a8df250348841d6872c5f7b38ce5db68c27c16b803d084"
PREFIX="/opt/bfserver"
SERVICE="bfserver"
DB_NAME="bubblefighter_cn0261"
DB_USER="bfserver"
LOG_FILE="/var/log/bfserver-install.log"
SECRET_FILE="/root/.bfserver-secrets"
UNIT="/etc/systemd/system/${SERVICE}.service"

FORCE_YES=0
DRY_RUN=0
DO_UNINSTALL=0
OFFLINE=0
NO_SHA256=0
REBUILD_DB=0
BIN_LOCAL=""
SRC_LOCAL=""
STEP=0

# ----------------------------------------------------------------------------
# 输出
# ----------------------------------------------------------------------------
c_reset=$'\033[0m'; c_b=$'\033[1m'; c_g=$'\033[32m'; c_y=$'\033[33m'; c_r=$'\033[31m'; c_c=$'\033[36m'
say()   {
  printf '%s\n' "$*" | sed -E 's/\x1b\[[0-9;]*m//g' >> "$LOG_FILE" 2>/dev/null || true
  printf '%s\n' "$*"
}
ok()    { say "${c_g}  ✓${c_reset} $*"; }
info()  { say "    $*"; }
warn()  { say "${c_y}  !${c_reset} $*"; }
# 敏感值只给屏幕，绝不进日志文件
say_secret() { printf '%s\n' "$*"; }
die()   { say "${c_r}  ✗ $*${c_reset}"; exit 1; }
step()  { STEP=$1; say ""; say "${c_b}${c_c}[$1/$TOTAL_STEPS] $2${c_reset}"; }
TOTAL_STEPS=8

on_error() {
  local code=$?
  say ""
  say "${c_r}  ✗ 安装在第 ${STEP} 步中断（退出码 ${code}）${c_reset}"
  say "    完整日志：${LOG_FILE}"
  say "    修好问题后可以重新执行本脚本，已完成的步骤不会重复做。"
  exit $code
}
trap on_error ERR

# ----------------------------------------------------------------------------
# 参数
# ----------------------------------------------------------------------------
abspath() { # 相对路径 → 绝对路径（后面会 cd，路径必须提前定死）
  local p="$1"
  case "$p" in /*) ;; *) p="$PWD/$p" ;; esac
  readlink -f -- "$p" 2>/dev/null || printf '%s' "$p"
}

while [ $# -gt 0 ]; do
  case "$1" in
    --yes|-y)     FORCE_YES=1; shift ;;
    --dry-run)    DRY_RUN=1; shift ;;
    --uninstall)  DO_UNINSTALL=1; shift ;;
    --offline)    OFFLINE=1; shift ;;
    --no-sha256)  NO_SHA256=1; shift ;;
    --rebuild-db) REBUILD_DB=1; shift ;;
    --bin)
      [ $# -ge 2 ] || die "--bin 后面要跟本地二进制包的路径"
      BIN_LOCAL=$(abspath "$2"); shift 2 ;;
    --bin=*)      BIN_LOCAL=$(abspath "${1#*=}"); shift ;;
    --src)
      [ $# -ge 2 ] || die "--src 后面要跟本地源码包的路径"
      SRC_LOCAL=$(abspath "$2"); shift 2 ;;
    --src=*)      SRC_LOCAL=$(abspath "${1#*=}"); shift ;;
    --help|-h)
      awk 'NR>2 && /^# =====/{exit} NR>2{sub(/^# ?/,""); print}' "$0"
      exit 0 ;;
    *)            die "未知参数：$1（用 --help 查看用法）" ;;
  esac
done

# 本地包提前检查：别等装完 apt、建完库才发现路径写错了
for f in "$BIN_LOCAL" "$SRC_LOCAL"; do
  [ -z "$f" ] && continue
  [ -f "$f" ] || die "找不到本地包：$f"
  [ -r "$f" ] || die "本地包没有读权限：$f"
done

if [ "$OFFLINE" = "1" ]; then
  [ -n "$BIN_LOCAL" ] || die "--offline 需要同时指定 --bin（本地二进制包路径）"
  [ -n "$SRC_LOCAL" ] || die "--offline 需要同时指定 --src（本地源码包路径）"
fi

# ----------------------------------------------------------------------------
# 工具
# ----------------------------------------------------------------------------
rand_str() { # rand_str <长度>：纯字母数字，避免任何需要在命令行转义的字符
  local pool
  pool=$(LC_ALL=C tr -dc 'A-Za-z0-9' < /dev/urandom 2>/dev/null | head -c 512 || true)
  printf '%s' "${pool:0:$1}"
}

ask() { # ask <变量名> <提示> <默认值> [mask]
  local __var="$1" __prompt="$2" __default="$3" __mask="${4:-}" __answer=""
  local __shown="$__default"
  [ "$__mask" = "mask" ] && __shown="******"   # 敏感值不在屏幕上回显、也不进日志
  if [ "$FORCE_YES" = "1" ] || [ ! -t 0 ]; then
    printf -v "$__var" '%s' "$__default"
    info "$__prompt ${__shown:-(留空)}  （自动采用）"
    return
  fi
  if [ -n "$__default" ]; then
    printf '%s [%s]: ' "$__prompt" "$__shown"
  else
    printf '%s: ' "$__prompt"
  fi
  read -r __answer || true
  [ -z "$__answer" ] && __answer="$__default"
  printf -v "$__var" '%s' "$__answer"
}

run() { # 干跑模式下只打印命令
  if [ "$DRY_RUN" = "1" ]; then
    say "    [dry-run] $*"
    return 0
  fi
  eval "$@"
}

need_root() {
  [ "$(id -u)" = "0" ] || die "请用 root 运行：sudo bash $0"
}

# ----------------------------------------------------------------------------
# 卸载
# ----------------------------------------------------------------------------
if [ "$DO_UNINSTALL" = "1" ]; then
  need_root
  say "${c_b}卸载泡泡战士服务端${c_reset}"
  systemctl stop "$SERVICE" 2>/dev/null || true
  systemctl disable "$SERVICE" 2>/dev/null || true
  rm -f "$UNIT"
  systemctl daemon-reload 2>/dev/null || true
  if [ -d "$PREFIX" ]; then
    rm -rf "$PREFIX"
    ok "已删除 $PREFIX"
  fi
  ask dropdb "是否连数据库 ${DB_NAME} 一起删除？（数据会永久丢失，输入 yes 确认）" ""
  if [ "$dropdb" = "yes" ]; then
    if command -v mariadb >/dev/null 2>&1; then
      mariadb -e "DROP DATABASE IF EXISTS \`${DB_NAME}\`;" && ok "数据库已删除"
      mariadb -e "DROP USER IF EXISTS '${DB_USER}'@'127.0.0.1', '${DB_USER}'@'localhost';" || true
    else
      warn "没找到 mariadb 客户端，请手动删除数据库 ${DB_NAME}"
    fi
  else
    info "数据库 ${DB_NAME} 已保留"
  fi
  rm -f "$SECRET_FILE"
  ok "卸载完成"
  exit 0
fi

# ============================================================================
# 1. 环境体检
# ============================================================================

[ -f "$LOG_FILE" ] || : > "$LOG_FILE"
chmod 600 "$LOG_FILE" 2>/dev/null || true

say "${c_b}泡泡战士 CN0261 服务端 · 安装程序 v${VERSION}${c_reset}"
say "日志：${LOG_FILE}"
[ "$DRY_RUN" = "1" ] && warn "干跑模式：不会修改系统，只显示将要做的事"

step 1 "检查系统环境"
[ "$DRY_RUN" = "1" ] || need_root

# 注意：用子 shell 读取 os-release，避免它的 VERSION 变量覆盖本脚本的 VERSION
if [ -r /etc/os-release ]; then
  OS_PRETTY=$( . /etc/os-release && printf '%s' "${PRETTY_NAME:-unknown}" )
  OS_ID=$( . /etc/os-release && printf '%s' "${ID:-}" )
  OS_VER=$( . /etc/os-release && printf '%s' "${VERSION_ID:-0}" )
  info "系统：${OS_PRETTY}"
  [ "$OS_ID" = "debian" ] || warn "本脚本为 Debian 12+ 编写，当前是 ${OS_ID:-未知}，可能不兼容"
  case "$OS_VER" in
    1[2-9]|2[0-9]) :
      ;;
    *) warn "本脚本要求 Debian 12 或更高，当前 ${OS_VER}" ;;
  esac
else
  warn "读不到 /etc/os-release，跳过系统检查"
fi

ARCH="$(uname -m)"
[ "$ARCH" = "x86_64" ] || die "本包只支持 x86_64（amd64），当前是 ${ARCH}"
ok "架构：${ARCH}"

MEM_MB=$(awk '/MemTotal/{printf "%d", $2/1024}' /proc/meminfo)
[ "$MEM_MB" -ge 480 ] || warn "内存只有 ${MEM_MB} MB，建议 1 GB 以上"
ok "内存：${MEM_MB} MB"

AVAIL_MB=$(df -Pm /opt 2>/dev/null | awk 'NR==2{print $4}')
[ "${AVAIL_MB:-0}" -ge 1500 ] || warn "剩余磁盘只有 ${AVAIL_MB:-?} MB，建议 2 GB 以上"
ok "磁盘：可用 ${AVAIL_MB:-?} MB"

for c in curl tar openssl; do
  command -v "$c" >/dev/null 2>&1 && info "已有 $c" || info "需要安装 $c"
done

# ============================================================================
# 2. 收集信息
# ============================================================================
step 2 "确认安装参数"

# 公网 IP：优先沿用已有配置，其次自动探测
PUBLIC_IP=""
if [ -f "${PREFIX}/config/server.json" ]; then
  PUBLIC_IP=$(python3 - "${PREFIX}/config/server.json" <<'PY' 2>/dev/null || true
import json,sys
try:
    d=json.load(open(sys.argv[1]))
    ip=d.get("relay",{}).get("advertisedAddress","")
    print("" if ip=="127.0.0.1" else ip)
except Exception:
    print("")
PY
)
fi
if [ -z "$PUBLIC_IP" ]; then
  PUBLIC_IP=$(ip -4 route get 1.1.1.1 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="src"){print $(i+1); exit}}' || true)
fi
[ -n "$PUBLIC_IP" ] || PUBLIC_IP=""
ask PUBLIC_IP "服务器公网 IP（客户端要连的那个）" "$PUBLIC_IP"
[ -n "$PUBLIC_IP" ] || die "公网 IP 不能为空。在云控制台可以看到，填它就好"
echo "$PUBLIC_IP" | grep -Eq '^([0-9]{1,3}\.){3}[0-9]{1,3}$' || die "公网 IP 格式不对：${PUBLIC_IP}"
case "$PUBLIC_IP" in
  10.*|192.168.*|127.*) warn "这个地址看起来是内网/回环地址。云主机请填控制台上的「公网 IP」，否则远程玩家进不了对局" ;;
  172.1[6-9].*|172.2[0-9].*|172.3[01].*) warn "这个地址看起来是内网地址（172.16-31 段）。云主机请填公网 IP" ;;
esac

# 邀请码：沿用已有配置，否则随机生成
INVITE_CODE=""
if [ -f "${PREFIX}/config/server.json" ]; then
  INVITE_CODE=$(python3 - "${PREFIX}/config/server.json" <<'PY' 2>/dev/null || true
import json,sys
try:
    print(json.load(open(sys.argv[1])).get("authentication",{}).get("inviteCode",""))
except Exception:
    print("")
PY
)
fi
[ -n "$INVITE_CODE" ] || INVITE_CODE="$(rand_str 10)"
ask INVITE_CODE "登录器注册邀请码（玩家注册时要填，回车用默认）" "$INVITE_CODE" mask
INVITE_CODE=$(printf '%s' "$INVITE_CODE" | LC_ALL=C tr -dc 'A-Za-z0-9')
[ ${#INVITE_CODE} -ge 4 ] || die "邀请码至少 4 位，只能是字母和数字"

# 数据库密码：沿用已有配置，否则随机生成
DB_PASS=""
if [ -f "${PREFIX}/config/server.json" ]; then
  DB_PASS=$(python3 - "${PREFIX}/config/server.json" <<'PY' 2>/dev/null || true
import json,sys
try:
    print(json.load(open(sys.argv[1])).get("database",{}).get("password",""))
except Exception:
    print("")
PY
)
fi
[ -n "$DB_PASS" ] || DB_PASS="$(rand_str 24)"

if [ -d "$PREFIX" ] && [ -f "${PREFIX}/config/server.json" ]; then
  warn "检测到已有安装在 ${PREFIX}：本次为「更新」——配置会备份后沿用，数据库要看第 6 步的检查结果"
  MODE="update"
else
  MODE="fresh"
fi
info "公网 IP：${PUBLIC_IP}"
say_secret "    邀请码：${INVITE_CODE}"   # 邀请码同样只给屏幕，不落日志
info "数据库密码：已随机生成 24 位，不打印（存在配置文件与 ${SECRET_FILE}）"
info "模式：$([ "$MODE" = "fresh" ] && echo 全新安装 || echo 更新)"

# ============================================================================
# 3. 安装依赖
# ============================================================================
step 3 "检查并安装系统依赖（MariaDB、python3、tar、openssl）"
MISSING_PKGS=""
for p in mariadb-server mariadb-client python3 curl tar openssl ca-certificates; do
  dpkg -s "$p" >/dev/null 2>&1 || MISSING_PKGS="$MISSING_PKGS $p"
done
if [ -z "$MISSING_PKGS" ]; then
  ok "系统依赖已就绪，跳过 apt（不用联网）"
elif [ "$DRY_RUN" = "1" ]; then
  say "    [dry-run] apt-get update && apt-get install -y${MISSING_PKGS}"
elif [ "$OFFLINE" = "1" ]; then
  die "离线模式下缺少系统依赖：${MISSING_PKGS# }
    请先用本地源或离线 deb 装好这些包（apt-get install 会联网）。
    已经能联网的话，去掉 --offline 让脚本自己装。"
else
  export DEBIAN_FRONTEND=noninteractive
  apt-get update -qq >>"$LOG_FILE" 2>&1 || die "apt update 失败。换一个软件源或稍后重试，日志：${LOG_FILE}"
  apt-get install -y -qq mariadb-server mariadb-client python3 curl tar openssl ca-certificates >>"$LOG_FILE" 2>&1 \
    || die "安装 MariaDB 失败，日志：${LOG_FILE}"
  ok "依赖就绪"
fi

# migrate.sh 需要 mysql 命令，Debian 新版客户端叫 mariadb
if [ "$DRY_RUN" = "0" ]; then
  if ! command -v mysql >/dev/null 2>&1 && command -v mariadb >/dev/null 2>&1; then
    ln -sf "$(command -v mariadb)" /usr/local/bin/mysql
  fi
  systemctl enable --now mariadb >>"$LOG_FILE" 2>&1 || die "MariaDB 启动失败，执行 systemctl status mariadb 查看原因"
  for i in $(seq 1 30); do
    mariadb -e 'SELECT 1' >/dev/null 2>&1 && break
    sleep 1
  done
  mariadb -e 'SELECT 1' >/dev/null 2>&1 || die "MariaDB 连不上，检查 systemctl status mariadb"
fi
[ "$DRY_RUN" = "1" ] && info "[dry-run] 启动并设为开机自启" || ok "MariaDB 已运行"

# ============================================================================
# 4. 准备安装包（在线下载 或 本地文件）
# ============================================================================
step 4 "准备 v${VERSION} 的二进制包与源码包"
TMP=$(mktemp -d /tmp/bfserver-install.XXXXXX)
trap 'rm -rf "$TMP"' EXIT

BIN_URL="https://github.com/${REPO}/releases/download/v${VERSION}/${BIN_ASSET}"
SRC_URL="https://github.com/${REPO}/archive/refs/tags/v${VERSION}.tar.gz"

fetch() { # fetch <url> <输出文件>
  if [ "$DRY_RUN" = "1" ]; then
    say "    [dry-run] curl -L $1 -o $2"
    return 0
  fi
  curl -fL --retry 3 --retry-delay 2 --connect-timeout 20 --progress-bar \
       -o "$2" "$1" >>"$LOG_FILE" 2>&1
}

take_local() { # take_local <名称> <本地文件> <目标文件>：只写临时目录，dry-run 下也真做
  local name="$1" from="$2" to="$3"
  cp -f "$from" "$to" || die "${name}复制失败：${from}"
  info "${name}：${from}（本地包，$(du -h "$to" | awk '{print $1}')）"
}

if [ -n "$BIN_LOCAL" ]; then
  take_local "二进制包" "$BIN_LOCAL" "$TMP/bin.tar.gz"
else
  fetch "$BIN_URL" "$TMP/bin.tar.gz"
fi
if [ -n "$SRC_LOCAL" ]; then
  take_local "源码包" "$SRC_LOCAL" "$TMP/src.tar.gz"
else
  fetch "$SRC_URL" "$TMP/src.tar.gz"
fi
ok "安装包就绪"

# 校验：本地包一定校验（dry-run 也校验）；下载包只在真跑时才存在
if [ -f "$TMP/bin.tar.gz" ]; then
  ACTUAL=$(sha256sum "$TMP/bin.tar.gz" | awk '{print $1}')
  if [ "$ACTUAL" = "$BIN_SHA256" ]; then
    ok "SHA256 校验通过"
  elif [ -n "$BIN_LOCAL" ] && [ "$NO_SHA256" = "1" ]; then
    warn "本地二进制包与官方 v${VERSION} 哈希不同，已按 --no-sha256 跳过校验"
  elif [ -n "$BIN_LOCAL" ]; then
    die "本地二进制包和官方 v${VERSION} 的哈希对不上。
    期望 ${BIN_SHA256}
    实际 ${ACTUAL}
    请确认选的是 v${VERSION} 的发布包；自己编译的包请加 --no-sha256。"
  else
    die "二进制包校验失败（可能是下载被截断）。
    期望 ${BIN_SHA256}
    实际 ${ACTUAL}
    请重试；反复失败可以手动下载后用 --bin 指定本地文件。"
  fi
fi

# 解包：本地包在 dry-run 下也真解（只解到临时目录），提前暴露坏包
if [ -f "$TMP/bin.tar.gz" ]; then
  tar xzf "$TMP/bin.tar.gz" -C "$TMP" || die "二进制包解不开：${BIN_LOCAL:-下载得到的 bin.tar.gz}"
  [ -f "$TMP/bfserver" ] || die "二进制包里没有 bfserver，请确认是官方发布包（--no-sha256 也不认其他结构）"
  chmod 0755 "$TMP/bfserver" 2>/dev/null || true
  ok "二进制包已解压"
else
  say "    [dry-run] 解压二进制包 → bfserver"
fi

SRC_ROOT="$TMP/bf-cn0261-go-server-${VERSION}"
if [ -f "$TMP/src.tar.gz" ]; then
  tar tzf "$TMP/src.tar.gz" > "$TMP/src.list" 2>/dev/null || die "源码包解不开：${SRC_LOCAL:-下载得到的 src.tar.gz}"
  tar xzf "$TMP/src.tar.gz" -C "$TMP" || die "源码包解压失败：${SRC_LOCAL:-下载得到的 src.tar.gz}"
  FIRST=$(head -1 "$TMP/src.list" | cut -d/ -f1)
  SRC_ROOT=""
  for cand in "$TMP/$FIRST" "$TMP"; do   # GitHub 归档有顶层目录；自己打的包可能没有
    if [ -d "$cand/config" ] && [ -d "$cand/database" ]; then SRC_ROOT="$cand"; break; fi
  done
  [ -n "$SRC_ROOT" ] || die "源码包里找不到 config/ 和 database/ 目录，请确认是 v${VERSION} 的源码包"
  ok "源码包已解压：${SRC_ROOT}"
else
  say "    [dry-run] 解压源码包 → $SRC_ROOT"
fi

# ============================================================================
# 5. 创建数据库
# ============================================================================
step 5 "创建数据库与账号"
SQL=$(cat <<SQL
CREATE DATABASE IF NOT EXISTS \`${DB_NAME}\`
  CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS '${DB_USER}'@'127.0.0.1' IDENTIFIED BY '${DB_PASS}';
CREATE USER IF NOT EXISTS '${DB_USER}'@'localhost' IDENTIFIED BY '${DB_PASS}';
ALTER USER '${DB_USER}'@'127.0.0.1' IDENTIFIED BY '${DB_PASS}';
ALTER USER '${DB_USER}'@'localhost' IDENTIFIED BY '${DB_PASS}';
GRANT ALL PRIVILEGES ON \`${DB_NAME}\`.* TO '${DB_USER}'@'127.0.0.1';
GRANT ALL PRIVILEGES ON \`${DB_NAME}\`.* TO '${DB_USER}'@'localhost';
FLUSH PRIVILEGES;
SQL
)
if [ "$DRY_RUN" = "1" ]; then
  say "    [dry-run] 创建库 ${DB_NAME}、账号 ${DB_USER}@127.0.0.1"
else
  printf '%s\n' "$SQL" | mariadb >>"$LOG_FILE" 2>&1 || die "建库失败，日志：${LOG_FILE}"
fi
ok "数据库就绪：${DB_NAME} / ${DB_USER}"

# ============================================================================
# 6. 先定数据库怎么处理，再部署文件
# ============================================================================
# 顺序很重要：数据库没搞定之前不动 $PREFIX 里的任何东西，失败也不会留下半装的现场。
step 6 "检查数据库并部署文件"

# 期望表清单来自本次发布自带的基线，而不是写死的数字
# 干跑时如果没指定本地源码包（还没下载），基线不在本地，就跳过着这一步
if [ -f "$SRC_ROOT/database/baseline.sql" ]; then
  EXPECTED_TABLES=$(sed -n 's/^CREATE TABLE `\([^`]*\)`.*/\1/p' "$SRC_ROOT/database/baseline.sql" | LC_ALL=C sort)
else
  EXPECTED_TABLES=""
fi
EXPECTED_COUNT=$(printf '%s\n' "$EXPECTED_TABLES" | grep -c . || true)
if [ -z "$EXPECTED_TABLES" ] && [ "$DRY_RUN" = "1" ]; then
  EXPECTED_COUNT="?"   # 干跑且没下载源码包：表数以基线文件为准，真跑时会重新算
elif [ -z "$EXPECTED_TABLES" ]; then
  die "源码包的 database/baseline.sql 里没有 CREATE TABLE，包不完整"
fi

if [ "$DRY_RUN" = "1" ]; then
  say "    [dry-run] 检查 ${DB_NAME} 现有表：空库 → 导入基线；已是基线 → 跳过；旧结构 → 要求 --rebuild-db"
  DB_STATE="dry"
else
  TABLES_NOW=$(mariadb -N -B -e "SELECT table_name FROM information_schema.tables WHERE table_schema='${DB_NAME}';" 2>/dev/null || echo '?')
  if [ "$TABLES_NOW" = "?" ]; then
    db_names=""
    DB_STATE="unknown"
  else
    db_names=$(printf '%s\n' "$TABLES_NOW" | grep -c . || true)
    if [ "${db_names:-0}" = "0" ]; then
      DB_STATE="empty"
    else
      # comm 要跟 sort 用同一个排序：本机可能是 UTF-8 locale，得显式 LC_ALL=C，
      # 否则会报 "file 1 is not in sorted order"
      MISSING_TABLES=$(LC_ALL=C comm -23 <(printf '%s\n' "$EXPECTED_TABLES") \
        <(printf '%s\n' "$TABLES_NOW" | LC_ALL=C sort))
      if [ -z "$(printf '%s' "$MISSING_TABLES" | tr -d ' ')" ]; then
        DB_STATE="baseline"
      else
        DB_STATE="other"
      fi
    fi
  fi
fi

case "$DB_STATE" in
  empty)    DB_ACTION="import" ;;
  baseline) DB_ACTION="skip" ;;
  other)    DB_ACTION="rebuild" ;;
  dry)      DB_ACTION="dry" ;;
  *)        DB_ACTION="unknown" ;;
esac

if [ "$DB_ACTION" = "unknown" ]; then
  die "读不到数据库 ${DB_NAME} 的现状（是否已启动？），为避免误操作已停止"
fi

if [ "$DB_ACTION" = "rebuild" ] && [ "$REBUILD_DB" != "1" ]; then
  missing_count=$(printf '%s\n' "$MISSING_TABLES" | grep -c . || true)
  missing_preview=$(printf '%s\n' "$MISSING_TABLES" | head -5 | tr '\n' ' ')
  cat <<EOF

    数据库 ${DB_NAME} 里已经有数据，但结构和本版（baseline revision 1，${EXPECTED_COUNT} 张表）不一样：
      缺 ${missing_count} 张表：${missing_preview}$([ "$missing_count" -gt 5 ] && echo '…')

    本版不做原地升级——旧表结构和现在的服务端对不上，直接覆盖又会让账号、角色、道具丢干净。
    确认要重建（数据会全部丢失）就跑：
      bash $0 --rebuild-db
    脚本会先把现有库备份到 /opt/bf-backup-${DB_NAME}-<时间>.sql.gz，再重建。
    只想先看看现状：mariadb -e "SHOW TABLES FROM \`${DB_NAME}\`"
EOF
  die "数据库结构与本版不符，已停止（${PREFIX} 未做任何改动）"
fi

# 重建前先备份：备份失败就不往下走
if [ "$DB_ACTION" = "rebuild" ]; then
  DB_BACKUP="/opt/bf-backup-${DB_NAME}-$(date +%Y%m%d-%H%M%S).sql.gz"
  DUMP_BIN=mysqldump
  command -v mysqldump >/dev/null 2>&1 || DUMP_BIN=mariadb-dump
  if [ "$DRY_RUN" = "1" ]; then
    say "    [dry-run] ${DUMP_BIN} 备份 ${DB_NAME} → ${DB_BACKUP}"
    say "    [dry-run] 导入 database/baseline.sql（--force，现有 ${db_names} 张表会被清空）"
  else
    $DUMP_BIN --single-transaction --routines "$DB_NAME" 2>>"$LOG_FILE" | gzip > "$DB_BACKUP" \
      || die "备份失败，未做任何改动。检查磁盘空间与 ${DUMP_BIN} 权限，日志：${LOG_FILE}"
    [ -s "$DB_BACKUP" ] || die "备份文件是空的（${DB_BACKUP}），为安全起见已停止"
    ok "旧库已备份：${DB_BACKUP}（$(du -h "$DB_BACKUP" | awk '{print $1}')）"
  fi
fi

# 建库：仓库自带的 migrate.sh 导入 database/baseline.sql（schema revision 1）
case "$DB_ACTION" in
  skip)
    info "库结构已是本版基线（${EXPECTED_COUNT} 张表），跳过导入，数据原样保留"
    ;;
  import)
    if [ "$DRY_RUN" = "1" ]; then
      say "    [dry-run] 导入 database/baseline.sql（schema revision 1，${EXPECTED_COUNT} 张表）"
    else
      ( BF_DB_USER="$DB_USER" BF_DB_PASSWORD="$DB_PASS" BF_DB_NAME="$DB_NAME" \
          bash "$SRC_ROOT/database/migrate.sh" 127.0.0.1 3306 ) >>"$LOG_FILE" 2>&1 \
        || die "数据库初始化失败（详情见 ${LOG_FILE}）"
    fi
    ;;
  rebuild)
    if [ "$DRY_RUN" = "0" ]; then
      ( BF_DB_USER="$DB_USER" BF_DB_PASSWORD="$DB_PASS" BF_DB_NAME="$DB_NAME" \
          bash "$SRC_ROOT/database/migrate.sh" 127.0.0.1 3306 --force ) >>"$LOG_FILE" 2>&1 \
        || die "数据库重建失败（备份在 ${DB_BACKUP}，详情见 ${LOG_FILE}）"
    fi
    ;;
  dry)
    if [ "$REBUILD_DB" = "1" ]; then
      say "    [dry-run] 会先备份现有库，再用 --force 重建为 ${EXPECTED_COUNT} 张表的基线"
    else
      say "    [dry-run] 真跑时：空库导入基线 / 已是基线跳过 / 旧结构要求 --rebuild-db"
    fi
    ;;
esac

if [ "$DRY_RUN" = "0" ]; then
  REVISION=$(mariadb -N -B -e "SELECT revision FROM \`${DB_NAME}\`.schema_version WHERE id = 1;" 2>/dev/null || echo '?')
  TABLES=$(mariadb -N -B -e "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='${DB_NAME}';" 2>/dev/null || echo '?')
  ok "数据库就绪（${TABLES} 张表，schema revision ${REVISION}）"
fi

# ---- 数据库确定无误，这时才动磁盘上的文件 ----
run "mkdir -p '$PREFIX'"
if [ -f "${PREFIX}/bfserver" ]; then
  run "cp -f '${PREFIX}/bfserver' '${PREFIX}/bfserver.prev'"   # 回退用
fi
run "install -m 0755 '$TMP/bfserver' '$PREFIX/bfserver'"

# 配置：整目录先备份，随版本走的 JSON 表覆盖，server.json 与证书保留（第 7 步只补字段）
if [ -d "${PREFIX}/config" ]; then
  CFG_BACKUP="${PREFIX}/config.bak.$(date +%Y%m%d-%H%M%S)"
  run "cp -a '${PREFIX}/config' '${CFG_BACKUP}'"
  [ "$DRY_RUN" = "1" ] || info "旧配置已备份：${CFG_BACKUP}"
fi
run "mkdir -p '$PREFIX/config'"
if [ -d "$SRC_ROOT/config" ]; then
  for cfg_file in "$SRC_ROOT"/config/*.json; do
    [ "$(basename "$cfg_file")" = "server.json" ] && continue   # server.json 由第 7 步生成/补字段
    run "cp -f '$cfg_file' '$PREFIX/config/$(basename "$cfg_file")'"
  done
  if [ ! -f "${PREFIX}/config/server.json" ]; then
    run "cp -f '$SRC_ROOT/config/server.json' '$PREFIX/config/server.json'"
  fi
else
  say "    [dry-run] 覆盖 config/ 里随版本走的 JSON，保留 server.json 与证书"   # 干跑且无本地源码包时
fi
run "rm -rf '$PREFIX/database'"
run "cp -r '$SRC_ROOT/database' '$PREFIX/database'"
run "mkdir -p '$PREFIX/Logs'"

# ============================================================================
# 7. 生成配置 + 证书
# ============================================================================
step 7 "生成配置与自签证书"
# 旧配置已在第 6 步整目录备份（config.bak.<时间>），这里不再单独备份 server.json

if [ "$DRY_RUN" = "1" ]; then
  say "    [dry-run] 写入 config/server.json（数据库密码、公网 IP、邀请码；已有配置只补字段，不覆盖你改过的项）"
  say "    [dry-run] 缺证书时生成 26102 认证端自签证书（SAN: ${PUBLIC_IP}）"
else
  BF_CFG_MODE="$MODE" BF_DB_PASS="$DB_PASS" BF_PUBLIC_IP="$PUBLIC_IP" BF_INVITE="$INVITE_CODE" \
  python3 - "$PREFIX/config/server.json" <<'PY'
import json, os, sys
path = sys.argv[1]
update = os.environ.get("BF_CFG_MODE") == "update"
cfg = json.load(open(path, encoding="utf-8"))

# 数据库连接是脚本自己的领地，两种模式都写死。
cfg.setdefault("database", {})
cfg["database"].update({
    "host": "127.0.0.1",
    "port": 3306,
    "databaseName": "bubblefighter_cn0261",
    "username": "bfserver",
    "password": os.environ["BF_DB_PASS"],   # 直接写进配置文件，启动时不依赖环境变量
    "tlsMode": "disable",
})

# 端口/日志级别这类用户可能手改过的项：更新时只补缺失字段，不恢复成默认值。
def fill(section, defaults):
    cfg.setdefault(section, {})
    for key, value in defaults.items():
        if update:
            cfg[section].setdefault(key, value)
        else:
            cfg[section][key] = value

fill("network", {"listenAddress": "0.0.0.0", "listenPort": 26100})
fill("relay", {"listenAddress": "0.0.0.0", "listenPort": 26104})
# 关键：留 127.0.0.1 的话，远程客户端拿到的中继地址是本机回环地址，进对局会卡住
cfg["relay"]["advertisedAddress"] = os.environ["BF_PUBLIC_IP"]

fill("authentication", {
    "enabled": True,
    "listenAddress": "0.0.0.0",
    "listenPort": 26102,
})
# 证书路径、对外主机名、邀请码跟安装参数走
cfg["authentication"].update({
    "certificateFile": "config/authentication_certificate.pem",
    "privateKeyFile": "config/authentication_private_key.pem",
    "gameHost": os.environ["BF_PUBLIC_IP"],
    "inviteCode": os.environ["BF_INVITE"],
})

fill("logging", {"level": "info", "directory": "Logs"})

json.dump(cfg, open(path, "w", encoding="utf-8"), ensure_ascii=False, indent=2)
print("config written")
PY
  chmod 600 "$PREFIX/config/server.json"
  # 认证端（26102）用自签证书：客户端不校验证书链，自签即可
  # 更新时已有证书就沿用：换证书会让已登录的客户端下次连接时握手失败
  if [ -f "${PREFIX}/config/authentication_certificate.pem" ] && [ -f "${PREFIX}/config/authentication_private_key.pem" ]; then
    info "沿用已有证书（${PREFIX}/config/authentication_certificate.pem）"
  else
    ( cd "$PREFIX" && openssl req -x509 -newkey rsa:2048 -nodes -sha256 -days 3650 \
        -keyout config/authentication_private_key.pem \
        -out config/authentication_certificate.pem \
        -subj "/CN=bfserver-cn0261" \
        -addext "subjectAltName=IP:${PUBLIC_IP},IP:127.0.0.1,DNS:localhost" ) >>"$LOG_FILE" 2>&1 \
      || die "证书生成失败，日志：${LOG_FILE}"
  fi
  chmod 600 "$PREFIX/config/authentication_private_key.pem"
  ok "配置与证书已就绪"
fi

# ============================================================================
# 8. 注册服务并启动
# ============================================================================
step 8 "注册系统服务并启动"
if ! id "$SERVICE" >/dev/null 2>&1; then
  run "useradd --system --no-create-home --shell /usr/sbin/nologin '$SERVICE'"
fi
run "chown -R ${SERVICE}:${SERVICE} '$PREFIX'"
run "chmod 600 '$PREFIX/config/server.json'"

UNIT_CONTENT="[Unit]
Description=BubbleFighter CN0261 server
After=network-online.target mariadb.service
Wants=network-online.target

[Service]
Type=simple
User=${SERVICE}
Group=${SERVICE}
WorkingDirectory=${PREFIX}
ExecStart=${PREFIX}/bfserver --config config/server.json
Restart=always
RestartSec=5
LimitNOFILE=65535
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=full
ProtectHome=true
ReadWritePaths=${PREFIX}/Logs

[Install]
WantedBy=multi-user.target
"
if [ "$DRY_RUN" = "1" ]; then
  say "    [dry-run] 写入 ${UNIT} 并 systemctl enable --now ${SERVICE}"
else
  printf '%s' "$UNIT_CONTENT" > "$UNIT"
  systemctl daemon-reload
  systemctl enable "$SERVICE" >>"$LOG_FILE" 2>&1
  systemctl restart "$SERVICE" >>"$LOG_FILE" 2>&1 || true
  sleep 3
  if ! systemctl is-active --quiet "$SERVICE"; then
    say ""
    say "    服务没能启动，最近日志："
    tail -20 "$PREFIX/Logs/Server.log" 2>/dev/null | sed 's/^/      /' || journalctl -u "$SERVICE" -n 20 --no-pager | sed 's/^/      /'
    die "启动失败。常见原因：端口被占用、数据库密码不对、配置文件格式错误。"
  fi
  ok "服务已启动"
fi

if [ "$DRY_RUN" = "0" ]; then
  PORT_OK=0
  for p in 26100 26102 26104; do
    if ss -ltn 2>/dev/null | awk '{print $4}' | grep -q ":${p}\$"; then
      PORT_OK=$((PORT_OK+1))
    else
      warn "端口 ${p} 没在监听"
    fi
  done
  [ "$PORT_OK" = "3" ] && ok "三个端口都在监听（26100 / 26102 / 26104）"

  umask 077
  {
    echo "# 泡泡战士 CN0261 服务端 · 安装信息（$(date '+%Y-%m-%d %H:%M:%S')）"
    echo "PUBLIC_IP=${PUBLIC_IP}"
    echo "DB_NAME=${DB_NAME}"
    echo "DB_USER=${DB_USER}"
    echo "DB_PASSWORD=${DB_PASS}"
    echo "INVITE_CODE=${INVITE_CODE}"
    echo "CONFIG=${PREFIX}/config/server.json"
  } > "$SECRET_FILE"
  chmod 600 "$SECRET_FILE"
fi

# ============================================================================
# 完成
# ============================================================================
say ""
say "${c_g}${c_b}────────────────────────────────────────────────${c_reset}"
say "${c_g}${c_b} ✅ 安装完成，服务已在后台运行${c_reset}"
say "${c_g}${c_b}────────────────────────────────────────────────${c_reset}"
say " 公网 IP    : ${c_b}${PUBLIC_IP}${c_reset}"
say " 端口       : 游戏 26100 · 认证 26102 · 中继 26104"
say " 数据库     : ${DB_NAME} / 用户 ${DB_USER}"
say_secret " ${c_y}注册邀请码 : ${INVITE_CODE}${c_reset}   ← 玩家用登录器注册时要填这个"
say " 配置文件   : ${PREFIX}/config/server.json（权限 600，数据库密码在里面）"
say " 安装记录   : ${SECRET_FILE}（数据库密码 + 邀请码，别外传）"
say " 运行日志   : ${PREFIX}/Logs/Server.log"
say ""
say " ${c_b}常用命令${c_reset}"
say "   看状态     systemctl status ${SERVICE}"
say "   重启       systemctl restart ${SERVICE}"
say "   看实时日志 tail -f ${PREFIX}/Logs/Server.log"
say "   建测试账号 ${PREFIX}/bfserver --config config/server.json --create-account 你的账号名 --nickname 角色名"
say "   卸载       bash $0 --uninstall"
say ""
say " ${c_b}管理后台（WebUI，可选）${c_reset}"
say "   现在是${c_y}关闭${c_reset}的：仓库里公开过的示例 token 不应该出现在任何人的机器上。"
say "   要用就编辑 ${PREFIX}/config/server.json 的 admin 段："
say "     \"enabled\": true                                  ← 打开"
say "     \"token\":   \"<用 openssl rand -hex 32 生成一个>\"        ← 必须自己填，否则拒绝启动"
say "   改完 systemctl restart ${SERVICE}，面板在 ${c_b}http://127.0.0.1:26110/${c_reset}"
say "   默认只绑 127.0.0.1；远程看请用 SSH 隧道，不要把 listenAddress 改成 0.0.0.0："
say "     ssh -L 26110:127.0.0.1:26110 你的用户@这台服务器"
say "   开启步骤与安全说明：仓库 docs/admin.md"
say ""
say " ${c_y}${c_b}⚠ 还差一步（脚本替你做不了）：${c_reset}"
say "   到云厂商控制台的「安全组 / 防火墙」放行 ${c_b}26100、26102、26104${c_reset} 三个 TCP 端口。"
say "   只放行 26100 的话：能登录、能进广场，但进对局会一直转圈。"
say ""
