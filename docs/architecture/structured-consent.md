# Structured Consent Architecture Design

本文定义 Lovart MCP/CLI 在高风险操作上的结构化 consent contract。目标是把“用户同意了什么”从一个通用布尔值，迁移为可审计、可比对、可逐步落地的对象级确认字段，同时不在设计阶段贸然改变现有 policy gate 行为。

## 背景与目标

Lovart 现有 MCP 安全层包含：

- `mcp/capability.go` / `mcp/policy.go`：以 `remote_write`、`paid_possible`、`destructive`、`local_filesystem_write` 等能力标记工具风险。
- `mcp/policy_gate.go`：对高风险能力要求 `confirm_policy=true` 或兼容别名 `confirm=true` 后才允许 MCP 执行。
- `mcp/tools.go`：向 MCP clients 暴露工具 schema 和 capability annotations。
- `internal/audit` 与 MCP/jobs audit：记录 attempt、policy decision、result 等安全事件。

新的 structured consent 不替代这些机制，而是在业务层补充“确认对象、金额、路径、远端目标”的机器可校验字段。核心目标：

1. 对付费、破坏性、远端写、本地写操作提供稳定字段名和匹配规则。
2. 让 MCP/CLI 可以在迁移期同时接受旧 `confirm_policy` 和新业务确认，避免破坏现有调用方。
3. 让 audit 能把一次高风险操作从 attempt 到 finish 串联起来，并记录确认字段是否匹配，而不记录 secrets 或 prompt body。
4. 保持渐进式实施：先文档和测试矩阵，再从 paid/generate/jobs 开始收敛。

## 风险类别与 consent contract

### 通用约定

所有结构化确认字段遵循以下规则：

- 字段只确认用户明确看到并批准的执行计划，不应由工具自动推断后静默填入。
- `confirmed_*` 表示调用方确认的“规范化后实际值”；服务端/CLI 必须和即将执行的值精确比对。
- `confirm_*` 表示对象级确认或布尔确认；对象级字段必须和输入对象或解析结果完全匹配。
- 缺失或不匹配时返回业务确认错误，错误里说明需要的字段、实际规范化值、风险类别；不得回显 secret 或完整 prompt body。
- 空字符串、大小写不一致、路径未规范化、集合顺序/成员不一致都视为不匹配，除非该字段显式定义了规范化规则。
- 对 MCP，高风险工具在迁移期仍必须满足 policy gate；结构化确认属于第二层业务确认。

### paid：付费/可能扣 credits 的提交

适用：`lovart_generate`、`lovart_jobs_run`、`lovart_jobs_resume`，以及未来任何可能消耗 Lovart credits 的提交。

字段：

- `confirm_paid`：布尔值。用户明确允许本次请求可能扣 credits。
- `confirmed_max_credits`：数字。必须等于将用于 gate 的 credit 上限：单次生成对应 `max_credits`；batch jobs Phase 1 使用 `confirmed_max_total_credits` / `max_total_credits`，不按单个 request enforce `confirmed_max_credits`。
- `confirmed_model`：字符串。单次生成必须等于规范化后的 generator model 名称；batch jobs Phase 1 不 enforce 该 per-request 字段。
- `confirmed_mode`：字符串。单次生成必须等于规范化后的 mode（`auto`、`fast`、`relax`）；batch jobs Phase 1 不 enforce 该 per-request 字段。
- `confirmed_outputs`：数字或结构化摘要。单次生成确认本次计划生成的输出数量；batch jobs Phase 1 不 enforce 该 per-request 字段，因为 batch 内 model/mode/outputs 可能不唯一。
- `confirmed_remote_requests`：数字。确认本次会向 Lovart 发起的远端 submit 请求数。单次生成通常为 `1`；jobs run/resume 为本次 invocation 计划提交的未完成 request 数，若设置 `submit_limit > 0`，则为 pending/quoted request 数与 `submit_limit` 的较小值。

匹配要求：

- `confirm_paid=true` 是必要条件，但不是充分条件；必须同时匹配 credit 上限和远端请求数。单次生成还必须匹配 model、mode、输出规模；batch jobs Phase 1 仅 enforce batch-level `confirm_paid` / `confirmed_max_total_credits` / `confirmed_remote_requests` / `confirmed_max_active_tasks`，当前不 enforce per-request model/mode/outputs。
- 若 quote 结果显示 zero-credit，可不要求 `confirm_paid`，但仍应保留 remote_write/project/local_write 的确认要求。
- 若实际 quote 大于 `confirmed_max_credits` 或业务上限，应在 submit 前停止。
- jobs resume 必须确认“本次可能新增提交”的请求数，不应把已经有 task id 的历史请求重复计入。

示例（MCP 参数草案）：

```json
{
  "model": "imagen-x",
  "mode": "fast",
  "allow_paid": true,
  "max_credits": 12,
  "confirm_paid": true,
  "confirmed_max_credits": 12,
  "confirmed_model": "imagen-x",
  "confirmed_mode": "fast",
  "confirmed_outputs": 4,
  "confirmed_remote_requests": 1
}
```

### destructive：破坏性或不可逆远端变更

适用：删除项目、取消任务、重命名项目、未来删除 canvas 元素等。

字段：

- `confirm_project_id`：必须等于目标 `project_id`。已在 `lovart_project_delete` 中使用，应推广为所有项目级 destructive 操作的对象确认。
- `confirm_task_ids`：字符串数组。必须和目标 `task_ids` 集合完全一致；建议排序后比对集合，错误提示中只列 task id，不列 prompt/body。
- `confirm_new_name`：字符串。用于 rename。必须等于 `new_name`；如果服务端会 trim 或规范化名称，应确认规范化后的最终名称。
- 未来对象字段遵循 `confirm_<object>_id` 或 `confirm_<object>_ids` 命名，例如 `confirm_canvas_shape_ids`。

匹配要求：

- destructiveness 来自对象语义，不只来自 capability。即使 `confirm_policy=true` 已解锁 MCP policy gate，业务层仍必须检查对象级确认。
- rename 虽非删除，但会改变远端对象标识展示，应要求 `confirm_project_id` 和 `confirm_new_name`。
- cancel task 应要求 `confirm_task_ids`，避免 agent 用过期或错误任务列表取消无关生成。

### remote_write：远端写入非付费/非 destructive

适用：项目 create/select 以外会写入 Lovart 远端状态的工具，如 `lovart_project_create`、`lovart_task_canvas`、`lovart_jobs_finalize` 的 canvas 写入、未来 canvas mutation。

字段：

- `confirmed_project_id`：明确确认将写入的项目 id。对于 project create，若目标 project 尚不存在，可确认 `confirmed_project_name` 和 `confirmed_select_after_create`。
- `confirm_current_project`：布尔值。仅当调用方没有显式传 `project_id`、工具将使用当前 selected project 时可用；必须和 audit 中记录的 resolved current project id 绑定。
- `confirmed_project_name`：项目创建或重命名时确认最终项目名。
- `confirmed_canvas_write`：布尔值。确认会把 artifacts 写回远端 canvas。

匹配要求：

- 如果参数传了 `project_id`，优先要求 `confirmed_project_id` 精确匹配，不接受只传 `confirm_current_project`。
- 如果使用当前项目，必须先 resolve current project，然后要求调用方确认“使用当前项目”或在二次调用中传入 `confirmed_project_id`。
- 对 `jobs_finalize`，只有 `canvas=true` 时触发 remote_write consent；纯 download 只走 local_write consent。

### local_write：本地文件系统写入

适用：extension install、task/canvas download、generate download、jobs run/status/resume/finalize 产生本地 run files 或下载 artifacts。

字段：

- `confirmed_canonical_path`：字符串。必须等于即将写入根路径的 canonical absolute path（解析 `~`、相对路径、symlink 之后的安全表示；若无法 resolve symlink，应至少 clean + abs）。
- `confirm_overwrite`：布尔值。仅当会覆盖现有文件或 `overwrite=true` 时必须为 true。
- `confirm_outside_default_root`：布尔值。当写入路径不在 Lovart 默认 runtime/download root 下时必须为 true。
- `confirmed_file_count`：数字。可选但建议用于批量下载，确认预计写入文件数或上限。

匹配要求：

- consent 比对使用 canonical path，不使用用户原始输入路径，防止 `../`、symlink 或 WSL/Windows 路径混淆。
- 默认 root 内的新文件写入可只要求 `confirmed_canonical_path`；覆盖或 root 外写入需额外确认。
- 任何 audit 事件不得记录文件内容、prompt body 或下载 URL 中的 credential query。

## 迁移表

| 领域/工具 | 当前风险 | 结构化 consent 字段 | 迁移说明 |
| --- | --- | --- | --- |
| `lovart_project_create` | remote_write；可能 local current project write（select） | `confirmed_project_name`，`confirmed_select_after_create`，可选 `confirm_current_project` 不适用 | Phase 2/3 前仅保留 policy gate；实施后确认创建名称和是否选中新项目。 |
| `lovart_project_select` | local_write（写当前项目上下文） | `confirmed_project_id`，`confirmed_canonical_path`（context store） | 先确认 project id；路径字段可在 local_write phase 覆盖。 |
| `lovart_project_rename` | remote_write，destructive-like | `confirm_project_id`，`confirm_new_name` | 作为对象级变更处理；`confirm_policy` 不替代名称确认。 |
| `lovart_project_delete` | destructive remote_write | `confirm_project_id` | 已有字段继续保留；迁移期将其作为业务确认并可顺带解锁 policy gate。 |
| `lovart_task_cancel` | destructive remote_write | `confirm_task_ids` | 必须匹配目标 task id 集合。 |
| `lovart_task_canvas` | remote_write | `confirmed_project_id` 或 `confirm_current_project=true`，`confirmed_canvas_write=true` | 若传 project override，必须确认该 id；若用 current project，记录 resolved id。 |
| `lovart_task_download` | local_write | `confirmed_canonical_path`，`confirm_overwrite`，`confirm_outside_default_root`，可选 `confirmed_file_count` | 对下载目录和覆盖行为做路径级确认。 |
| `lovart_canvas_artifacts` / `lovart_canvas_artifact` | remote_read | 无结构化 consent | 只读，不新增业务确认。 |
| `lovart_canvas_download` | local_write + remote_read | `confirmed_canonical_path`，`confirm_overwrite`，`confirm_outside_default_root`，`confirmed_file_count` | 选择 `all=true` 时建议要求 file count 或 artifact count 确认。 |
| 未来 canvas mutation | remote_write；部分 destructive | `confirmed_project_id` 或 `confirm_current_project`，`confirm_canvas_shape_ids`，`confirmed_canvas_write=true` | Phase 3 后统一 canvas 对象确认。 |
| `lovart_quote` | remote_read | 无结构化 consent | quote 是 paid gate 的输入，不提交。 |
| `lovart_generate` | paid_possible，remote_write，local_write（可选 download），canvas remote_write（可选） | paid 字段全集；`confirmed_project_id`/`confirm_current_project`；download 时 local_write 字段；canvas 时 `confirmed_canvas_write` | Phase 1 首批落地，先保护 submit 和 paid quote。 |
| `lovart_jobs_run` | paid_possible，remote_write，local_write | paid 字段全集（batch 语义），`confirmed_project_id`/`confirm_current_project`，`confirmed_canonical_path` | 确认本次计划 submit 数和 max total credits。 |
| `lovart_jobs_resume` | paid_possible，remote_write，local_write | paid 字段全集（仅未提交 job），`confirmed_canonical_path`，`confirm_overwrite`（如会改写 run state） | 不重复确认已存在 task id；只确认新增提交。 |
| `lovart_jobs_status` | remote_read + local_write（刷新 state） | `confirmed_canonical_path`（run dir） | refresh 写本地 state 时确认 run dir；可低优先级迁移。 |
| `lovart_jobs_finalize` | local_write；canvas=true 时 remote_write | download 时 local_write 字段；canvas 时 `confirmed_project_id`/`confirm_current_project` + `confirmed_canvas_write` | finalize 不 submit paid jobs，因此不需要 paid 字段，除非未来加入重试提交。 |
| `lovart_extension_install` | local_write | `confirmed_canonical_path`，`confirm_overwrite`，`confirm_outside_default_root` | 安装/覆盖扩展目录需要路径确认。 |
| `lovart_auth_login` | remote_write/session handoff | connector pairing phase 可选：`confirm_connector_pairing`，`confirmed_callback_origin` | 不在首批范围；保持现有 policy gate。 |

## 与现有 `confirm_policy` 的关系

`confirm_policy` 是 MCP policy gate 的通用解锁信号：它表示 MCP client 声明“用户已经批准执行这个带副作用的工具调用”。它不包含业务对象、credit 上限、路径、项目 id 等可比对信息。

structured consent 是业务确认：它回答“用户批准的是哪个对象、多少 credits、哪个模型/模式、哪个项目、哪个本地路径”。因此：

- `confirm_policy=true` 只能解锁 MCP capability gate，不替代 `confirm_paid`、`confirm_project_id`、`confirmed_canonical_path` 等业务确认。
- 业务确认字段可以在迁移期被 `hasBusinessConfirmation` 用来兼容解锁部分 policy gate，但这只是为了减少重复确认；执行前仍必须做完整业务比对。
- CLI 没有 MCP policy gate，也应使用同一套业务确认字段/flags 语义，保证 MCP 与 CLI 行为一致。
- 对旧调用方，迁移期保留 `confirm_policy` 和 `confirm` alias；新增字段先进入 schema、错误提示和 audit，再逐步从 warning 变为 required。

迁移兼容策略：

1. 文档期：不改变运行时行为，只定义字段和测试矩阵。
2. Soft gate：工具 schema 暴露新字段；缺失时返回更具体的 recommended actions，或在 CLI 中 warning。
3. Dual gate：接受 `confirm_policy` + 结构化字段；对已存在的对象级字段（如 `confirm_project_id`）继续兼容。
4. Strict business gate：高风险业务操作必须匹配结构化字段；`confirm_policy` 仍作为 MCP 外层 gate 存在。
5. Deprecation：长期可保留 `confirm` alias，但文档和错误提示只推荐 `confirm_policy` + structured consent。

## Audit 关联要求

所有高风险操作应通过 `event_id` 和 `correlation_id` 串联。建议：

- `event_id`：每条 audit event 的唯一 id。
- `correlation_id`：一次用户意图/工具调用/CLI command 的稳定 id，从 attempt 贯穿 finish；jobs 中每个 row/task 可额外有 `job_id` 或 `task_id`。
- MCP request id（若有）可作为 metadata，但不替代 `correlation_id`。

推荐事件链：

1. `attempt`：记录工具/命令名、risk flags、参数摘要、是否提供 structured consent 字段；不记录 secrets、cookie、token、prompt body、完整 request body。
2. `policy`：记录 MCP policy decision、是否由 `confirm_policy` 或业务确认解锁。
3. `quote` / `gate`：记录 quote credits、max credits、paid gate decision、确认字段匹配状态。字段状态只记录 `present`、`matched`、`mismatched`、`not_required`，必要时记录 hash 或安全摘要，不记录 prompt body。
4. `submit` / `write`：记录远端 submit、canvas write、本地 write 的目标摘要，例如 project id、task id、canonical path、file count、overwrite flag。
5. `finish`：记录 ok/error、错误 code、远端 task id 或本地输出摘要。

确认字段匹配状态建议结构：

```json
{
  "consent": {
    "schema_version": "structured-consent/v1",
    "paid": {
      "confirm_paid": "matched",
      "confirmed_max_credits": "matched",
      "confirmed_model": "matched",
      "confirmed_mode": "matched",
      "confirmed_outputs": "matched",
      "confirmed_remote_requests": "matched"
    },
    "remote_write": {
      "confirmed_project_id": "matched"
    },
    "local_write": {
      "confirmed_canonical_path": "matched",
      "confirm_overwrite": "not_required",
      "confirm_outside_default_root": "not_required"
    }
  }
}
```

隐私/安全要求：

- 不记录 secrets：cookie、token、CSRF、签名材料、auth headers、connector payload。
- 不记录 prompt body 或完整 generation body；可记录 model、mode、schema field count、body hash、request count。
- 不记录下载 URL 中的敏感 query；只记录 host、artifact id、安全文件名或 hash。
- audit error details 不应包含原始 prompt、完整 jobs JSONL 行或本地文件内容。

## 实施阶段

### Phase 0：文档 + 测试矩阵

交付：本文档、工具/CLI 测试矩阵、schema 草案。

测试矩阵至少覆盖：

- MCP policy gate：无 `confirm_policy` 阻止，高风险有 `confirm_policy` 可进入业务 gate。
- paid：缺失每个 paid 字段、不匹配 model/mode/credits/request count、zero-credit quote、超过 max credits。
- destructive：project delete id 不匹配、task cancel ids 少/多/乱序、rename name 不匹配。
- remote_write：显式 project id vs current project acknowledgment。
- local_write：相对路径规范化、root 外路径、overwrite、批量下载 file count。
- audit：每条链路有 correlation id；确认字段状态出现；secrets/prompt body 不出现。

### Phase 1：paid / generate / jobs

范围：`lovart_generate`、`lovart_jobs_run`、`lovart_jobs_resume`。

- 在 MCP schema 和 CLI flags/options 加 paid 字段。
- CLI Phase 1 flags：
  - `lovart generate`：`--confirm-paid`、`--confirmed-max-credits`、`--confirmed-model`、`--confirmed-mode`、`--confirmed-outputs`、`--confirmed-remote-requests`。
  - `lovart jobs run|resume`：`--confirm-paid`、`--confirmed-max-total-credits`、`--confirmed-remote-requests`、`--confirmed-max-active-tasks`。
- 在 quote/gate 后、submit 前做 paid consent 比对。
- jobs 使用“本次新增 submit 请求数”和 total credits 上限。
- audit 写入 quote/gate/submit/finish 的 correlation chain。
- 保持 `confirm_policy` 行为不变。

### Phase 2：destructive / task / project

范围：`lovart_project_delete`、`lovart_project_rename`、`lovart_task_cancel`，以及项目 select/create 的基础 remote/local 确认。

- 将 `confirm_project_id` 规范化为 destructive object confirmation。
- 为 task cancel 增加 `confirm_task_ids`。
- 为 rename 增加 `confirm_new_name`。
- 对错误提示和 audit 增加对象级 mismatch 状态。

### Phase 3：local_write / canvas

范围：downloads、extension install、jobs finalize/status run dir、task canvas、canvas download 和未来 canvas mutation。

- 引入 canonical path resolver 和 default root 判定。
- 覆盖/默认 root 外写入必须额外确认。
- canvas write 统一要求 project acknowledgment 和 `confirmed_canvas_write`。
- 对 WSL/Windows 路径转换、symlink、相对路径建立测试。

### Phase 4：connector pairing（可选）

范围：`lovart_auth_login`、Connector extension pairing/session handoff。

- 可增加 `confirm_connector_pairing`、`confirmed_callback_origin`、`confirmed_browser_profile` 等字段。
- 重点是防止错误浏览器/profile/session 交接，而不是记录任何 credential。
- 该阶段不阻塞 paid/destructive/local_write 的落地。

## 非目标

- 本设计不要求一次性重构所有 MCP tools 或 policy gate。
- 本设计不改变 Lovart 远端 API 协议。
- 本设计不要求在 audit 中保存 prompts、完整 request bodies 或 secret-bearing headers。
- 本设计不把 `confirm_policy` 废弃；它仍是 MCP capability gate 的外层信号。

## 成功标准

- 高风险操作的“用户确认了什么”可以由字段精确还原和验证。
- 旧 MCP clients 在迁移期不会因为文档落地立即失效。
- paid submit、destructive object mutation、remote project/canvas write、本地路径写入都具备独立测试用例。
- audit 能用 `correlation_id` 串起 attempt -> policy -> quote/gate -> submit/write -> finish，且不泄露 secrets 或 prompt body。
