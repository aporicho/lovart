package mcp

// Tool describes one MCP tool.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

func stringSchema(description string) map[string]any {
	schema := map[string]any{"type": "string"}
	if description != "" {
		schema["description"] = description
	}
	return schema
}

func numberSchema(description string) map[string]any {
	schema := map[string]any{"type": "number"}
	if description != "" {
		schema["description"] = description
	}
	return schema
}

func boolSchema(description string) map[string]any {
	schema := map[string]any{"type": "boolean"}
	if description != "" {
		schema["description"] = description
	}
	return schema
}

func stringArraySchema(description string) map[string]any {
	schema := map[string]any{
		"type":  "array",
		"items": map[string]any{"type": "string"},
	}
	if description != "" {
		schema["description"] = description
	}
	return schema
}

func detailSchema() map[string]any {
	return map[string]any{"type": "string", "enum": []string{"summary", "requests", "full"}}
}

func modeSchema() map[string]any {
	return map[string]any{"type": "string", "enum": []string{"auto", "fast", "relax"}}
}

func artifactDetailSchema() map[string]any {
	return map[string]any{"type": "string", "enum": []string{"summary", "full"}}
}

func tool(name, description string, properties map[string]any, required ...string) Tool {
	if required == nil {
		required = []string{}
	}
	return Tool{
		Name:        name,
		Description: description,
		InputSchema: map[string]any{
			"type":                 "object",
			"properties":           properties,
			"required":             required,
			"additionalProperties": false,
		},
	}
}

// Tools returns the safe public MCP tool list.
func Tools() []Tool {
	return []Tool{
		tool("lovart_auth_status", "Show Lovart auth status without exposing secrets.", map[string]any{}),
		tool("lovart_auth_login", "Open Lovart in the browser and wait for the Connector extension to authenticate without exposing secrets.", map[string]any{
			"timeout_seconds": numberSchema("seconds to wait for browser extension authentication; defaults to 300"),
		}),
		tool("lovart_extension_status", "Show local Lovart Connector extension file status.", map[string]any{
			"extension_dir": stringSchema("extension install directory"),
		}),
		tool("lovart_extension_install", "Prepare Lovart Connector extension files and open Chrome extension management.", map[string]any{
			"source_dir":    stringSchema("source unpacked extension directory"),
			"extension_dir": stringSchema("extension install directory"),
			"dry_run":       boolSchema("preview changes without writing files"),
			"open":          boolSchema("open chrome://extensions after preparing files; defaults to true"),
		}),
		tool("lovart_extension_open", "Open Chrome extension management for loading the Lovart Connector.", map[string]any{
			"extension_dir": stringSchema("extension install directory"),
		}),
		tool("lovart_setup", "Run Lovart readiness checks without exposing secrets.", map[string]any{}),
		tool("lovart_models", "List known Lovart generator models; pass refresh=true to fetch current remote metadata.", map[string]any{
			"refresh": boolSchema("fetch current remote metadata"),
		}),
		tool("lovart_config", "Return legal config values for one model.", map[string]any{
			"model":       stringSchema("Lovart generator model name"),
			"include_all": boolSchema("include non-user-facing metadata fields"),
		}, "model"),
		tool("lovart_balance", "Return the current Lovart credit balance.", map[string]any{}),
		tool("lovart_project_current", "Return the selected Lovart project context without exposing secrets.", map[string]any{}),
		tool("lovart_project_list", "List Lovart projects available to the current account.", map[string]any{}),
		tool("lovart_project_create", "Create a new Lovart project and optionally select it for future generation.", map[string]any{
			"name":   stringSchema("optional project name"),
			"select": boolSchema("select the new project after creation; defaults to true"),
		}),
		tool("lovart_project_select", "Select the Lovart project used by generation tools.", map[string]any{
			"project_id": stringSchema("Lovart project id"),
		}, "project_id"),
		tool("lovart_project_show", "Show Lovart project details; defaults to the selected project.", map[string]any{
			"project_id": stringSchema("optional Lovart project id"),
		}),
		tool("lovart_project_open", "Open a Lovart project in the local browser; defaults to the selected project.", map[string]any{
			"project_id": stringSchema("optional Lovart project id"),
		}),
		tool("lovart_project_rename", "Rename a Lovart project.", map[string]any{
			"project_id": stringSchema("Lovart project id"),
			"new_name":   stringSchema("new project name"),
		}, "project_id", "new_name"),
		tool("lovart_project_delete", "Delete a Lovart project; project_id and confirm_project_id must match.", map[string]any{
			"project_id":         stringSchema("Lovart project id"),
			"confirm_project_id": stringSchema("must exactly match project_id"),
		}, "project_id", "confirm_project_id"),
		tool("lovart_task_list", "List active Lovart generation tasks currently occupying the account's task pool.", map[string]any{
			"active": boolSchema("list active running tasks; defaults to true"),
		}),
		tool("lovart_task_cancel", "Cancel active Lovart generation tasks by task id. Use only when the user explicitly wants tasks stopped.", map[string]any{
			"task_ids": stringArraySchema("Lovart generation task ids to cancel"),
		}, "task_ids"),
		tool("lovart_task_status", "Read the status of one Lovart generation task. Failed remote tasks return ok=true with data.failure.", map[string]any{
			"task_id": stringSchema("Lovart generation task id"),
			"detail":  artifactDetailSchema(),
		}, "task_id"),
		tool("lovart_task_wait", "Wait for one Lovart generation task to reach a terminal status.", map[string]any{
			"task_id":         stringSchema("Lovart generation task id"),
			"detail":          artifactDetailSchema(),
			"timeout_seconds": numberSchema("seconds to wait before returning timeout; defaults to 90"),
			"poll_interval":   numberSchema("seconds between status polls; defaults to 2"),
		}, "task_id"),
		tool("lovart_task_canvas", "Write completed task artifacts back to the selected Lovart project canvas.", map[string]any{
			"task_id":    stringSchema("Lovart generation task id"),
			"project_id": stringSchema("optional project override"),
			"detail":     artifactDetailSchema(),
		}, "task_id"),
		tool("lovart_task_download", "Download artifacts from a completed Lovart generation task.", map[string]any{
			"task_id":                stringSchema("Lovart generation task id"),
			"artifact_index":         numberSchema("optional 1-based artifact index to download"),
			"download_dir":           stringSchema("artifact download directory"),
			"download_dir_template":  stringSchema("download subdirectory template"),
			"download_file_template": stringSchema("download filename template"),
			"overwrite":              boolSchema("replace existing downloaded files"),
			"detail":                 artifactDetailSchema(),
		}, "task_id"),
		tool("lovart_canvas_artifacts", "List downloadable image artifacts on a Lovart project canvas.", map[string]any{
			"project_id": stringSchema("optional project id; defaults to current project"),
			"task_id":    stringSchema("filter artifacts by generation task id"),
			"limit":      numberSchema("maximum number of artifacts to return"),
			"offset":     numberSchema("number of artifacts to skip"),
			"detail":     artifactDetailSchema(),
		}),
		tool("lovart_canvas_artifact", "Return details for one downloadable canvas image artifact.", map[string]any{
			"project_id":  stringSchema("optional project id; defaults to current project"),
			"artifact_id": stringSchema("canvas artifact id"),
			"include_raw": boolSchema("include raw canvas shape JSON"),
		}, "artifact_id"),
		tool("lovart_canvas_download", "Download image artifacts from a Lovart project canvas.", map[string]any{
			"project_id":             stringSchema("optional project id; defaults to current project"),
			"artifact_id":            stringSchema("download one canvas artifact by artifact_id"),
			"artifact_index":         numberSchema("download one canvas artifact by 1-based list index"),
			"task_id":                stringSchema("download artifacts associated with a generation task id"),
			"all":                    boolSchema("download all canvas image artifacts"),
			"original":               boolSchema("prefer originalUrl when available"),
			"download_dir":           stringSchema("artifact download directory"),
			"download_dir_template":  stringSchema("download subdirectory template"),
			"download_file_template": stringSchema("download filename template"),
			"overwrite":              boolSchema("replace existing downloaded files"),
		}),
		tool("lovart_quote", "Fetch an exact Lovart credit quote for a model request.", map[string]any{
			"model": stringSchema("Lovart generator model name"),
			"body":  map[string]any{"type": "object"},
			"mode":  modeSchema(),
		}, "model", "body"),
		tool("lovart_generate", "Submit a single generation request after the normal paid/zero-credit gate. Defaults to asynchronous submission; use lovart_task_wait for the normal flow.", map[string]any{
			"model":                  stringSchema("Lovart generator model name"),
			"body":                   map[string]any{"type": "object"},
			"mode":                   modeSchema(),
			"allow_paid":             boolSchema("allow paid generation"),
			"max_credits":            numberSchema("maximum credits allowed when allow_paid=true"),
			"project_id":             stringSchema("optional project override"),
			"wait":                   boolSchema("wait for completion; defaults to false for MCP"),
			"download":               boolSchema("download artifacts after completion"),
			"canvas":                 boolSchema("write completed artifacts back to project canvas"),
			"download_dir":           stringSchema("artifact download directory"),
			"download_dir_template":  stringSchema("download subdirectory template"),
			"download_file_template": stringSchema("download filename template"),
		}, "model", "body"),
		tool("lovart_jobs_run", "Submit a batch generation from a user-level jobs JSONL file and return the run directory for status polling.", map[string]any{
			"jobs_file":               stringSchema("path to jobs.jsonl"),
			"allow_paid":              boolSchema("allow paid batch generation"),
			"max_total_credits":       numberSchema("maximum total credits allowed when allow_paid=true"),
			"project_id":              stringSchema("optional project override"),
			"download_dir":            stringSchema("artifact download directory"),
			"submit_interval_seconds": numberSchema("seconds to wait between batch task submissions; defaults to 2"),
			"submit_limit":            numberSchema("maximum submit attempts in this MCP call; 0 means unlimited"),
			"max_active_tasks":        numberSchema("maximum active Lovart tasks allowed before deferring; defaults to 10, 0 disables"),
		}, "jobs_file"),
		tool("lovart_jobs_status", "Read local batch run state.", map[string]any{
			"run_dir": stringSchema("batch run directory"),
			"detail":  detailSchema(),
			"refresh": boolSchema("refresh active remote task statuses"),
		}, "run_dir"),
		tool("lovart_jobs_resume", "Resume an interrupted batch without resubmitting existing task IDs.", map[string]any{
			"run_dir":                 stringSchema("batch run directory"),
			"allow_paid":              boolSchema("allow paid batch generation"),
			"max_total_credits":       numberSchema("maximum total credits allowed when allow_paid=true"),
			"download_dir":            stringSchema("artifact download directory"),
			"retry_failed":            boolSchema("retry failed requests that were never submitted"),
			"submit_interval_seconds": numberSchema("seconds to wait between batch task submissions; defaults to 2"),
			"submit_limit":            numberSchema("maximum submit attempts in this MCP call; 0 means unlimited"),
			"max_active_tasks":        numberSchema("maximum active Lovart tasks allowed before deferring; defaults to 10, 0 disables"),
		}, "run_dir"),
		tool("lovart_jobs_finalize", "Download and/or write completed batch artifacts without resubmitting jobs.", map[string]any{
			"run_dir":       stringSchema("batch run directory"),
			"download":      boolSchema("download completed artifacts"),
			"canvas":        boolSchema("write completed artifacts to the project canvas"),
			"project_id":    stringSchema("optional project override"),
			"download_dir":  stringSchema("artifact download directory"),
			"detail":        detailSchema(),
			"canvas_layout": stringSchema("canvas layout: frame or plain; defaults to frame"),
		}, "run_dir"),
	}
}

// ToolNames returns stable tool names for status output.
func ToolNames() []string {
	tools := Tools()
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	return names
}
