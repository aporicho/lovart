# 真实远端 Batch Scheduler 验证 Runbook

本文档用于在真实 Lovart 账号和真实远端环境中验证 batch scheduler。默认按 Level 0 -> Level 1 -> Level 2 逐级执行；不要跳级。Level 2 会创建远端 generation task，可能消耗 credits，必须由操作者显式 opt-in。

硬性安全规则：

- 不在 issue、PR、终端录屏或日志中粘贴 cookie、token、CSRF、CID、完整凭据文件或浏览器 profile。
- 不把 `~/.lovart/creds.json`、audit JSONL、jobs state 中的敏感字段提交到 git。
- Level 0 和 Level 1 不允许提交生成任务；所有 smoke 命令都不要带 `--submit`。
- Level 2 只允许极小预算：1-3 条 logical jobs、`--submit-limit 1`、`--max-active-tasks 1` 或 `2`、`--allow-paid`、极小 `--max-total-credits`。
- canvas 写回是单独 opt-in；不要把 `--canvas` 混进首次 Level 2 验证。

## 0. 通用准备

### 0.1 选择隔离 runtime home

建议先使用隔离目录，避免污染日常 Lovart runtime。下面命令只设置本地 shell 环境，不访问网络。

```sh
export LOVART_HOME="${LOVART_HOME:-$HOME/.lovart-remote-batch-verify}"
mkdir -p "$LOVART_HOME"
```

命令标注：

| 命令 | execution_class | network_required | remote_write | paid_possible | 预计副作用 | 停止条件 |
| --- | --- | --- | --- | --- | --- | --- |
| `export LOVART_HOME=...` | local | false | false | false | 仅影响当前 shell 的 Lovart runtime 路径 | 变量为空或指向不想使用的目录时停止 |
| `mkdir -p "$LOVART_HOME"` | local | false | false | false | 创建本地目录 | 目录无法创建时停止 |

### 0.2 统一输出和审计路径

```sh
export VERIFY_ROOT="$PWD/lovart-remote-batch-verify"
export VERIFY_AUDIT="$VERIFY_ROOT/audit.jsonl"
export LOVART_AUDIT_LOG="$VERIFY_AUDIT"
mkdir -p "$VERIFY_ROOT"
: > "$VERIFY_AUDIT"
```

命令标注：

| 命令 | execution_class | network_required | remote_write | paid_possible | 预计副作用 | 停止条件 |
| --- | --- | --- | --- | --- | --- | --- |
| `export VERIFY_ROOT=...` | local | false | false | false | 仅设置本地变量 | 路径不是本次验证目录时停止 |
| `export VERIFY_AUDIT=...` | local | false | false | false | 仅设置本地变量 | 路径不可写时停止 |
| `export LOVART_AUDIT_LOG=...` | local | false | false | false | 开启 CLI/MCP audit JSONL 写入 | 不希望保留审计文件时停止 |
| `mkdir -p "$VERIFY_ROOT"` | local | false | false | false | 创建本地验证目录 | 目录无法创建时停止 |
| `: > "$VERIFY_AUDIT"` | local | false | false | false | 清空/创建本地审计 JSONL | 文件包含需要保留的历史审计时停止 |

## Level 0：本地无网络 / 无远端写验证

目标：确认二进制、基础自检、MCP 本地配置和 batch 输入文件都可用。此级别不要求 Lovart 登录，不发起远端读写。

### 0.1 版本和自检

```sh
lovart --version
lovart self-test
lovart doctor
```

命令标注：

| 命令 | execution_class | network_required | remote_write | paid_possible | 预计副作用 | 停止条件 |
| --- | --- | --- | --- | --- | --- | --- |
| `lovart --version` | local | false | false | false | 读取本地二进制版本 | 命令不存在或版本不符合待验证 release/commit 时停止 |
| `lovart self-test` | local | false | false | false | 读取本地 runtime、配置、权限；不提交任务 | 返回 `ok:false` 且不是预期的未登录/未配置提示时停止 |
| `lovart doctor` | local | false | false | false | 本地 readiness 检查；不会运行 online update check | 返回 `ok:false` 或本地依赖缺失时停止 |

期望状态：

- envelope 中 `execution_class` 为 `local`。
- `network_required=false`、`remote_write=false`。
- 未登录时可以出现 auth/project 相关 recommended actions；不应出现真实远端 task id。

### 0.2 MCP 本地状态和本地 smoke

```sh
lovart mcp status
lovart mcp smoke --model openai/gpt-image-2 --prompt "local mcp smoke only" --mode relax
```

命令标注：

| 命令 | execution_class | network_required | remote_write | paid_possible | 预计副作用 | 停止条件 |
| --- | --- | --- | --- | --- | --- | --- |
| `lovart mcp status` | local | false | false | false | 读取/渲染本地 MCP client 配置状态；不写配置 | 输出包含不应公开的 secret 时停止并上报安全问题 |
| `lovart mcp smoke ...` | local 或 preflight | false unless 已登录后触发安全读检查 | false | false | 启动本地 stdio MCP server；不带 `--submit`，不会提交生成 | 任何输出显示 submit/generate 已执行时停止 |

说明：未登录环境下 `mcp smoke` 应完成 initialize/ping/tools/list，并跳过需要 auth 的在线 quote/balance。已登录环境下它可能做安全远端读检查，但只要不带 `--submit` 就不应创建 generation task。

### 0.3 创建 Level 1/2 共用请求文件和本地 schema 检查

```sh
cat > "$VERIFY_ROOT/request.json" <<'JSON'
{
  "prompt": "a tiny verification icon, simple geometric shapes, neutral background"
}
JSON

cat > "$VERIFY_ROOT/jobs.jsonl" <<'JSONL'
{"job_id":"verify-001","title":"remote batch verification 001","fields":{"folder":"verify","scene_no":"001"},"model":"openai/gpt-image-2","mode":"relax","outputs":1,"body":{"prompt":"a tiny verification icon, simple geometric shapes, neutral background"}}
JSONL

lovart config openai/gpt-image-2
```

命令标注：

| 命令 | execution_class | network_required | remote_write | paid_possible | 预计副作用 | 停止条件 |
| --- | --- | --- | --- | --- | --- | --- |
| `cat > "$VERIFY_ROOT/request.json" ...` | local | false | false | false | 写入本地 request JSON | 文件路径不是验证目录时停止 |
| `cat > "$VERIFY_ROOT/jobs.jsonl" ...` | local | false | false | false | 写入 1 条本地 batch JSONL | 超过 3 条 logical jobs 或 prompt 不是测试用途时停止 |
| `lovart config openai/gpt-image-2` | local | false | false | false | 读取 cached model schema/config | metadata 不存在或 schema 不包含 request 字段时先运行 Level 1 setup/doctor |

期望 jobs 文件：

- logical jobs: 1
- remote requests: 1
- requested outputs: 1
- 初始 `status_counts` 在 run 前不存在；run 后应至少包含 `quoted`、`submitted`、`running`、`completed`、`failed`、`downloaded` 中的一种。

## Level 1：远端读 / preflight，不 submit

目标：确认 auth、runtime metadata、project context、余额、报价和 MCP online smoke 可以读取真实远端状态，但不创建 generation task，也不修改 project。

### 1.1 登录状态和登录

```sh
lovart auth status
lovart auth login --timeout-seconds 300
lovart auth status
```

命令标注：

| 命令 | execution_class | network_required | remote_write | paid_possible | 预计副作用 | 停止条件 |
| --- | --- | --- | --- | --- | --- | --- |
| `lovart auth status` | local | false | false | false | 读取本地凭据可用性；不打印 secret | 输出包含 cookie/token/CSRF 时停止 |
| `lovart auth login --timeout-seconds 300` | preflight | true | false | false | 打开浏览器和本地 callback；保存用户批准的会话凭据到 `LOVART_HOME` | 不是本人账号、浏览器 URL 异常、或不想保存本地凭据时停止 |
| `lovart auth status` | local | false | false | false | 确认 `available=true` | 未认证时停止，不进入 Level 2 |

### 1.2 runtime cache 和在线 doctor

```sh
lovart setup
lovart doctor --online
lovart models --refresh
lovart config openai/gpt-image-2 --all
```

命令标注：

| 命令 | execution_class | network_required | remote_write | paid_possible | 预计副作用 | 停止条件 |
| --- | --- | --- | --- | --- | --- | --- |
| `lovart setup` | local/preflight | may be true | false | false | 准备 signer/metadata cache；可能写本地 cache | signer 或 metadata 来源异常时停止 |
| `lovart doctor --online` | preflight | true | false | false | 读取在线 update/network 状态 | `network_unavailable` 或 signer/metadata stale 无法恢复时停止 |
| `lovart models --refresh` | preflight | true | false | false | 刷新本地 generator metadata cache | model 列表缺少测试 model 时停止 |
| `lovart config ... --all` | local | false | false | false | 打印 cached model config/schema | request body 与 schema 不兼容时停止 |

### 1.3 project list/select 和只读余额/报价

```sh
lovart project list
lovart project select <project_id>
lovart project current
lovart balance
lovart quote openai/gpt-image-2 --body-file "$VERIFY_ROOT/request.json" --mode relax
```

命令标注：

| 命令 | execution_class | network_required | remote_write | paid_possible | 预计副作用 | 停止条件 |
| --- | --- | --- | --- | --- | --- | --- |
| `lovart project list` | preflight | true | false | false | 读取账号 projects | 没有明确测试 project 时停止，不要随意用生产 project |
| `lovart project select <project_id>` | local | false | false | false | 保存本地 project context (`project_id`/CID) | project 不是测试目标或 CID 异常时停止 |
| `lovart project current` | local | false | false | false | 读取本地已选 project context | project context 缺失时停止 |
| `lovart balance` | preflight | true | false | false | 读取余额/credits | 余额不足、账号异常或输出包含 secret 时停止 |
| `lovart quote ...` | preflight | true | false | false | 获取报价，不创建 task | quote 超过 Level 2 预算或返回 `credit_risk` 时停止 |

### 1.4 MCP online smoke，不 submit

```sh
lovart mcp smoke --model openai/gpt-image-2 --body-file "$VERIFY_ROOT/request.json" --mode relax
```

命令标注：

| 命令 | execution_class | network_required | remote_write | paid_possible | 预计副作用 | 停止条件 |
| --- | --- | --- | --- | --- | --- | --- |
| `lovart mcp smoke ...` | preflight | true | false | false | 启动本地 MCP server，执行 auth/config/balance/quote 等安全工具；不提交生成 | 输出中出现 `task_id` 或 audit 中出现 `jobs.submit_request`/generate submit 时停止 |

Level 1 期望状态：

- 所有命令都不应创建 remote task，不应消耗 credits。
- `quote` 返回的价格应小于你计划在 Level 2 使用的 `--max-total-credits`。
- audit JSONL 可能出现 MCP tool call、policy gate 或 preflight 事件，但不应出现 `operation="jobs.submit_request"`。

检查 audit JSONL：

```sh
python3 - <<'PY'
import json, os
path = os.environ["LOVART_AUDIT_LOG"]
for i, line in enumerate(open(path, encoding="utf-8"), 1):
    if not line.strip():
        continue
    row = json.loads(line)
    print(i, row.get("timestamp"), row.get("operation"), row.get("action"), row.get("decision"), row.get("risk_flags"), row.get("metadata", {}))
PY
```

命令标注：

| 命令 | execution_class | network_required | remote_write | paid_possible | 预计副作用 | 停止条件 |
| --- | --- | --- | --- | --- | --- | --- |
| `python3 ... audit reader` | local | false | false | false | 读取本地 audit JSONL 并打印摘要 | JSON parse 失败、含 secret、或出现未预期 submit 事件时停止 |

## Level 2：明确付费 / 远端写，极小预算

目标：真实创建极少量 remote task，验证 batch scheduler 的提交门禁、状态刷新、恢复、finalize 下载和可选 canvas 写回。执行前必须完成 Level 1，并再次确认预算。

执行 Level 2 前确认：

```sh
lovart balance
lovart quote openai/gpt-image-2 --body-file "$VERIFY_ROOT/request.json" --mode relax
```

命令标注：

| 命令 | execution_class | network_required | remote_write | paid_possible | 预计副作用 | 停止条件 |
| --- | --- | --- | --- | --- | --- | --- |
| `lovart balance` | preflight | true | false | false | 读取当前 credits | 余额不足或非测试账号时停止 |
| `lovart quote ...` | preflight | true | false | false | 读取本次最小任务报价 | 报价超过显式预算时停止 |

### 2.1 单次提交 1 个 remote request

把 `VERIFY_MAX_CREDITS` 设置为刚才 quote 结果以上、但仍然极小的值。例如 quote 为 2 credits 时可设为 3；不要使用大预算。

```sh
export VERIFY_MAX_CREDITS="3"
lovart jobs run "$VERIFY_ROOT/jobs.jsonl" \
  --allow-paid \
  --max-total-credits "$VERIFY_MAX_CREDITS" \
  --submit-limit 1 \
  --max-active-tasks 1 \
  --submit-interval-seconds 2
```

命令标注：

| 命令 | execution_class | network_required | remote_write | paid_possible | 预计副作用 | 停止条件 |
| --- | --- | --- | --- | --- | --- | --- |
| `export VERIFY_MAX_CREDITS="3"` | local | false | false | false | 设置当前 shell 的预算变量 | 数值大于本次验证最小需要时停止并调低 |
| `lovart jobs run ... --allow-paid ...` | submit | true | true | true | 读取 quote、创建最多 1 个 remote task、写本地 run state、写 audit JSONL；可能消耗 credits | 未带 `--allow-paid`/预算为空/`--submit-limit` 不是 1/`--max-active-tasks` 大于 2/project 非测试时停止 |

期望输出状态：

- envelope: `ok=true`，`execution_class="submit"`，`network_required=true`，`remote_write=true`。
- `data.run_dir` 指向本地 run 目录，例如 `$LOVART_HOME/runs/jobs-<hash>`。
- `data.summary.logical_jobs=1`、`remote_requests=1`、`requested_outputs=1`。
- `data.submission.submit_limit=1`、`max_active_tasks=1`、`submitted_count` 为 `0` 或 `1`。
- 常见 `summary.status_counts`：
  - 成功提交后：`{"submitted":1}` 或很快刷新为 `{"running":1}`。
  - active cap 或 rate limit：`{"quoted":1}` 或 `{"pending":1}`，并在 `submission.deferred=true` 中给出 reason。
  - 远端同步失败：`{"failed":1}`，查看 `failed[].last_error.code`。

保存 run_dir：

```sh
export VERIFY_RUN_DIR="$(python3 - <<'PY'
import json, os, glob
home = os.environ.get('LOVART_HOME') or os.path.expanduser('~/.lovart')
runs = sorted(glob.glob(os.path.join(home, 'runs', '*')), key=os.path.getmtime)
print(runs[-1] if runs else '')
PY
)"
printf 'VERIFY_RUN_DIR=%s\n' "$VERIFY_RUN_DIR"
test -n "$VERIFY_RUN_DIR" && test -f "$VERIFY_RUN_DIR/jobs_state.json"
```

命令标注：

| 命令 | execution_class | network_required | remote_write | paid_possible | 预计副作用 | 停止条件 |
| --- | --- | --- | --- | --- | --- | --- |
| `export VERIFY_RUN_DIR=...` | local | false | false | false | 从本地 runs 目录选最新 run；仅设置变量 | 最新 run 不是本次 jobs 文件时停止，手动从 `jobs run` 输出复制 `data.run_dir` |
| `printf ...` | local | false | false | false | 打印本地 run_dir | 为空时停止 |
| `test -n ... && test -f ...` | local | false | false | false | 校验 state 文件存在 | state 文件不存在时停止 |

### 2.2 刷新状态并恢复调度

```sh
lovart jobs status "$VERIFY_RUN_DIR" --refresh --detail requests
lovart jobs resume "$VERIFY_RUN_DIR" \
  --allow-paid \
  --max-total-credits "$VERIFY_MAX_CREDITS" \
  --submit-limit 1 \
  --max-active-tasks 1 \
  --submit-interval-seconds 2
lovart jobs status "$VERIFY_RUN_DIR" --refresh --detail full
```

命令标注：

| 命令 | execution_class | network_required | remote_write | paid_possible | 预计副作用 | 停止条件 |
| --- | --- | --- | --- | --- | --- | --- |
| `lovart jobs status ... --refresh` | preflight | true | false | false | 读取本地 state，并刷新已有 active remote task 状态 | task 属于非测试 project 或状态异常时停止 |
| `lovart jobs resume ... --allow-paid ...` | submit | true | true | true | 继续提交 pending/quoted request；本 runbook 中最多再尝试 1 次 | 预算变大、`--submit-limit` 不是 1、或已无 pending 但仍准备重复提交时停止 |
| `lovart jobs status ... --refresh --detail full` | preflight | true | false | false | 刷新状态并打印完整本地 state | 看到 unexpected failed/content policy/timeout 时按恢复策略处理 |

期望状态转换：

- `submitted`/`running` 最终应转为 `completed` 或 `failed`。
- `summary.complete=true` 代表所有 remote requests 处于终态或已下载等完成状态。
- 若还有 `pending` 或 `quoted`，按 `data.recommended_actions` 继续执行同样的 `resume`，但保持同样极小预算和 submit limit。
- 若 `remote_status_counts` 包含远端 running 状态，等待后重复 `status --refresh`，不要提高并发。

### 2.3 finalize 下载，不写 canvas

只在任务 `completed` 且已有 artifacts 后执行。`--download` 只读取远端产物并写本地文件，不修改项目画布。

```sh
lovart jobs finalize "$VERIFY_RUN_DIR" \
  --download \
  --download-dir "$VERIFY_ROOT/downloads" \
  --detail requests
lovart jobs status "$VERIFY_RUN_DIR" --detail requests
```

命令标注：

| 命令 | execution_class | network_required | remote_write | paid_possible | 预计副作用 | 停止条件 |
| --- | --- | --- | --- | --- | --- | --- |
| `lovart jobs finalize ... --download` | preflight | true | false | false | 下载已完成 artifacts 到本地目录；更新本地 state download 记录 | 任务未 completed、download 目录错误、或需要 canvas 但尚未单独 opt-in 时停止 |
| `lovart jobs status ... --detail requests` | local | false | false | false | 读取本地 state，确认 download_count | `downloaded_artifacts` 未增加时检查 finalize 输出错误 |

期望状态：

- `summary.downloaded_artifacts >= 1`。
- 对应 request 的 `download_count >= 1`。
- 若实现将已下载 request 标记为 `downloaded`，`status_counts.downloaded=1`；否则可能仍为 `completed=1` 且 request 有 downloads，这两种都可接受。

### 2.4 canvas 写回，单独 opt-in

只有在明确要验证项目画布写回时执行。此步骤会修改远端 project canvas，不应默认执行。

```sh
# 仅在确认要写回测试 project canvas 后取消下一行注释：
# lovart jobs finalize "$VERIFY_RUN_DIR" --canvas --canvas-layout frame --detail requests
```

命令标注：

| 命令 | execution_class | network_required | remote_write | paid_possible | 预计副作用 | 停止条件 |
| --- | --- | --- | --- | --- | --- | --- |
| `lovart jobs finalize ... --canvas` | submit | true | true | false | 将已完成 artifacts 写回远端 project canvas；更新本地 state canvas 记录 | 未获 canvas opt-in、project 不是测试 project、或已经写回过同一批 request 时停止 |

canvas 期望状态：

- `summary.canvas_updated_requests >= 1`。
- `summary.canvas_images >= 1`。
- request 中 `canvas_updated=true`。

## audit JSONL 检查

Level 2 后必须检查 audit。重点确认：预算、远端写、submit 次数、task id 和 redaction。

```sh
python3 - <<'PY'
import json, os, collections
path = os.environ["LOVART_AUDIT_LOG"]
counts = collections.Counter()
submit_events = []
for i, line in enumerate(open(path, encoding="utf-8"), 1):
    if not line.strip():
        continue
    row = json.loads(line)
    op = row.get("operation") or "<missing>"
    counts[op] += 1
    if op == "jobs.submit_request":
        submit_events.append((i, row))
print("operation_counts", dict(counts))
print("submit_event_count", len(submit_events))
for i, row in submit_events:
    md = row.get("metadata", {})
    print("submit", i, row.get("decision"), md.get("request_id"), md.get("task_id"), md.get("model"), md.get("mode"))
PY
```

命令标注：

| 命令 | execution_class | network_required | remote_write | paid_possible | 预计副作用 | 停止条件 |
| --- | --- | --- | --- | --- | --- | --- |
| `python3 ... audit summary` | local | false | false | false | 汇总本地 audit JSONL | `submit_event_count` 超过预期、budget metadata 异常、或出现未脱敏 secret 时停止并保留证据 |

期望 audit：

- `jobs.submit_request` 数量不超过实际 `--submit-limit` 累计次数。本 runbook 的 1 条 jobs.jsonl 通常为 1。
- submit 事件包含 `request_id`、`model`、`mode`，成功时可包含 `task_id`。
- `risk_flags` 对 submit 包含 `remote_write` 和 `paid_possible`。
- `metadata.allow_paid=true`、`metadata.max_total_credits` 等于本次极小预算。
- 不应出现 cookie、token、CSRF、CID 原文、Authorization header 或完整凭据内容。

## 失败恢复策略

### 429 / rate limit

症状：`failed[].last_error.code` 或 envelope error 提到 `429`、`rate limit`、`too many requests`；或 `submission.deferred=true`。

恢复：

```sh
lovart jobs status "$VERIFY_RUN_DIR" --refresh --detail requests
# 等待 60-300 秒后，用同样小预算恢复：
lovart jobs resume "$VERIFY_RUN_DIR" \
  --allow-paid \
  --max-total-credits "$VERIFY_MAX_CREDITS" \
  --submit-limit 1 \
  --max-active-tasks 1 \
  --submit-interval-seconds 10
```

命令标注：

| 命令 | execution_class | network_required | remote_write | paid_possible | 预计副作用 | 停止条件 |
| --- | --- | --- | --- | --- | --- | --- |
| `lovart jobs status ... --refresh` | preflight | true | false | false | 刷新 active tasks，避免重复提交 | 发现非本次 task 或预算异常时停止 |
| `lovart jobs resume ... --submit-interval-seconds 10` | submit | true | true | true | 降低提交频率后继续未提交 request | 429 持续或 active cap 未释放时停止等待，不要提高并发 |

### timeout

症状：`timed_out=true`、`last_error.code=timeout` 或远端任务长时间 running。

恢复：

- 先运行 `lovart jobs status "$VERIFY_RUN_DIR" --refresh --detail requests`。
- 如果远端还在 running，继续等待，不要重新提交同一 request。
- 如果 task 已完成但本地未更新，再次运行 `status --refresh` 后执行 `finalize --download`。
- 如果 request 从未拿到 `task_id` 且仍为 `pending`/`quoted`，用同样 Level 2 限制执行 `resume`。

### content policy

症状：`content_policy_rejected`、`last_error.code=content_policy_rejected`、远端 task failure 中有审核/策略拒绝证据。

恢复：

- 不要在同一 run 中盲目 `--retry-failed`。
- 修改本地 jobs.jsonl 为明确安全的 prompt，创建新的验证 run。
- 保留原 run state 用于审计；不要手工改写 state 伪造成成功。

### active cap / 并发上限

症状：`submission.deferred=true`、`deferred_reason` 提到 active cap/concurrency，或 `task list --active` 显示已有任务。

恢复：

```sh
lovart task list --active
lovart jobs status "$VERIFY_RUN_DIR" --refresh --detail requests
# 等 active task 结束后恢复；不要提高 --max-active-tasks
lovart jobs resume "$VERIFY_RUN_DIR" \
  --allow-paid \
  --max-total-credits "$VERIFY_MAX_CREDITS" \
  --submit-limit 1 \
  --max-active-tasks 1
```

命令标注：

| 命令 | execution_class | network_required | remote_write | paid_possible | 预计副作用 | 停止条件 |
| --- | --- | --- | --- | --- | --- | --- |
| `lovart task list --active` | preflight | true | false | false | 读取账号 active remote tasks | 发现非本验证任务占用时等待，不要取消他人/生产任务 |
| `lovart jobs status ... --refresh` | preflight | true | false | false | 更新本地 run state | active cap 未释放时停止等待 |
| `lovart jobs resume ... --max-active-tasks 1` | submit | true | true | true | 在并发门禁允许后继续最多 1 次提交 | 仍 deferred 时停止，不要改成更高并发 |

## 完成标准

Level 2 通过需要同时满足：

- audit JSONL 已开启且可解析。
- `jobs run` 至少创建或正确 defer 一个真实 remote request；若创建成功，submit event 数量符合 `--submit-limit`。
- `status --refresh` 能把 remote task 推进到清晰状态：`submitted`/`running`/`completed`/`failed`。
- 对 completed task，`finalize --download` 下载至少 1 个 artifact 到本地验证目录。
- 未默认执行 canvas 写回；如果执行了 canvas，必须有单独 opt-in 记录。
- 没有 secret 泄漏到 stdout、audit、README 或提交内容。
