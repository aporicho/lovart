# Lovart Reverse

Lovart 网页端生成能力的本地逆向工具。目标是把模型列表、schema、价格、权益、提交、任务查询、下载和更新检测封装成稳定 JSON CLI，供 Claude Code、Codex、opencode 这类 agent 调用。

默认策略是不扣积分：真实生成前必须完成 schema 校验、0 积分权益判断和价格估算。非 0 积分或价格未知时，必须显式传 `--allow-paid --max-credits N`，并设置 `LOVART_ALLOW_GENERATION=1`。

## Install

```bash
uv sync
uv run python -m lovart_reverse.cli.main models
```

安装为普通 wheel 后会提供 console script：

```bash
lovart models
```

## CLI Contract

stdout 只输出 JSON，stderr 只输出诊断信息。

成功：

```json
{"ok":true,"data":{},"warnings":[]}
```

失败：

```json
{"ok":false,"error":{"code":"...","message":"...","details":{}}}
```

## Commands

```bash
uv run python -m lovart_reverse.cli.main auth status
uv run python -m lovart_reverse.cli.main auth extract captures/request.json
uv run python -m lovart_reverse.cli.main models
uv run python -m lovart_reverse.cli.main schema openai/gpt-image-2
uv run python -m lovart_reverse.cli.main price openai/gpt-image-2 --body '{"prompt":"cat","quality":"low","size":"1024*1024"}'
uv run python -m lovart_reverse.cli.main free openai/gpt-image-2 --mode auto --body '{"prompt":"cat","quality":"low","size":"1024*1024"}'
uv run python -m lovart_reverse.cli.main generate openai/gpt-image-2 --dry-run --body '{"prompt":"cat","quality":"low","size":"1024*1024"}'
uv run python -m lovart_reverse.cli.main task TASK_ID
uv run python -m lovart_reverse.cli.main reverse capture
uv run python -m lovart_reverse.cli.main reverse replay captures/request.json
```

## Update Detection

本地 manifest 位于 `ref/lovart_manifest.json`，记录 Lovart canvas 静态 bundle、Sentry release、generator list/schema、pricing、entitlement shape 和 signer WASM 信息的 hash。不会记录 token、cookie、邮箱或账号 ID。

```bash
uv run python -m lovart_reverse.cli.main update check
uv run python -m lovart_reverse.cli.main update diff
uv run python -m lovart_reverse.cli.main update sync --metadata-only
```

`update check` 是只读操作。`update sync --metadata-only` 会刷新：

- `ref/lovart_generator_list.json`
- `ref/lovart_generator_schema.json`
- `ref/lovart_pricing_table.json`
- `ref/lovart_manifest.json`

同步后会自动跑最小离线检查：registry、pricing、entitlement。

## Package Layout

业务代码全部在 `lovart_reverse/`：

- `auth/`：凭证读取、抓包抽取、敏感信息保护。
- `signing/`：LGW 签名、时间同步、签名 provider。
- `http/`：`www`、`canva`、`lgw` client。
- `discovery/`：模型列表与 OpenAPI schema。
- `registry/`：统一模型 registry、schema、能力。
- `pricing/`：价格表、余额、峰谷倍率、批量估算。
- `entitlement/`：低速无限、快速 0 积分、请求约束匹配。
- `generation/`：生成 dry-run、paid gate、提交。
- `task/`：任务查询、状态归一化。
- `assets/`：上传能力，占位到抓包确认后实现。
- `downloads/`：artifact 下载。
- `update/`：官方版本漂移检测、快照 hash、同步建议。
- `capture/`：mitm 抓包、replay、逆向辅助。
- `cli/`：唯一 CLI 实现。

架构规则见 `docs/architecture/file-architecture-philosophy.md`。运行检查：

```bash
uv run python -m lovart_reverse.cli.main doctor
```

## Safety

这些路径默认不入库：

- `.lovart/`
- `scripts/creds.json`
- `captures/`
- `downloads/`
- `.lovart-chrome-profile/`
- `.mitmproxy/`
- `.venv/`

不要用本项目绕过登录、验证码、额度、付费、风控或访问控制。只逆向你自己账号在浏览器里合法产生的请求。
