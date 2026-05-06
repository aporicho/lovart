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

func detailSchema() map[string]any {
	return map[string]any{"type": "string", "enum": []string{"summary", "requests", "full"}}
}

func modeSchema() map[string]any {
	return map[string]any{"type": "string", "enum": []string{"auto", "fast", "relax"}}
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
		tool("lovart_project_select", "Select the Lovart project used by generation tools.", map[string]any{
			"project_id": stringSchema("Lovart project id"),
		}, "project_id"),
		tool("lovart_quote", "Fetch an exact Lovart credit quote for a model request.", map[string]any{
			"model": stringSchema("Lovart generator model name"),
			"body":  map[string]any{"type": "object"},
		}, "model", "body"),
		tool("lovart_generate_dry_run", "Run online generation preflight without submitting.", map[string]any{
			"model":       stringSchema("Lovart generator model name"),
			"body":        map[string]any{"type": "object"},
			"mode":        modeSchema(),
			"allow_paid":  boolSchema("allow paid generation during gate evaluation"),
			"max_credits": numberSchema("maximum credits allowed when allow_paid=true"),
			"project_id":  stringSchema("optional project override"),
			"cid":         stringSchema("optional client id override"),
		}, "model", "body"),
		tool("lovart_generate", "Submit a single generation request after the normal paid/zero-credit gate.", map[string]any{
			"model":                  stringSchema("Lovart generator model name"),
			"body":                   map[string]any{"type": "object"},
			"mode":                   modeSchema(),
			"allow_paid":             boolSchema("allow paid generation"),
			"max_credits":            numberSchema("maximum credits allowed when allow_paid=true"),
			"project_id":             stringSchema("optional project override"),
			"cid":                    stringSchema("optional client id override"),
			"wait":                   boolSchema("wait for completion"),
			"download":               boolSchema("download artifacts after completion"),
			"canvas":                 boolSchema("write completed artifacts back to project canvas"),
			"download_dir":           stringSchema("artifact download directory"),
			"download_dir_template":  stringSchema("download subdirectory template"),
			"download_file_template": stringSchema("download filename template"),
		}, "model", "body"),
		tool("lovart_jobs_quote", "Quote a user-level batch jobs JSONL file.", map[string]any{
			"jobs_file": stringSchema("path to jobs.jsonl"),
		}, "jobs_file"),
		tool("lovart_jobs_dry_run", "Run whole-batch preflight without submitting.", map[string]any{
			"jobs_file":         stringSchema("path to jobs.jsonl"),
			"out_dir":           stringSchema("batch state output directory"),
			"allow_paid":        boolSchema("allow paid batch generation during gate evaluation"),
			"max_total_credits": numberSchema("maximum total credits allowed when allow_paid=true"),
			"detail":            detailSchema(),
		}, "jobs_file"),
		tool("lovart_jobs_run", "Submit all pending batch remote requests after the whole-batch gate passes.", map[string]any{
			"jobs_file":              stringSchema("path to jobs.jsonl"),
			"out_dir":                stringSchema("batch state output directory"),
			"allow_paid":             boolSchema("allow paid batch generation"),
			"max_total_credits":      numberSchema("maximum total credits allowed when allow_paid=true"),
			"wait":                   boolSchema("wait for submitted tasks"),
			"download":               boolSchema("download artifacts after completion"),
			"canvas":                 boolSchema("write completed artifacts back to project canvas"),
			"canvas_layout":          map[string]any{"type": "string", "enum": []string{"frame", "plain"}},
			"download_dir":           stringSchema("artifact download directory"),
			"download_dir_template":  stringSchema("download subdirectory template"),
			"download_file_template": stringSchema("download filename template"),
			"timeout_seconds":        numberSchema("local wait timeout; capped at 90 seconds through MCP"),
			"poll_interval":          numberSchema("task polling interval in seconds"),
			"project_id":             stringSchema("optional project override"),
			"cid":                    stringSchema("optional client id override"),
			"detail":                 detailSchema(),
		}, "jobs_file"),
		tool("lovart_jobs_status", "Read local batch run state.", map[string]any{
			"run_dir": stringSchema("batch run directory"),
			"detail":  detailSchema(),
			"refresh": boolSchema("refresh active remote task statuses"),
		}, "run_dir"),
		tool("lovart_jobs_resume", "Resume an interrupted batch without resubmitting existing task IDs.", map[string]any{
			"run_dir":                stringSchema("batch run directory"),
			"allow_paid":             boolSchema("allow paid batch generation"),
			"max_total_credits":      numberSchema("maximum total credits allowed when allow_paid=true"),
			"wait":                   boolSchema("wait for submitted tasks"),
			"download":               boolSchema("download artifacts after completion"),
			"canvas":                 boolSchema("write completed artifacts back to project canvas"),
			"canvas_layout":          map[string]any{"type": "string", "enum": []string{"frame", "plain"}},
			"download_dir":           stringSchema("artifact download directory"),
			"download_dir_template":  stringSchema("download subdirectory template"),
			"download_file_template": stringSchema("download filename template"),
			"retry_failed":           boolSchema("retry failed requests that were never submitted"),
			"timeout_seconds":        numberSchema("local wait timeout; capped at 90 seconds through MCP"),
			"poll_interval":          numberSchema("task polling interval in seconds"),
			"project_id":             stringSchema("optional project override"),
			"cid":                    stringSchema("optional client id override"),
			"detail":                 detailSchema(),
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
