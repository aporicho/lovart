"""LGW signing backed by Lovart's WASM signer wrapper."""

from __future__ import annotations

import subprocess
import time
import uuid

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


def signed_headers(offset_ms: int = 0, language: str = "zh") -> dict[str, str]:
    timestamp = str(int(time.time() * 1000 + offset_ms))
    req_uuid = uuid.uuid4().hex
    return {
        "accept-language": language,
        "X-Send-Timestamp": timestamp,
        "X-Req-Uuid": req_uuid,
        "X-Client-Signature": sign(timestamp, req_uuid),
    }
