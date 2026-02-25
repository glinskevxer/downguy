#!/usr/bin/env bash
# race.sh
#
# Runs the suite under the race detector. The detector needs cgo and cgo needs
# a C compiler, and a build machine without one is common enough that failing
# there would mean nobody runs this locally at all. So the script says plainly
# which it did: it runs the detector where it can and reports a skip where it
# cannot, rather than passing silently either way.
set -euo pipefail

if [ "${RACE_REQUIRED:-0}" = "1" ]; then
    required=1
else
    required=0
fi

compiler=$(go env CC)
if ! command -v "$compiler" > /dev/null 2>&1; then
    if [ "$required" = "1" ]; then
        echo "race: RACE_REQUIRED is set and there is no C compiler ($compiler)" >&2
        exit 1
    fi
    echo "race: skipped, no C compiler ($compiler) for cgo"
    exit 0
fi

echo "race: running under the detector with $compiler"
CGO_ENABLED=1 go test -race -count=1 ./tests/...
