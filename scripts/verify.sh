#!/usr/bin/env bash
# 统一本地验证门槛：后端编译/静态检查/测试 + 前端类型检查/构建。
# 用法：bash scripts/verify.sh   （任一步失败即非零退出）
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

echo "==> [1/5] go build ./..."
( cd backend && go build ./... )

echo "==> [2/5] go vet ./..."
( cd backend && go vet ./... )

echo "==> [3/5] go test ./..."
( cd backend && go test ./... )

echo "==> [4/5] frontend typecheck (tsc --noEmit)"
( cd frontend && node ./node_modules/typescript/bin/tsc --noEmit )

echo "==> [5/5] frontend build (vite)"
( cd frontend && node ./node_modules/vite/bin/vite.js build )

echo ""
echo "✅ verify 通过：后端编译/vet/测试 + 前端类型检查/构建 全部成功"
