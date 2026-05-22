package mcp

import (
	"context"
	"testing"

	"github.com/aporicho/lovart/internal/envelope"
)

func TestPolicyGateAllowsRemoteRead(t *testing.T) {
	executor := &fakeExecutor{}
	server := NewServerWithExecutor(executor)
	env := server.CallTool(context.Background(), "lovart_project_show", map[string]any{"project_id": "proj_123"})
	if !env.OK {
		t.Fatalf("remote read should execute without policy confirmation: %#v", env)
	}
	if executor.projectShow.ProjectID != "proj_123" {
		t.Fatalf("executor was not called: %#v", executor.projectShow)
	}
}

func TestPolicyGateBlocksRemoteWriteWithoutConfirmation(t *testing.T) {
	executor := &fakeExecutor{}
	server := NewServerWithExecutor(executor)
	env := server.CallTool(context.Background(), "lovart_project_create", map[string]any{"name": "Draft"})
	assertPolicyConfirmationRequired(t, env, "lovart_project_create")
	if executor.projectCreate.Name != "" {
		t.Fatalf("executor should not be called: %#v", executor.projectCreate)
	}
}

func TestPolicyGateConfirmationEntersExecutor(t *testing.T) {
	executor := &fakeExecutor{}
	server := NewServerWithExecutor(executor)
	env := server.CallTool(context.Background(), "lovart_project_create", map[string]any{"name": "Draft", "confirm_policy": true})
	if !env.OK {
		t.Fatalf("confirmed remote write should execute: %#v", env)
	}
	if executor.projectCreate.Name != "Draft" {
		t.Fatalf("executor was not called with parsed args: %#v", executor.projectCreate)
	}
}

func TestPolicyGateBlocksDestructiveWithoutConfirmation(t *testing.T) {
	executor := &fakeExecutor{}
	server := NewServerWithExecutor(executor)
	env := server.CallTool(context.Background(), "lovart_task_cancel", map[string]any{"task_ids": []any{"task-1"}})
	assertPolicyConfirmationRequired(t, env, "lovart_task_cancel")
	if len(executor.taskCancel.TaskIDs) != 0 {
		t.Fatalf("executor should not be called: %#v", executor.taskCancel)
	}
}

func assertPolicyConfirmationRequired(t *testing.T, env envelope.Envelope, toolName string) {
	t.Helper()
	if env.OK || env.Error == nil || env.Error.Code != policyConfirmationRequiredCode {
		t.Fatalf("expected policy confirmation envelope, got %#v", env)
	}
	decision, ok := env.Data.(PolicyDecision)
	if !ok {
		t.Fatalf("expected PolicyDecision data, got %#v", env.Data)
	}
	if decision.Decision != "confirmation_required" || !decision.RequiredConfirmation || decision.ToolName != toolName {
		t.Fatalf("unexpected policy decision: %#v", decision)
	}
	if decision.Reason == "" || len(decision.RecommendedActions) == 0 {
		t.Fatalf("policy decision missing reason/actions: %#v", decision)
	}
}
