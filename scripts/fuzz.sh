#!/usr/bin/env bash
# fuzz.sh [seconds per target]
#
# Runs every fuzz target for a short burst. The seed corpus already runs as an
# ordinary test on every build, so this is the extra pass: a few thousand
# generated inputs per parser, which is where the input nobody thought of
# comes from.
set -euo pipefail

seconds="${1:-10}"

targets=$(grep -ho '^func Fuzz[A-Za-z0-9_]*' tests/*_test.go | sed 's/^func //' | sort)
if [ -z "$targets" ]; then
    echo "fuzz: there are no fuzz targets" >&2
    exit 1
fi

for target in $targets; do
    echo "fuzz: $target for ${seconds}s"
    go test ./tests/ -run "^${target}\$" -fuzz "^${target}\$" -fuzztime "${seconds}s"
done

echo "fuzz: $(echo "$targets" | wc -l | tr -d ' ') targets, no failures"
