#!/usr/bin/env bash
# smoke.sh <binary>
#
# Runs the built binary through one whole application: load a little plant,
# survey it, check the clearances, plan the make ready, price it and bill the
# year. It is here because every one of those steps passes its unit tests
# individually and the thing that actually breaks is one of them changing its
# output shape under another.
set -euo pipefail

bin="${1:?usage: smoke.sh <binary>}"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
db="$work/downguy.db"

run() {
    echo "+ $*" >&2
    "$@"
}

cat > "$work/poles.csv" <<'CSV'
pole_id,length_ft,class,species,owner,condition
NW-441829,40,4,SPF,SUMMIT VALLEY ELECTRIC,sound
NW-441830,40,4,SPF,SUMMIT VALLEY ELECTRIC,sound
CSV

cat > "$work/attachments.csv" <<'CSV'
attachment_id,pole_id,owner,kind,height,authorized,in_force_from,voltage
A1,NW-441829,SUMMIT VALLEY ELECTRIC,primary,31.5,y,2001-06-01,7620
A2,NW-441829,SUMMIT VALLEY ELECTRIC,neutral,28.5,y,2001-06-01,
A3,NW-441829,RIDGEPORT CABLEVISION,comm_cable,20.0,y,2019-04-01,
A7,NW-441829,MERIDIAN FIBER,fiber,19.6,y,2025-07-01,
CSV

cat > "$work/spans.csv" <<'CSV'
span_id,from_pole,to_pole,length_ft,cover
SP-1,NW-441829,NW-441830,200,road
CSV

cat > "$work/surveys.csv" <<'CSV'
survey_id,pole_id,taken,crew,condition,subject,height,method
SV-1,NW-441829,2025-05-06,CREW 41,sound,A1,31.5,laser
SV-1,NW-441829,2025-05-06,CREW 41,sound,A2,28.5,laser
SV-1,NW-441829,2025-05-06,CREW 41,sound,A3,20.0,laser
SV-1,NW-441829,2025-05-06,CREW 41,sound,A7,19.6,laser
CSV

run "$bin" init -db "$db"
run "$bin" import -db "$db" poles "$work/poles.csv"
run "$bin" import -db "$db" attachments "$work/attachments.csv"
run "$bin" import -db "$db" spans "$work/spans.csv"
run "$bin" import -db "$db" surveys "$work/surveys.csv"
run "$bin" pole -db "$db" -spans NW-441829
run "$bin" load -db "$db" -default-wire '2 ACSR SPARROW' NW-441829

# The clearance and make ready steps report findings, which is exit code 1 and
# is the expected answer on this plant rather than a failure.
set +e
"$bin" clear -db "$db" NW-441829
clear_code=$?
"$bin" makeready -db "$db" -json -applicant 'MERIDIAN FIBER' NW-441829 > "$work/plan.json"
plan_code=$?
set -e

if [ "$clear_code" -gt 1 ]; then
    echo "smoke: clear exited $clear_code" >&2
    exit 1
fi
if [ "$plan_code" -gt 1 ]; then
    echo "smoke: makeready exited $plan_code" >&2
    exit 1
fi

run "$bin" estimate -on 2025-06-01 -quote < "$work/plan.json"
run "$bin" packet < "$work/plan.json" || packet_code=$?
if [ "${packet_code:-0}" -gt 1 ]; then
    echo "smoke: packet exited ${packet_code}" >&2
    exit 1
fi

# The allocation reads the findings off the same document the plan came on.
# Handing it the plain clearance output instead is the join that has broken
# before, so the smoke run exercises the one that is meant to work.
run "$bin" allocate -on 2025-06-01 -findings "$work/plan.json" \
    -owner 'SUMMIT VALLEY ELECTRIC=OWN-1' \
    -applicant 'MERIDIAN FIBER=ATT-9' \
    -attacher 'RIDGEPORT CABLEVISION=ATT-3' < "$work/plan.json"

run "$bin" rate -explain 2025
run "$bin" bill -db "$db" -issued 2026-01-15 2025
run "$bin" sweep -db "$db" -workers 2 || sweep_code=$?
if [ "${sweep_code:-0}" -gt 1 ]; then
    echo "smoke: sweep exited ${sweep_code}" >&2
    exit 1
fi
run "$bin" maintain -db "$db"
run "$bin" serve -db "$db" -check

echo "smoke: the whole application ran"
