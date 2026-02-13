#!/usr/bin/env bash
# cover-floor.sh <profile> <floor percent>
#
# Fails the build when the total statement coverage falls under the floor. The
# floor is a whole percent because go tool cover reports one decimal place and
# arguing about a tenth of a percent of coverage is not an argument worth
# having in a pipeline.
set -euo pipefail

profile="${1:?usage: cover-floor.sh <profile> <floor>}"
floor="${2:?usage: cover-floor.sh <profile> <floor>}"

if [ ! -s "$profile" ]; then
    echo "cover-floor: $profile is missing or empty" >&2
    exit 1
fi

total=$(go tool cover -func="$profile" | awk '/^total:/ { gsub(/%/, "", $3); print $3 }')
if [ -z "$total" ]; then
    echo "cover-floor: could not read a total out of $profile" >&2
    exit 1
fi

whole=${total%%.*}
if [ "$whole" -lt "$floor" ]; then
    echo "cover-floor: $total% is under the $floor% floor" >&2
    exit 1
fi

echo "cover-floor: $total% clears the $floor% floor"
