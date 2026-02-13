#!/usr/bin/env bash
# tidy-check.sh
#
# Fails when go mod tidy would change go.mod or go.sum. A dependency that is in
# go.mod and not used, or used and not in go.mod, is the sort of thing that
# builds locally and then does not build in a clean checkout.
set -euo pipefail

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

cp go.mod "$tmp/go.mod"
cp go.sum "$tmp/go.sum"

go mod tidy

status=0
if ! diff -u "$tmp/go.mod" go.mod; then
    echo "tidy-check: go mod tidy changes go.mod" >&2
    status=1
fi
if ! diff -u "$tmp/go.sum" go.sum; then
    echo "tidy-check: go mod tidy changes go.sum" >&2
    status=1
fi

cp "$tmp/go.mod" go.mod
cp "$tmp/go.sum" go.sum

if [ "$status" -eq 0 ]; then
    echo "tidy-check: go.mod and go.sum are tidy"
fi
exit "$status"
