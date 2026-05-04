"""Model registry types."""

from __future__ import annotations

from dataclasses import asdict, dataclass


@dataclass(frozen=True)
class ModelRecord:
    model: str
    display_name: str
    type: str
    vip: bool
    path: str
    summary: str
    request_schema: str
    response_schema: str
    index: int

    def to_dict(self) -> dict[str, object]:
        return asdict(self)
