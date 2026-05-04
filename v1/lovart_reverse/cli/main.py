"""Console script entry point for the Lovart CLI."""

from __future__ import annotations

from lovart_reverse.cli.application import main

__all__ = ["main"]


if __name__ == "__main__":
    raise SystemExit(main())
