#!/bin/bash
set -eu
cd "$(dirname "$0")/.."
. deploy/docker/ai-output-language.sh
check() {
    local result
    result=$(TZ="$1" AGENT_OUTPUT_LANGUAGE="$2" liaison_ai_language "$3" </dev/null)
    [ "$result" = "$4" ] || { echo "unexpected language: $result" >&2; exit 1; }
}
check Asia/Shanghai '' '' zh
check Asia/Urumqi '' '' zh
check America/New_York '' '' en
check UTC '' '' en
check Asia/Tokyo '' '' en
check Asia/Shanghai en '' en
check UTC zh '' zh
check UTC en zh zh
check Asia/Shanghai zh en en
if AGENT_OUTPUT_LANGUAGE=invalid liaison_ai_language </dev/null; then exit 1; fi
echo 'PASS installation language selection and upgrade preservation'
