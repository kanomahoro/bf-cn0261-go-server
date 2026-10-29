#!/usr/bin/env bash
# 生成认证端（26102）用的自签 TLS 证书。
#
# 生成的正是 config/server.json 里 authentication 段指向的两个文件：
#   certificateFile: config/authentication_certificate.pem
#   privateKeyFile : config/authentication_private_key.pem
#
# 用法：
#   ./gen-cert.sh                          # localhost + 127.0.0.1，有效期 3650 天
#   DAYS=365 ./gen-cert.sh                 # 自定义有效期（天）
#   HOSTS="bf.example.com 203.0.113.10" ./gen-cert.sh    # 追加客户端会访问的域名/IP
#   FORCE=1 ./gen-cert.sh                  # 已存在时强制覆盖
#
# 客户端不校验证书链，所以自签即可；SAN 只是为了规范，填错不影响登录。
set -euo pipefail
cd "$(dirname "$0")"

CERT="config/authentication_certificate.pem"
KEY="config/authentication_private_key.pem"
DAYS="${DAYS:-3650}"
HOSTS="${HOSTS:-}"
FORCE="${FORCE:-0}"

if ! command -v openssl >/dev/null 2>&1; then
  echo "未找到 openssl，请先安装（Debian/Ubuntu: apt install openssl）" >&2
  exit 1
fi

if [ -f "$CERT" ] || [ -f "$KEY" ]; then
  if [ "$FORCE" != "1" ]; then
    echo "证书已存在，未做任何改动：" >&2
    [ -f "$CERT" ] && echo "  $CERT" >&2
    [ -f "$KEY" ] && echo "  $KEY" >&2
    echo "如需重新生成：FORCE=1 $0   （会覆盖现有证书，已部署的客户端需要重新取证书）" >&2
    exit 1
  fi
  echo "FORCE=1：覆盖现有证书"
fi

mkdir -p config

# 组装 subjectAltName：默认 localhost + 127.0.0.1，再追加 HOSTS 里的域名/IP
SAN="DNS:localhost,IP:127.0.0.1"
for h in ${HOSTS//,/ }; do
  if [[ "$h" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    SAN="$SAN,IP:$h"
  else
    SAN="$SAN,DNS:$h"
  fi
done

SUBJ="/CN=BubbleFighter CN0261 Auth"
# -addext 需要 OpenSSL 1.1.1+；老版本退回到临时配置文件方式
if openssl req -help 2>&1 | grep -q -- "-addext"; then
  openssl req -x509 -newkey rsa:2048 -nodes -days "$DAYS" \
    -keyout "$KEY" -out "$CERT" \
    -subj "$SUBJ" -addext "subjectAltName=$SAN" 2>/dev/null
else
  TMP_CONF="$(mktemp)"
  trap 'rm -f "$TMP_CONF"' EXIT
  cat >"$TMP_CONF" <<EOF
[req]
distinguished_name = dn
x509_extensions    = v3
prompt             = no
[dn]
CN = BubbleFighter CN0261 Auth
[v3]
subjectAltName = $SAN
EOF
  openssl req -x509 -newkey rsa:2048 -nodes -days "$DAYS" \
    -keyout "$KEY" -out "$CERT" -config "$TMP_CONF" 2>/dev/null
fi

chmod 600 "$KEY"
chmod 644 "$CERT"

echo "已生成："
echo "  证书 $CERT"
echo "  私钥 $KEY  (权限 600，已在 .gitignore 中排除)"
echo "  SAN  $SAN"
echo "  有效期 ${DAYS} 天"
openssl x509 -in "$CERT" -noout -subject -dates | sed 's/^/  /'
echo
echo "确认 config/server.json 中 authentication.certificateFile / privateKeyFile 指向上面的文件，"
echo "并把 authentication.enabled 设为 true，然后启动服务端。"
