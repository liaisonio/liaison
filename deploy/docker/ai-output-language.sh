#!/bin/bash
# Sourced by installers only. No network requests or runtime locale detection.
liaison_ai_language() {
    local existing="${1:-}" zone="${TZ:-}" choice="${AGENT_OUTPUT_LANGUAGE:-}"
    # Upgrades preserve existing installation defaults, including legacy Chinese.
    if [ -n "$existing" ]; then
        choice="$existing"
    elif [ -z "$choice" ]; then
        if [ -z "$zone" ] && command -v timedatectl >/dev/null 2>&1; then
            zone=$(timedatectl show -p Timezone --value 2>/dev/null || true)
        fi
        if [ -z "$zone" ] && [ -r /etc/timezone ]; then zone=$(head -n 1 /etc/timezone); fi
        if [ -z "$zone" ]; then zone=$(readlink /etc/localtime 2>/dev/null || true); fi
        zone=${zone##*/zoneinfo/}
        zone=${zone#:}
        case "$zone" in
            Asia/Shanghai|Asia/Chongqing|Asia/Chungking|Asia/Harbin|Asia/Urumqi|PRC) choice=zh ;;
            *) choice=en ;;
        esac
        if [ -t 0 ]; then
            printf 'AI output language / AI 输出语言 [zh/en] (%s, timezone: %s): ' "$choice" "${zone:-unknown}" >&2
            local answer=""
            read -r answer || true
            choice=${answer:-$choice}
        fi
    fi
    case "$choice" in zh|en) printf '%s\n' "$choice" ;; *) printf 'AGENT_OUTPUT_LANGUAGE must be zh or en\n' >&2; return 1 ;; esac
}
