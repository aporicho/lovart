package mcp

import (
	"fmt"

	"github.com/aporicho/lovart/internal/envelope"
)

const policyConfirmationRequiredCode = "policy_confirmation_required"

// PolicyDecision is returned when MCP call policy allows or blocks execution.
type PolicyDecision struct {
	Decision             string         `json:"decision"`
	ToolName             string         `json:"tool_name"`
	Capability           ToolCapability `json:"capability"`
	RequiredConfirmation bool           `json:"required_confirmation"`
	Reason               string         `json:"reason,omitempty"`
	RecommendedActions   []string       `json:"recommended_actions,omitempty"`
}

func gateToolCallPolicy(name string, args map[string]any) (PolicyDecision, bool) {
	capability := capabilityForTool(name)
	decision := PolicyDecision{
		Decision:   "allow",
		ToolName:   name,
		Capability: capability,
	}
	if !requiresPolicyConfirmation(capability) || hasPolicyConfirmation(name, args) {
		return decision, true
	}

	decision.Decision = "confirmation_required"
	decision.RequiredConfirmation = true
	decision.Reason = policyConfirmationReason(capability)
	decision.RecommendedActions = []string{
		"Only continue if the user explicitly approved this tool call and its side effects.",
		"Repeat the call with confirm_policy=true or confirm=true after approval.",
	}
	return decision, false
}

func policyGateEnvelope(decision PolicyDecision) envelope.Envelope {
	return envelope.Envelope{
		OK:   false,
		Data: decision,
		Error: &envelope.ErrorBody{
			Code:    policyConfirmationRequiredCode,
			Message: fmt.Sprintf("policy confirmation required for MCP tool %s", decision.ToolName),
			Details: map[string]any{
				"policy_decision":       decision.Decision,
				"tool_name":             decision.ToolName,
				"capability":            decision.Capability,
				"required_confirmation": decision.RequiredConfirmation,
				"reason":                decision.Reason,
				"recommended_actions":   decision.RecommendedActions,
			},
		},
	}
}

func requiresPolicyConfirmation(capability ToolCapability) bool {
	return capability.RemoteWrite || capability.PaidPossible || capability.Destructive || capability.LocalFilesystemWrite || capability.RequiresUserConfirmation
}

func hasPolicyConfirmation(name string, args map[string]any) bool {
	return boolArg(args, "confirm_policy", false) || boolArg(args, "confirm", false) || hasBusinessConfirmation(name, args)
}

func hasBusinessConfirmation(name string, args map[string]any) bool {
	switch name {
	case "lovart_project_delete":
		return stringArg(args, "confirm_project_id", "") != ""
	default:
		return false
	}
}

func policyConfirmationReason(capability ToolCapability) string {
	flags := []string{}
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
	return "tool capability requires explicit confirmation: " + stringsJoin(flags, ", ")
}

func stringsJoin(values []string, sep string) string {
	if len(values) == 0 {
		return "none"
	}
	result := values[0]
	for _, value := range values[1:] {
		result += sep + value
	}
	return result
}
