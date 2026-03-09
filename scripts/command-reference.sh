#!/usr/bin/env bash
# command-reference.sh [binary]
#
# Checks that the command table in the README says the same thing the binary
# does. Documentation that drifts from the program is worse than none: it is
# the thing somebody reads instead of running the help.
set -euo pipefail

bin="${1:-bin/downguy}"
readme="${README:-README.md}"

if [ ! -x "$bin" ]; then
    echo "command-reference: $bin is not there, run make build first" >&2
    exit 1
fi
if [ ! -s "$readme" ]; then
    echo "command-reference: $readme is missing" >&2
    exit 1
fi

actual=$("$bin" help | awk '/^commands:/ { on = 1; next } /^$/ { on = 0 } on { print $1 }' | sort)
documented=$(awk -F'|' '/^\| `[a-z]+` \|/ { gsub(/[ `]/, "", $2); print $2 }' "$readme" | sort)

if [ -z "$actual" ]; then
    echo "command-reference: the binary listed no commands" >&2
    exit 1
fi
if [ -z "$documented" ]; then
    echo "command-reference: the readme documents no commands" >&2
    exit 1
fi

missing=$(comm -23 <(echo "$actual") <(echo "$documented"))
extra=$(comm -13 <(echo "$actual") <(echo "$documented"))

status=0
if [ -n "$missing" ]; then
    echo "command-reference: the readme does not document:" >&2
    echo "$missing" | sed 's/^/  /' >&2
    status=1
fi
if [ -n "$extra" ]; then
    echo "command-reference: the readme documents commands that do not exist:" >&2
    echo "$extra" | sed 's/^/  /' >&2
    status=1
fi

if [ "$status" -eq 0 ]; then
    echo "command-reference: $(echo "$actual" | wc -l | tr -d ' ') commands, all documented"
fi
exit "$status"
