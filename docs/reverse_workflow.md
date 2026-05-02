# Lovart Reverse Workflow

## 1. Capture

Start a mitm capture command preview:

```bash
uv run python -m lovart_reverse.cli.main reverse capture
```

Run the returned `mitmdump` command in a separate shell, browse Lovart through `http://127.0.0.1:8080`, and perform one narrow flow at a time. Capture files stay in `captures/`, which is ignored by git.

If the goal is only metadata, a generation run is not required. The read-only discovery endpoints are:

```text
GET https://lgw.lovart.ai/v1/generator/list?biz_type=16
GET https://lgw.lovart.ai/v1/generator/schema?biz_type=16
```

## 2. Validate Metadata

Use the stable JSON CLI:

```bash
uv run python -m lovart_reverse.cli.main models
uv run python -m lovart_reverse.cli.main schema openai/gpt-image-2
uv run python -m lovart_reverse.cli.main update check
```

Refresh local metadata without generating:

```bash
uv run python -m lovart_reverse.cli.main update sync --metadata-only
```

## 3. Validate Cost And Entitlement

Run price and free checks before any batch:

```bash
uv run python -m lovart_reverse.cli.main price openai/gpt-image-2 \
  --body '{"prompt":"cat","quality":"low","size":"2048*2048","n":1}' \
  --batch 20 \
  --with-balance \
  --with-time-variant

uv run python -m lovart_reverse.cli.main free openai/gpt-image-2 \
  --mode auto \
  --body '{"prompt":"cat","quality":"low","size":"2048*2048","n":1}'
```

`free --mode fast` checks fast zero-credit eligibility. `free --mode relax` checks unlimited low-speed generation. `free --mode auto` evaluates both.

## 4. Dry Run Generation

Dry run builds the signed-request preview and runs the paid gate:

```bash
uv run python -m lovart_reverse.cli.main generate openai/gpt-image-2 \
  --dry-run \
  --mode auto \
  --body '{"prompt":"cat","quality":"low","size":"2048*2048","n":1}'
```

Real submission is blocked unless the request is zero-credit, or `--allow-paid --max-credits N` is present. Real generation also requires:

```bash
export LOVART_ALLOW_GENERATION=1
```

## 5. Replay Evidence

Replay is for reverse validation, not stable API use:

```bash
uv run python -m lovart_reverse.cli.main reverse replay captures/<request>.json
uv run python -m lovart_reverse.cli.main reverse replay captures/<request>.json --submit
```

Move confirmed behavior into package modules only after there is capture evidence. Do not add guessed upload or task APIs as production wrappers.
