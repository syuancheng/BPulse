#!/bin/sh
set -eu

unformatted=$(find server -name '*.go' -type f -exec gofmt -l {} +)
if [ -n "$unformatted" ]; then
  echo "Go files require formatting:"
  echo "$unformatted"
  exit 1
fi

git diff --check
