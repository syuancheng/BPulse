#!/bin/sh
set -eu

go vet ./...
npm --prefix media-parser run lint
npm --prefix reminder-worker run lint

find miniprogram -name '*.js' -type f -exec node --check {} \;
node -e 'for (const file of process.argv.slice(1)) JSON.parse(require("node:fs").readFileSync(file, "utf8"))' \
  miniprogram/app.json miniprogram/project.config.json miniprogram/sitemap.json \
  miniprogram/pages/home/index.json miniprogram/pages/bp-entry/index.json \
  media-parser/package.json reminder-worker/package.json

docker compose config --quiet
find scripts -name '*.sh' -type f -exec sh -n {} \;
