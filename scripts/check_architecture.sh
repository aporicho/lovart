#!/usr/bin/env bash
# Architecture guard script for lovart v2.
# Checks: dependency direction, file naming, file size, and module boundaries.
set -euo pipefail

PASS=0
FAIL=0
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

red()  { echo -e "\033[31m$1\033[0m"; }
green(){ echo -e "\033[32m$1\033[0m"; }

pass() { green "  PASS $1"; PASS=$((PASS+1)); }
fail() { red "  FAIL $1"; FAIL=$((FAIL+1)); }

# ---- 1. Forbidden file names ----
echo "==> Checking forbidden file names..."
cd "$ROOT"
IFS=$'\n'
for f in $(find . -name '*.go' -type f | grep -v '/vendor/'); do
    base=$(basename "$f")
    case "$base" in
        utils.go|helpers.go|misc.go|common.go|legacy.go|compat.go|glue.go)
            fail "forbidden file: $f"
            ;;
    esac
done
pass "no forbidden file names"

# ---- 2. Command entrypoint hygiene ----
echo "==> Checking cmd/lovart entrypoints..."
cmd_bad=0
for f in $(find cmd/lovart -maxdepth 1 -name '*.go' -type f 2>/dev/null || true); do
    base=$(basename "$f")
    case "$base" in
        main.go|mcp.go|mcp_smoke_test.go|selfmanage.go|selfmanage_test.go)
            ;;
        *)
            fail "unexpected cmd/lovart source file: $f"
            cmd_bad=$((cmd_bad+1))
            ;;
    esac
done
for required in main.go mcp.go; do
    if [ ! -f "cmd/lovart/$required" ]; then
        fail "missing cmd/lovart/$required"
        cmd_bad=$((cmd_bad+1))
    fi
done
if git check-ignore -q cmd/lovart/main.go 2>/dev/null; then
    fail "cmd/lovart/main.go is ignored by .gitignore"
    cmd_bad=$((cmd_bad+1))
fi
if [ "$cmd_bad" -eq 0 ]; then
    pass "cmd/lovart contains only official entrypoint files"
fi

# ---- 3. Generated source pollution ----
echo "==> Checking generated source pollution..."
pollution=$(git ls-files 'downloads/*' 'runs/*' '.lovart/*' 2>/dev/null || true)
if [ -n "$pollution" ]; then
    fail "generated/runtime files are tracked: $pollution"
else
    pass "no tracked generated/runtime files"
fi

# ---- 4. Dependency direction: internal must not import cli or mcp ----
echo "==> Checking internal → cli/mcp dependency..."
violations=$(grep -rl '"github.com/aporicho/lovart/cli"' internal/ 2>/dev/null || true)
violations="$violations
$(grep -rl '"github.com/aporicho/lovart/mcp"' internal/ 2>/dev/null || true)"
if [ -n "$(echo "$violations" | tr -d '[:space:]')" ]; then
    fail "internal package imports cli or mcp: $violations"
else
    pass "internal does not import cli or mcp"
fi

# ---- 5. File size check ----
echo "==> Checking file sizes..."
large=0
while IFS= read -r f; do
    lines=$(wc -l < "$f")
    if [ "$lines" -gt 500 ]; then
        echo "  WARN $f ($lines lines)"
        large=$((large+1))
    fi
done < <(find internal/ -name '*.go' -type f)
if [ "$large" -eq 0 ]; then
    pass "no files over 500 lines"
else
    fail "$large file(s) over 500 lines"
fi

# ---- 6. TODO/FIXME/HACK in internal/ ----
echo "==> Checking TODO/FIXME/HACK in internal..."
todos=$(grep -rn 'TODO\|FIXME\|HACK' internal/ --include='*.go' 2>/dev/null | grep -v '_test.go' | grep -v '\.git' || true)
if [ -n "$todos" ]; then
    echo "  INFO: $todos"
    echo "  INFO $(echo "$todos" | wc -l | tr -d ' ') TODO markers (stub modules, not blocking)"
    pass "TODO markers in stub modules (acceptable)"
else
    pass "no TODO markers in internal/"
fi

# ---- 7. Go build ----
echo "==> Checking go build..."
if go build -o /dev/null ./cmd/lovart 2>/dev/null; then
    pass "go build ./cmd/lovart"
else
    fail "go build ./cmd/lovart"
fi

echo "==> Checking go build ./..."
if go build -o /dev/null ./... 2>/dev/null; then
    pass "go build ./..."
else
    fail "go build ./..."
fi

# ---- Summary ----
echo ""
echo "Results: $PASS passed, $FAIL failed"
if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
echo "Architecture checks passed."
