package mcp

import (
	"sort"

	"github.com/aporicho/lovart/internal/audit"
	"github.com/aporicho/lovart/internal/envelope"
)

func (s *Server) auditToolAttempt(name string, args map[string]any) {
	s.writeAudit(audit.Event{
		Operation: "mcp.tool_call",
		Source:    "mcp",
		Tool:      name,
		Action:    "start",
		Decision:  "attempt",
		RiskFlags: capabilityRiskFlags(capabilityForTool(name)),
		Metadata: map[string]any{
			"argument_keys": sortedMapKeys(args),
		},
	})
}

func (s *Server) auditPolicyDecision(decision PolicyDecision) {
	action := "policy_allow"
	if decision.Decision != "allow" {
		action = "policy_deny"
	}
	s.writeAudit(audit.Event{
		Operation: "mcp.policy_gate",
		Source:    "mcp",
		Tool:      decision.ToolName,
		Action:    action,
		Decision:  decision.Decision,
		RiskFlags: capabilityRiskFlags(decision.Capability),
		Metadata: map[string]any{
			"required_confirmation": decision.RequiredConfirmation,
			"recommended_actions":   decision.RecommendedActions,
		},
		Error: decision.Reason,
	})
}

func (s *Server) auditToolResult(name string, result envelope.Envelope) {
	decision := "success"
	metadata := map[string]any{"ok": result.OK}
	var errText string
	if !result.OK {
		decision = "error"
		if result.Error != nil {
			metadata["error_code"] = result.Error.Code
			errText = result.Error.Message
		}
	}
	s.writeAudit(audit.Event{
		Operation: "mcp.tool_call",
		Source:    "mcp",
		Tool:      name,
		Action:    "finish",
		Decision:  decision,
		RiskFlags: capabilityRiskFlags(capabilityForTool(name)),
		Metadata:  metadata,
		Error:     errText,
	})
}

func (s *Server) writeAudit(event audit.Event) {
	if s == nil || s.auditLogger == nil {
		return
	}
	_ = s.auditLogger.Write(event)
}

func capabilityRiskFlags(capability ToolCapability) []string {
	flags := []string{}
	if capability.NetworkRequired {
		flags = append(flags, "network_required")
	}
	if capability.RemoteWrite {
		flags = append(flags, "remote_write")
	}
	if capability.PaidPossible {
		flags = append(flags, "paid_possible")
	}
	if capability.Destructive {
		flags = append(flags, "destructive")
	}
	if capability.LocalFilesystemWrite {
		flags = append(flags, "local_filesystem_write")
	}
	if capability.RequiresUserConfirmation {
		flags = append(flags, "requires_user_confirmation")
	}
	return flags
}

func sortedMapKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
