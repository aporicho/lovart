# Lovart

Lovart 是一个用于从本地工具和 agent 操作 Lovart AI 图像生成平台的 Go
CLI 和 stdio MCP server。单个 `lovart` 二进制文件负责浏览器会话认证交接、
runtime metadata 和 signer cache、请求校验、价格预检、生成提交、产物下载、
项目画布回写、批量任务以及 MCP 工具暴露。

本仓库还包含 Chrome MV3 连接器扩展、release 安装脚本、架构文档，以及在开发
协议集成时使用的 Lovart HTTP/API 抓包参考材料。

## 功能

- 单个静态 Go 二进制文件，同时支持 CLI 和 MCP server。
- 面向自动化命令的 JSON stdout 契约：操作返回 `ok` envelope 或 error envelope。
- 基于浏览器扩展的认证流程，可将当前 Lovart 浏览器会话转交给本地 runtime，
  且不会打印 secret。
- 从 Lovart 前端/API 来源同步 runtime signer WASM 和 generator metadata。
- 在发起签名网络请求前，使用本地 model registry 和 schema 做请求校验。
- 支持 `auto`、`fast`、`relax` 的 mode-aware pricing。
- 通过 `--allow-paid` 和 credit limit 显式控制付费请求。
- 支持单次生成，并可选择等待、下载和项目画布回写。
- 支持可恢复状态、产物路由和画布分组的批量 JSONL jobs。
- 为 agent 暴露包含 19 个公共 Lovart 工具的 MCP server。
- 提供 upgrade 和 uninstall 自管理命令。

## 要求

- 从源码构建需要 Go `1.26.1`。
- 需要 Lovart 账号，并能访问 `www.lovart.ai` 和 `lgw.lovart.ai`。
- 正常连接器扩展登录流程需要 Google Chrome 或兼容 Chromium 的浏览器。
- release installer 优先用公开 release 直链下载；私有 fork 或公开下载受限时，
  fallback 到 GitHub CLI (`gh`) 并要求 `gh auth login`。
- 支持的 release assets 为 macOS arm64、Linux x64 和 Windows x64。

## 从源码快速开始

构建 CLI：

```sh
go build -o dist/lovart ./cmd/lovart
dist/lovart --version
dist/lovart self-test
```

安装或加载连接器扩展：

```sh
make ext-build
lovart extension install --yes --open
```

从源码 checkout 使用时，在 `chrome://extensions` 中启用 Developer mode，然后加载
`extension/` 目录。release installer 会把扩展解压到
`~/.lovart/extension/lovart-connector`；从 release 安装时加载该目录。Chrome 普通用户
环境仍需要在扩展页启用 Developer mode 并手动 Load unpacked，CLI/MCP 不会绕过
Chrome 的扩展信任流程。

在普通 Linux 桌面环境中，`lovart` 会尝试用本机 Chrome/Chromium 或 `xdg-open`
打开登录页和扩展页。在 WSL 中，`lovart` 会优先使用检测到的 Windows opener
（例如 `wslview`、`cmd.exe`、`powershell.exe` 或 `explorer.exe`）打开 Windows
浏览器；如果自动打开失败，命令输出会包含可手动打开的 URL。
推荐在 WSL 中安装 `wslu` 来提供标准 `wslview`：

```sh
sudo apt update
sudo apt install -y wslu
```

连接 Lovart 认证并准备 runtime cache：

```sh
lovart auth login
lovart setup
lovart doctor
lovart project list
lovart project select <project_id>
```

`lovart auth login` 会在 `127.0.0.1` 的 `47821` 到 `47830` 端口范围启动一次性本地
callback，打开 Lovart，并等待连接器扩展发送用户批准的浏览器会话。保存的凭据文件
属于私有 runtime state，status 命令不会打印其中的 secret。

## 从 Release Assets 安装

installer 脚本会优先使用 `curl` 或 `Invoke-WebRequest` 通过公开 GitHub release
直链下载 assets；如果公开下载失败，会 fallback 到 `gh release download`。私有 fork
或 API 受限场景需要先运行 `gh auth login`。脚本会校验 `SHA256SUMS`，安装二进制文件，
安装连接器扩展文件，并可选择配置本地 MCP client。

远程一键安装：

```sh
/bin/bash -c "$(curl -fsSL https://github.com/aporicho/lovart/releases/latest/download/install.sh)" -- --yes --force
```

远程预览安装：

```sh
/bin/bash -c "$(curl -fsSL https://github.com/aporicho/lovart/releases/latest/download/install.sh)" -- --dry-run --json
```

如果已经 checkout 源码或下载了 release installer，也可以运行本地脚本。

预览安装：

```sh
bash packaging/install/install.sh --dry-run --json
pwsh -File packaging/install/install.ps1 -DryRun -Json
```

在 Unix-like 系统上安装：

```sh
bash packaging/install/install.sh --yes --force
```

在 Windows 上安装：

```powershell
pwsh -File packaging/install/install.ps1 -Yes -Force
```

默认值：

- Unix binary path: `~/.local/bin/lovart`
- Windows binary path: `%USERPROFILE%\bin\lovart.exe`
- Extension path: `~/.lovart/extension/lovart-connector`
- MCP clients: `auto`

release installer 会安装 Lovart Connector 扩展文件，但 Chrome 仍需要用户在
`chrome://extensions` 中 Load unpacked。可用下面的命令重新打开扩展页并查看加载路径：

```sh
lovart extension status
lovart extension open
```

在 WSL 中，`lovart extension status` 会在可用时输出 `windows_extension_dir`，Load
unpacked 时优先选择该 Windows 可读路径；如果没有该字段，可安装 `wslu` 提供
`wslview`，或手动使用 `wslpath -w ~/.lovart/extension/lovart-connector` 转换路径。
如果 Windows 弹出“获取打开此 `chrome` 链接的应用”，不要进入 Microsoft Store；
请直接打开 Chrome，在地址栏输入 `chrome://extensions/`。

## Runtime Data

runtime root 默认是 `~/.lovart`。测试或隔离自动化运行时可用 `LOVART_HOME` 覆盖。

重要路径：

| 路径 | 用途 |
| --- | --- |
| `creds.json` | 浏览器会话凭据和已选项目上下文 |
| `signing/current.wasm` | runtime Lovart request signer |
| `signing/manifest.json` | signer source URL 和 hash metadata |
| `metadata/generator_list.json` | cached generator list |
| `metadata/generator_schema.json` | cached generator OpenAPI schema |
| `metadata/manifest.json` | metadata hashes 和 sync time |
| `runs/` | 批量 job state |
| `downloads/` | 已下载产物 |
| `tmp/` | runtime 临时文件 |
| `extension/lovart-connector/` | 已安装的连接器扩展文件 |

`lovart clean` 默认预览全部 scope。破坏性清理需要显式指定 scope 并传入 `--yes`：

```sh
lovart clean
lovart clean --cache --dry-run
lovart clean --runs --downloads --yes
lovart clean --all --yes
```

## JSON Envelope 契约

面向自动化的 CLI 命令会打印紧凑 JSON。success envelope 包含 execution metadata，
方便 agent 区分本地读取、远端检查和远端写入：

```json
{
  "ok": true,
  "data": {},
  "execution_class": "local",
  "network_required": false,
  "remote_write": false,
  "cache_used": true
}
```

execution classes：

- `local`: 读取本地文件、凭据、registry data 或已保存的 job state。
- `preflight`: 执行网络读取或根据远端状态做校验，但不创建 generation task，
  也不修改 project。
- `submit`: 执行远端写入，例如创建 task 或修改 project。

错误使用稳定 code，例如 `input_error`、`auth_missing`、`metadata_stale`、
`signer_stale`、`schema_invalid`、`credit_risk`、`task_failed`、`timeout`、
`network_unavailable` 和 `internal_error`。

## 核心 CLI 命令

readiness 和 runtime：

```sh
lovart --version
lovart self-test
lovart setup
lovart doctor
lovart doctor --online
lovart update check
lovart update sync --all
lovart update sync --signer
lovart update sync --metadata-only
```

认证：

```sh
lovart auth status
lovart auth login
lovart auth login --timeout-seconds 300
lovart auth logout --yes
```

项目：

```sh
lovart project current
lovart project list
lovart project create --name "Campaign concepts"
lovart project select <project_id>
lovart project show [project_id]
lovart project open [project_id]
lovart project admin rename <project_id> "New name"
lovart project admin delete <project_id>
lovart project admin repair-canvas [project_id]
```

models、config 和 pricing：

```sh
lovart models
lovart models --refresh
lovart config openai/gpt-image-2
lovart config openai/gpt-image-2 --all
lovart balance
lovart quote openai/gpt-image-2 --body-file request.json --mode relax
```

单次生成：

```sh
lovart generate openai/gpt-image-2 --prompt "a clean product render" --mode relax
lovart generate openai/gpt-image-2 --body-file request.json --allow-paid --max-credits 20
lovart generate openai/gpt-image-2 --body-file request.json --no-wait
lovart generate openai/gpt-image-2 --body-file request.json --no-download --no-canvas
```

生成需要已认证会话、已同步的 signer 和 metadata，以及已选择的项目上下文。除非显式
禁用，完成后的产物会被下载，并回写到已选择的 Lovart project canvas。

## 请求 Body

使用 `lovart config <model>` 查看某个 model 的合法字段。request body 是 JSON
object，会先根据 cached model schema 校验，再发起签名网络请求。

最小示例：

```json
{
  "prompt": "a clean product render on a neutral background"
}
```

对付费请求，先 quote，再显式 opt in：

```sh
lovart quote openai/gpt-image-2 --body-file request.json
lovart generate openai/gpt-image-2 --body-file request.json --allow-paid --max-credits 20
```

## Batch Jobs

batch jobs 使用 newline-delimited JSON。每一行描述一个用户级 job：

```jsonl
{"job_id":"hero-001","title":"Hero / Blue","fields":{"folder":"hero","scene_no":"01"},"model":"openai/gpt-image-2","mode":"relax","outputs":2,"body":{"prompt":"blue ceramic mug on a white table"}}
{"job_id":"hero-002","model":"openai/gpt-image-2","body":{"prompt":"red ceramic mug on a white table"}}
```

运行、恢复和查看：

```sh
lovart jobs run jobs.jsonl --allow-paid --max-total-credits 100
lovart jobs resume ~/.lovart/runs/jobs-<hash> --retry-failed
lovart jobs status ~/.lovart/runs/jobs-<hash>
lovart jobs status ~/.lovart/runs/jobs-<hash> --refresh --detail full
```

默认值：

- `mode` 默认是 `auto`。
- `outputs` 默认是 `1`。
- `outputs` 不能和 body quantity fields 同时使用，例如 `n`、`max_images`、
  `num_images` 或 `count`。
- runs 会保存到 `~/.lovart/runs/<jobs-stem>-<hash>`。
- downloads 默认写入 `~/.lovart/downloads`，除非设置 `--download-dir`。
- download directories 默认是 `{{job.folder}}`。
- download filenames 默认是 `artifact-{{artifact.index:02}}.{{ext}}`。

支持的 download template variables 包括 `model`、`mode`、`ext`、`job.id`、
`job.title`、`job.folder`、`task.id`、`artifact.index`、`artifact.width`、
`artifact.height`，以及嵌套的 `fields.*` 值。

下载的 PNG、JPEG、WebP 和 GIF 文件在格式支持时会写入 Lovart effect metadata。

## MCP Server

运行 stdio MCP server：

```sh
lovart mcp
```

查看或安装 client 配置：

```sh
lovart mcp status
lovart mcp install --clients auto --yes
lovart mcp install --clients codex,claude --dry-run
```

支持的 MCP clients 是 `codex`、`claude`、`opencode` 和 `openclaw`。`lovart mcp status`
也会打印手动配置 Codex 的 TOML 片段。

MCP 也暴露安全的登录和扩展准备工具：`lovart_auth_login`、
`lovart_extension_status`、`lovart_extension_install` 和 `lovart_extension_open`。
这些工具不会返回 cookie、token、CSRF 或 CID。
`lovart_auth_login` 会返回 pending 登录 URL，并在后台继续等待连接器扩展回调；
如果浏览器没有自动打开，手动打开 `login_url` 后点击页面里的 Connect，再调用
`lovart_auth_status` 确认 `available=true`。

运行 agent-style smoke check：

```sh
lovart mcp smoke
lovart mcp smoke --model openai/gpt-image-2 --prompt "smoke test" --mode relax
lovart mcp smoke --submit --allow-paid --max-credits 5
```

smoke 命令会通过 stdio 启动本地 MCP server，检查 JSON-RPC `initialize`、`ping`
和 `tools/list`，然后执行安全工具。除非提供
`--submit --allow-paid --max-credits N`，否则不会提交生成。

MCP server 声明 protocol version `2024-11-05`，并提供 19 个 tools：

```text
lovart_auth_status
lovart_setup
lovart_models
lovart_config
lovart_balance
lovart_project_current
lovart_project_list
lovart_project_create
lovart_project_select
lovart_project_show
lovart_project_open
lovart_project_rename
lovart_project_delete
lovart_project_repair_canvas
lovart_quote
lovart_generate
lovart_jobs_run
lovart_jobs_status
lovart_jobs_resume
```

## Upgrade 和 Uninstall

检查或从 GitHub releases 应用升级：

```sh
lovart upgrade --check
lovart upgrade --dry-run
lovart upgrade --yes
lovart upgrade --version vX.Y.Z --yes
```

预览或移除本地安装：

```sh
lovart uninstall --dry-run
lovart uninstall --yes
lovart uninstall --yes --data
lovart uninstall --yes --keep-mcp --keep-extension
```

在 Windows 上，self-upgrade 和 uninstall 可能需要手动替换或删除正在运行的 binary。

## Developer Commands

developer-only diagnostics 位于 `lovart dev` 下：

```sh
lovart dev sign
lovart dev auth-login
lovart dev auth-login --timeout-seconds 90 --debug-port 47831
```

`dev auth-login` 会以 DevTools debugging port 重启 Chrome，捕获 Lovart 浏览器会话，
根据 Lovart project APIs 验证后，写入和扩展流程相同的 runtime credential file。
普通用户应优先使用 `lovart auth login`。

## 仓库结构

```text
.
|-- cmd/lovart/          # main binary, MCP command, upgrade/uninstall commands
|-- cli/                 # Cobra command tree and CLI JSON envelope helpers
|-- mcp/                 # stdio JSON-RPC MCP server and tool definitions
|-- internal/            # protocol, runtime, auth, signing, jobs, project logic
|-- extension/           # Chrome MV3 Lovart Connector extension
|-- packaging/install/   # Bash and PowerShell release installers
|-- docs/                # architecture notes and module rules
|-- scripts/             # repository checks
|-- captures/            # Lovart HTTP/API capture reference material
|-- captures_backup/     # archived capture reference material
|-- .github/workflows/   # release build workflow
|-- Makefile
|-- go.mod
`-- go.sum
```

internal packages 按领域拆分：

| Package | 职责 |
| --- | --- |
| `internal/auth` | 凭据存储、login callback、project context |
| `internal/signing` | WASM signer 加载和 request signature headers |
| `internal/http` | signed Lovart API client |
| `internal/update` | signer 和 generator metadata drift/sync |
| `internal/metadata` | runtime metadata files 和 stable hashes |
| `internal/registry` | model registry 和 schema validation |
| `internal/config` | 面向用户的 model config fields |
| `internal/pricing` | credit quotes 和 mode-aware effective pricing |
| `internal/generation` | preflight、submit 和 task polling |
| `internal/project` | project CRUD 和 canvas mutation/writeback |
| `internal/jobs` | JSONL batch parsing、gates、run/resume/status |
| `internal/downloads` | artifact persistence、routing、index、metadata embed |
| `internal/selftest` / `internal/setup` | 本地 readiness diagnostics |
| `internal/selfmanage` | upgrade 和 uninstall flows |

架构变更应遵循 `docs/architecture/file-architecture-philosophy.md`；
`scripts/check_architecture.sh` 会检查命名、依赖方向、generated-file pollution 和
build health。

## 开发

常用 targets：

```sh
make build
make test
make lint
make check
make release VERSION=vX.Y.Z
```

直接检查：

```sh
go test ./...
go test -race -count=1 ./...
go vet ./...
bash scripts/check_architecture.sh
```

release build 由 `.github/workflows/release-binaries.yml` 处理。workflow 会测试仓库，
构建 macOS arm64、Linux x64 和 Windows x64 binaries，smoke-test 生成的 binaries，
打包连接器扩展，添加 installers，写入 `SHA256SUMS`，并为匹配 `v*` 的 tag 上传
release assets。

## Troubleshooting

- `auth_missing`: 运行 `lovart auth login`，然后运行 `lovart auth status`。
- `signer_stale`: 运行 `lovart update sync --signer` 或 `lovart update sync --all`。
- `metadata_stale`: 运行 `lovart update sync --metadata-only` 或
  `lovart update sync --all`。
- `schema_invalid`: 运行 `lovart config <model>` 并更新 request body。
- `credit_risk`: 先 quote 请求，然后用合适的 `--max-credits` 或
  `--max-total-credits` 配合 `--allow-paid` 重新运行。
- 缺少 project context：运行 `lovart project list` 和
  `lovart project select <project_id>`。
- MCP client 未检测到 tools：运行 `lovart mcp status`，重新运行
  `lovart mcp install --clients <client> --yes`，然后重启 client。
