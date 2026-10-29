#!/usr/bin/env bash
# Apply the CN0261 schema in the order the server verifies at startup
# (schema_migrations 1..14). Alphabetical file order breaks foreign keys, so the
# version is read out of each file instead.
#
# Usage:
#   BF_DB_USER=bfserver BF_DB_PASSWORD=secret ./database/migrate.sh [host] [port]
set -euo pipefail

host="${1:-127.0.0.1}"
port="${2:-3306}"
user="${BF_DB_USER:-bfserver}"
password="${BF_DB_PASSWORD:?BF_DB_PASSWORD is required}"
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

version_of() {
  python3 - "$1" <<'PY'
import re, sys
text = open(sys.argv[1], encoding='utf-8').read()
match = re.search(r"schema_migrations\s*\(version[^)]*\)\s*(?:VALUES|SELECT)\s*\(?\s*(\d+)\s*,\s*'", text, re.S)
print(match.group(1) if match else 999)
PY
}

ordered=$(
  for file in "$here"/migrations/*.sql; do
    printf '%s\t%s\n' "$(version_of "$file")" "$file"
  done | sort -n -k1,1
)

while IFS=$'\t' read -r version file; do
  printf 'applying v%-3s %s\n' "$version" "$(basename "$file")"
  mysql -h "$host" -P "$port" -u "$user" -p"$password" < "$file" > /dev/null
done <<< "$ordered"

echo "schema applied"
