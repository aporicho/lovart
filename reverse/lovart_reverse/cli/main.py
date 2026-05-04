"""Lovart reverse tool CLI — capture browser traffic and extract credentials.

Commands:
    start        Start mitmproxy + Chrome capture session
    extract      Extract credentials from a capture file
"""
import json
import sys
from pathlib import Path


def main():
    args = sys.argv[1:]

    if not args:
        print(json.dumps({"ok": True, "data": {
            "package": "lovart-reverse",
            "version": "2.0.0-dev",
            "commands": ["start", "extract"],
        }}))
        return 0

    cmd = args[0]

    if cmd == "start":
        try:
            from lovart_reverse.capture.session import run_capture_session
            result = run_capture_session()
            print(json.dumps({"ok": True, "data": result}, ensure_ascii=False, default=str))
        except Exception as exc:
            print(json.dumps({"ok": False, "error": {
                "code": "capture_error",
                "message": str(exc),
            }}, ensure_ascii=False))
            return 2
        return 0

    if cmd == "extract":
        if len(args) < 2:
            print(json.dumps({"ok": False, "error": {
                "code": "input_error",
                "message": "usage: lovart-reverse extract <capture_file>",
            }}))
            return 2
        capture_file = Path(args[1])
        try:
            from lovart_reverse.auth.extract import extract_from_capture
            result = extract_from_capture(capture_file)
            print(json.dumps({"ok": True, "data": result}, ensure_ascii=False, default=str))
        except Exception as exc:
            print(json.dumps({"ok": False, "error": {
                "code": "extract_error",
                "message": str(exc),
            }}, ensure_ascii=False))
            return 2
        return 0

    print(json.dumps({"ok": False, "error": {
        "code": "input_error",
        "message": f"unknown command: {cmd}",
    }}))
    return 2


if __name__ == "__main__":
    sys.exit(main())
