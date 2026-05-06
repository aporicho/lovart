import json
import sys

from lovart_reverse.cli.main import main


def test_cli_no_args_prints_command_summary(monkeypatch, capsys):
    monkeypatch.setattr(sys, "argv", ["lovart-reverse"])

    code = main()

    assert code == 0
    payload = json.loads(capsys.readouterr().out)
    assert payload["ok"] is True
    assert payload["data"]["package"] == "lovart-reverse"
    assert "start" in payload["data"]["commands"]
    assert "extract" in payload["data"]["commands"]
