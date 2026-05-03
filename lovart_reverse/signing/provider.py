"""LGW signing backed by Lovart's WASM signer wrapper."""

from __future__ import annotations

import subprocess
import json
import threading
import time
import uuid
from pathlib import Path
from typing import Callable

import requests

from lovart_reverse.errors import SignatureError
from lovart_reverse.paths import SIGNATURE_JS

TIME_SYNC_URL = "https://www.lovart.ai/api/www/lovart/time/utc/timestamp"


def sign(timestamp: str, req_uuid: str, third: str = "", fourth: str = "") -> str:
    try:
        result = subprocess.run(
            ["node", str(SIGNATURE_JS), timestamp, req_uuid, third, fourth],
            capture_output=True,
            text=True,
            check=True,
        )
    except (subprocess.CalledProcessError, FileNotFoundError) as exc:
        raise SignatureError("failed to run Lovart signer", {"error": str(exc)})
    return result.stdout.strip()


class PersistentSigner:
    """Keep the Lovart WASM signer loaded in one Node process."""

    def __init__(self, wasm_path: Path | None = None):
        command = ["node", str(SIGNATURE_JS), "--stdio"]
        if wasm_path is not None:
            command.append(str(wasm_path))
        try:
            self._proc = subprocess.Popen(
                command,
                stdin=subprocess.PIPE,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                text=True,
                bufsize=1,
            )
        except FileNotFoundError as exc:
            raise SignatureError("failed to start persistent Lovart signer", {"error": str(exc)}) from exc
        self._lock = threading.Lock()
        self._next_id = 0

    def sign(self, timestamp: str, req_uuid: str, third: str = "", fourth: str = "") -> str:
        with self._lock:
            if self._proc.poll() is not None:
                raise SignatureError("persistent Lovart signer exited", {"returncode": self._proc.returncode})
            if self._proc.stdin is None or self._proc.stdout is None:
                raise SignatureError("persistent Lovart signer pipes are unavailable")
            self._next_id += 1
            request_id = self._next_id
            payload = {
                "id": request_id,
                "timestamp": str(timestamp),
                "req_uuid": str(req_uuid),
                "third": str(third),
                "fourth": str(fourth),
            }
            try:
                self._proc.stdin.write(json.dumps(payload, separators=(",", ":")) + "\n")
                self._proc.stdin.flush()
                line = self._proc.stdout.readline()
            except OSError as exc:
                raise SignatureError("persistent Lovart signer IO failed", {"error": str(exc)}) from exc
            if not line:
                raise SignatureError("persistent Lovart signer returned no output", {"returncode": self._proc.poll()})
            try:
                response = json.loads(line)
            except json.JSONDecodeError as exc:
                raise SignatureError("persistent Lovart signer returned invalid JSON", {"line": line.strip()}) from exc
            if response.get("id") != request_id:
                raise SignatureError("persistent Lovart signer response id mismatch", {"expected": request_id, "actual": response.get("id")})
            if not response.get("ok"):
                raise SignatureError("persistent Lovart signer failed", {"error": response.get("error")})
            signature = response.get("signature")
            if not isinstance(signature, str) or not signature:
                raise SignatureError("persistent Lovart signer returned an empty signature")
            return signature

    def close(self) -> None:
        if self._proc.poll() is None:
            self._proc.terminate()
            try:
                self._proc.wait(timeout=2)
            except subprocess.TimeoutExpired:
                self._proc.kill()

    def __enter__(self) -> "PersistentSigner":
        return self

    def __exit__(self, exc_type, exc, tb) -> None:
        self.close()


def sync_time(sess: requests.Session | None = None) -> int:
    sess = sess or requests.Session()
    start = int(time.time() * 1000)
    response = sess.get(f"{TIME_SYNC_URL}?_t={start}", timeout=20)
    end = int(time.time() * 1000)
    response.raise_for_status()
    data = response.json()
    server_ts = int(data["data"]["timestamp"])
    rtt = end - start
    return int(server_ts - (start + rtt / 2))


def signed_headers(
    offset_ms: int = 0,
    language: str = "zh",
    signer: Callable[[str, str, str, str], str] | None = None,
) -> dict[str, str]:
    timestamp = str(int(time.time() * 1000 + offset_ms))
    req_uuid = uuid.uuid4().hex
    sign_fn = signer or sign
    return {
        "accept-language": language,
        "X-Send-Timestamp": timestamp,
        "X-Req-Uuid": req_uuid,
        "X-Client-Signature": sign_fn(timestamp, req_uuid, "", ""),
    }
