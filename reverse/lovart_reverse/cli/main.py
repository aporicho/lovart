"""Reverse tool CLI entry point.

Commands:
    lovart-reverse start        Start capture session
    lovart-reverse capture      Low-level mitm capture
    lovart-reverse auth extract <file>   Extract credentials
    lovart-reverse update check          Check metadata drift
    lovart-reverse update sync --metadata-only  Sync metadata
"""
import json
import sys


def main():
    args = sys.argv[1:]
    if not args:
        print(json.dumps({"ok": False, "error": {"code": "input_error", "message": "no command"}}))
        return 2

    cmd = args[0]
    if cmd == "start":
        print(json.dumps({"ok": True, "data": {"status": "not implemented yet"}}))
    elif cmd == "capture":
        print(json.dumps({"ok": True, "data": {"status": "not implemented yet"}}))
    elif cmd == "auth":
        print(json.dumps({"ok": True, "data": {"status": "not implemented yet"}}))
    elif cmd == "update":
        print(json.dumps({"ok": True, "data": {"status": "not implemented yet"}}))
    else:
        print(json.dumps({"ok": False, "error": {"code": "input_error", "message": f"unknown command: {cmd}"}}))
        return 2
    return 0


if __name__ == "__main__":
    sys.exit(main())
