#!/usr/bin/env bash
# Guard capture directories from new large raw recordings and likely secrets.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

MAX_CAPTURE_BYTES=${MAX_CAPTURE_BYTES:-262144}
FAIL=0

tmp_paths=$(mktemp)
trap 'rm -f "$tmp_paths"' EXIT

add_diff_paths() {
    local range=$1
    git diff --name-only -z --diff-filter=AM "$range" -- 'captures/**' 'captures_backup/**' 2>/dev/null >> "$tmp_paths" || true
}

# Local pre-commit/pre-push use: inspect staged capture changes.
git diff --cached --name-only -z --diff-filter=AM -- 'captures/**' 'captures_backup/**' 2>/dev/null >> "$tmp_paths" || true

# CI use: inspect the pull request or push range instead of the whole historic tree.
if [ -n "${GITHUB_BASE_REF:-}" ]; then
    git fetch --no-tags --depth=1 origin "${GITHUB_BASE_REF}" >/dev/null 2>&1 || true
    base_ref="origin/${GITHUB_BASE_REF}"
    merge_base=$(git merge-base HEAD "$base_ref" 2>/dev/null || true)
    if [ -n "$merge_base" ]; then
        add_diff_paths "$merge_base...HEAD"
    else
        add_diff_paths "$base_ref...HEAD"
    fi
elif [ -n "${GITHUB_EVENT_BEFORE:-}" ] && [ "${GITHUB_EVENT_BEFORE}" != "0000000000000000000000000000000000000000" ]; then
    add_diff_paths "${GITHUB_EVENT_BEFORE}..HEAD"
fi

if [ ! -s "$tmp_paths" ]; then
    echo "Capture checks passed: no new or modified capture files in the checked diff."
    exit 0
fi

while IFS= read -r -d '' path; do
    case "$path" in
        captures/*|captures_backup/*) ;;
        *) continue ;;
    esac

    size=""
    if git cat-file -e ":$path" 2>/dev/null; then
        size=$(git cat-file -s ":$path" 2>/dev/null || true)
    elif [ -f "$path" ]; then
        size=$(wc -c < "$path" | tr -d ' ')
    fi

    if [ -n "$size" ] && [ "$size" -gt "$MAX_CAPTURE_BYTES" ]; then
        echo "FAIL capture file is too large ($size bytes > $MAX_CAPTURE_BYTES): $path"
        FAIL=$((FAIL+1))
        continue
    fi

    if [ -f "$path" ]; then
        if LC_ALL=C grep -Eiq '(^|[^A-Za-z])(authorization|proxy-authorization)[" ]*:[" ]*(bearer|basic)[[:space:]]+[A-Za-z0-9._~+/=-]{16,}' "$path"; then
            echo "FAIL capture file appears to contain an auth header: $path"
            FAIL=$((FAIL+1))
        elif LC_ALL=C grep -Eiq '(^|[^A-Za-z])(cookie|set-cookie|session|access[_-]?token|refresh[_-]?token|api[_-]?key)[" ]*[:=][" ]*[A-Za-z0-9._~+/=-]{24,}' "$path"; then
            echo "FAIL capture file appears to contain a secret-like value: $path"
            FAIL=$((FAIL+1))
        fi
    else
        echo "WARN cannot inspect missing worktree file content: $path"
    fi
done < <(sort -zu "$tmp_paths")

if [ "$FAIL" -gt 0 ]; then
    echo "Capture checks failed. Store sanitized fixtures outside captures/ or document an explicit exception."
    exit 1
fi

echo "Capture checks passed."
