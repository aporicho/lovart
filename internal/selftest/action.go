package selftest

// Action describes the single highest-value local recovery step for a failed
// readiness check. It is intentionally small and machine-readable so agents can
// choose between CLI commands and MCP tools without parsing prose.
type Action struct {
	Command     string `json:"command"`
	MCPTool     string `json:"mcp_tool"`
	Reason      string `json:"reason"`
	Blocking    bool   `json:"blocking"`
	Recoverable bool   `json:"recoverable"`
}

func authLoginAction(reason string) *Action {
	return &Action{
		Command:     "lovart auth login",
		MCPTool:     "lovart_auth_login",
		Reason:      reason,
		Blocking:    true,
		Recoverable: true,
	}
}

func credentialsPermissionsAction(reason string) *Action {
	return &Action{
		Command:     "lovart auth login",
		MCPTool:     "lovart_auth_login",
		Reason:      reason,
		Blocking:    true,
		Recoverable: true,
	}
}

func projectSelectionAction(reason string) *Action {
	return &Action{
		Command:     "lovart project list",
		MCPTool:     "lovart_project_list",
		Reason:      reason,
		Blocking:    true,
		Recoverable: true,
	}
}

func runtimeSyncAction(reason string) *Action {
	return &Action{
		Command:     "lovart update sync --all",
		Reason:      reason,
		Blocking:    true,
		Recoverable: true,
	}
}

func firstNextAction(checks []Check) *Action {
	for _, check := range checks {
		if check.NextAction != nil {
			return check.NextAction
		}
	}
	return nil
}
