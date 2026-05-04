"""Error types and CLI exit-code mapping."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any


@dataclass
class LovartError(Exception):
    code: str
    message: str
    details: dict[str, Any] = field(default_factory=dict)
    exit_code: int = 1

    def __str__(self) -> str:
        return self.message


class InputError(LovartError):
    def __init__(self, message: str, details: dict[str, Any] | None = None):
        super().__init__("input_error", message, details or {}, 2)


class AuthError(LovartError):
    def __init__(self, message: str, details: dict[str, Any] | None = None):
        super().__init__("auth_error", message, details or {}, 3)


class AuthMissingError(LovartError):
    def __init__(self, message: str, details: dict[str, Any] | None = None):
        super().__init__("auth_missing", message, details or {}, 3)


class SignatureError(LovartError):
    def __init__(self, message: str, details: dict[str, Any] | None = None):
        super().__init__("signature_error", message, details or {}, 4)


class SignerStaleError(LovartError):
    def __init__(self, message: str, details: dict[str, Any] | None = None):
        super().__init__("signer_stale", message, details or {}, 4)


class RemoteError(LovartError):
    def __init__(self, message: str, details: dict[str, Any] | None = None):
        super().__init__("remote_error", message, details or {}, 5)


class MetadataStaleError(LovartError):
    def __init__(self, message: str, details: dict[str, Any] | None = None):
        super().__init__("metadata_stale", message, details or {}, 5)


class SchemaInvalidError(LovartError):
    def __init__(self, message: str, details: dict[str, Any] | None = None):
        super().__init__("schema_invalid", message, details or {}, 2)


class CreditRiskError(LovartError):
    def __init__(self, message: str, details: dict[str, Any] | None = None):
        super().__init__("credit_risk", message, details or {}, 6)


class UnknownPricingError(LovartError):
    def __init__(self, message: str, details: dict[str, Any] | None = None):
        super().__init__("unknown_pricing", message, details or {}, 7)


class TaskFailedError(LovartError):
    def __init__(self, message: str, details: dict[str, Any] | None = None):
        super().__init__("task_failed", message, details or {}, 8)


class TimeoutError(LovartError):
    def __init__(self, message: str, details: dict[str, Any] | None = None):
        super().__init__("timeout", message, details or {}, 9)
