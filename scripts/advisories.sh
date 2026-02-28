#!/usr/bin/env bash
# advisories.sh
#
# Runs the vulnerability scanner and judges what it found against the written
# acceptances in security-advisories.json.
#
# The rule is not "no advisories". It is: nothing outside the standard library,
# and nothing inside it either unless somebody has read it, written down why
# this program is not affected, and put a date on when that has to be looked at
# again. When that date passes the build goes red, which is the point.
set -euo pipefail

register="${REGISTER:-security-advisories.json}"
today="${TODAY:-$(date -u +%Y-%m-%d)}"

if [ ! -s "$register" ]; then
    echo "advisories: $register is missing" >&2
    exit 1
fi

if ! command -v govulncheck > /dev/null 2>&1; then
    echo "advisories: govulncheck is not installed" >&2
    exit 1
fi

report=$(mktemp)
trap 'rm -f "$report"' EXIT

# govulncheck exits non zero when it finds anything, which is the ordinary
# case here, so its status is not the signal. The findings are.
set +e
govulncheck -json ./... > "$report"
scanner=$?
set -e
if [ ! -s "$report" ]; then
    echo "advisories: the scanner produced nothing (exit $scanner)" >&2
    exit 1
fi

python3 - "$report" "$register" "$today" <<'PY'
import json, sys

report_path, register_path, today = sys.argv[1], sys.argv[2], sys.argv[3]

with open(register_path) as f:
    register = json.load(f)

accepted = {}
for entry in register.get("acceptances", []):
    scope = entry.get("scope")
    if not scope:
        print("advisories: an acceptance with no scope", file=sys.stderr)
        sys.exit(1)
    for field in ("reason", "assessed", "assessed_by", "review_by"):
        if not entry.get(field):
            print(f"advisories: the {scope} acceptance is missing {field}", file=sys.stderr)
            sys.exit(1)
    if not entry.get("checked"):
        print(f"advisories: the {scope} acceptance records nothing that was checked", file=sys.stderr)
        sys.exit(1)
    if entry["review_by"] < today:
        print(f"advisories: the {scope} acceptance was due for review on {entry['review_by']}", file=sys.stderr)
        sys.exit(1)
    accepted[scope] = entry

# The report is a stream of json objects, one per line group. Decode it with a
# raw decoder so a concatenated stream reads the same as a json lines one.
decoder = json.JSONDecoder()
text = open(report_path).read()
messages, at = [], 0
while at < len(text):
    while at < len(text) and text[at] in " \t\r\n":
        at += 1
    if at >= len(text):
        break
    value, at = decoder.raw_decode(text, at)
    messages.append(value)

called = {}
for message in messages:
    finding = message.get("finding")
    if not finding or not finding.get("trace"):
        continue
    # A finding with a function on the top frame is one the scanner traced into
    # this program's own call graph. Anything else is a module the build
    # requires and never reaches.
    top = finding["trace"][0]
    if not top.get("function"):
        continue
    module = top.get("module") or "unknown"
    called.setdefault(module, set()).add(finding["osv"])

outside = {m: ids for m, ids in called.items() if m != "stdlib"}
if outside:
    for module, ids in sorted(outside.items()):
        print(f"advisories: {module} is affected by {', '.join(sorted(ids))}", file=sys.stderr)
    sys.exit(1)

stdlib = called.get("stdlib", set())
if stdlib and "stdlib" not in accepted:
    print(f"advisories: {len(stdlib)} standard library advisories with no acceptance", file=sys.stderr)
    sys.exit(1)

if stdlib:
    entry = accepted["stdlib"]
    print(f"advisories: {len(stdlib)} standard library advisories accepted until {entry['review_by']}")
else:
    print("advisories: nothing reachable")
PY
