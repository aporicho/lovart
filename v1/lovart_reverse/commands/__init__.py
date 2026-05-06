"""Stable command facade shared by CLI and MCP wrappers."""

from lovart_reverse.commands.facade import (
    config_command,
    generate_command,
    jobs_dry_run_command,
    jobs_quote_command,
    jobs_quote_status_command,
    jobs_resume_command,
    jobs_run_command,
    jobs_status_command,
    models_command,
    quote_command,
    self_test_command,
    setup_command,
    version_command,
)

__all__ = [
    "config_command",
    "generate_command",
    "jobs_dry_run_command",
    "jobs_quote_command",
    "jobs_quote_status_command",
    "jobs_resume_command",
    "jobs_run_command",
    "jobs_status_command",
    "models_command",
    "quote_command",
    "self_test_command",
    "setup_command",
    "version_command",
]
