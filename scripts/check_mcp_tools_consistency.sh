#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 /path/to/lovart-binary" >&2
  exit 2
fi

binary="$1"
if [[ ! -x "$binary" ]]; then
  echo "lovart binary is not executable: $binary" >&2
  exit 2
fi

status_json="$($binary mcp status --clients none)"
tools_json="$(printf '{"jsonrpc":"2.0","id":1,"method":"tools/list"}\n' | "$binary" mcp)"

python3 - "$status_json" "$tools_json" <<'PY'
import json
import sys

status = json.loads(sys.argv[1])
tools_rpc = json.loads(sys.argv[2])

if not status.get("ok"):
    raise SystemExit(f"mcp status failed: {status}")
if "error" in tools_rpc:
    raise SystemExit(f"tools/list failed: {tools_rpc}")

status_tools = status.get("data", {}).get("tools", {})
status_count = status_tools.get("count")
status_names = status_tools.get("names") or []
list_tools = tools_rpc.get("result", {}).get("tools") or []
list_names = [tool.get("name") for tool in list_tools]

missing_tools = [name for name in status_names if name not in set(list_names)]
extra_tools = [name for name in list_names if name not in set(status_names)]

if status_count != len(status_names) or len(status_names) != len(list_names) or missing_tools or extra_tools or status_names != list_names:
    print("MCP tool catalog drift detected", file=sys.stderr)
    print(f"status_count={status_count}", file=sys.stderr)
    print(f"status_names_count={len(status_names)}", file=sys.stderr)
    print(f"tools_list_count={len(list_names)}", file=sys.stderr)
    print(f"missing_tools={missing_tools}", file=sys.stderr)
    print(f"extra_tools={extra_tools}", file=sys.stderr)
    print(f"status_names={status_names}", file=sys.stderr)
    print(f"tools_list_names={list_names}", file=sys.stderr)
    raise SystemExit(1)

print(f"MCP tool catalog consistent: count={len(status_names)}")
PY
