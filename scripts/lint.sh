#!/usr/bin/env bash
# lint.sh
#
# The project rules a compiler will not enforce. Each one is here because it
# was broken at least once and the breakage was invisible until something else
# went wrong:
#
#   - nothing under internal/ writes to the process's own streams, because a
#     command that reaches for os.Stdout cannot be tested without a subprocess
#   - nothing outside main calls os.Exit, for the same reason
#   - a panic belongs in a Must function or an init, nowhere else
#   - a rule id in a finding has to be a constant somewhere the catalogue knows
#   - no test file under internal/, because the tests live in tests/
set -euo pipefail

fail=0

report() {
    fail=1
    echo "lint: $1" >&2
    shift
    printf '  %s\n' "$@" >&2
}

# The command line owns the process's streams; nothing under internal/ else does.
offenders=$(grep -rn 'os\.Std\(out\|err\|in\)' internal/ --include='*.go' \
    | grep -v '^internal/cliapp/' || true)
if [ -n "$offenders" ]; then
    report "a package under internal/ reaches for the process's streams" $offenders
fi

offenders=$(grep -rnE 'os\.Exit\(' internal/ --include='*.go' || true)
if [ -n "$offenders" ]; then
    report "os.Exit belongs in main, not in a package" $offenders
fi

# A panic belongs in a function whose name says so, or in an init. Anywhere
# else it is an error somebody decided not to return. The check tracks the
# enclosing function rather than the line, because the panic is never on the
# same line as the signature.
offenders=$(find internal -name '*.go' -print0 | xargs -0 awk '
    /^func / {
        fn = $0
        sub(/^func \([^)]*\) /, "", fn)
        sub(/^func /, "", fn)
        sub(/[( ].*$/, "", fn)
        allowed = (fn ~ /^[Mm]ust/) || (fn == "init")
    }
    /panic\(/ && !allowed { printf "%s:%d\n", FILENAME, FNR }
')
if [ -n "$offenders" ]; then
    report "a panic outside a Must function or an init" $offenders
fi

offenders=$(find internal cmd -name '*_test.go' 2>/dev/null || true)
if [ -n "$offenders" ]; then
    report "the tests live in tests/, not beside the code" $offenders
fi

# Every rule id a finding can carry has to be in the catalogue, or a report
# cites something nobody can look up.
cited=$(grep -rho 'NESC-[0-9A-Z]*' internal/clearance internal/makeready internal/loading \
    --include='*.go' | sort -u)
catalogued=$(grep -o '"NESC-[0-9A-Z]*":' internal/nesc/rule.go | tr -d '":' | sort -u)
missing=$(comm -23 <(echo "$cited") <(echo "$catalogued") || true)
if [ -n "$missing" ]; then
    report "a rule is cited and not catalogued" $missing
fi

if [ "$fail" -eq 0 ]; then
    echo "lint: clean"
fi
exit "$fail"
