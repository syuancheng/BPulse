#!/bin/sh
set -eu

mkdir -p bin
go build -o bin/bp-api ./server/cmd/api
go build -o bin/bp-migrate ./server/cmd/migrate

node -e 'require("./media-parser/src/index")'
node -e 'require("./reminder-worker/src/index")'
node -e 'require("./miniprogram/config"); require("./miniprogram/services/api")'

echo "Go commands and JavaScript skeletons built successfully"
