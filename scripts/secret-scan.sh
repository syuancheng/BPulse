#!/bin/sh
set -eu

pattern='-----BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY-----|AKID[A-Za-z0-9]{16,}|WECHAT_APP_SECRET=[A-Za-z0-9_-]{16,}|DATA_ENCRYPTION_KEY_B64=[A-Za-z0-9+/]{20,}'
file_list=$(mktemp)
trap 'rm -f "$file_list"' EXIT INT TERM
git ls-files --cached --others --exclude-standard >"$file_list"

while IFS= read -r file; do
  case "$file" in
    scripts/secret-scan.sh) continue ;;
  esac
  if [ -f "$file" ] && grep -En -- "$pattern" "$file"; then
    echo "Potential committed secret found in $file"
    exit 1
  fi
done <"$file_list"

echo "secret pattern scan passed"
