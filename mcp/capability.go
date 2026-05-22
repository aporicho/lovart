package mcp

// ToolCapability is machine-readable safety metadata exposed through tools/list.
type ToolCapability struct {
	ExecutionClass           string `json:"execution_class"`
	NetworkRequired          bool   `json:"network_required"`
	RemoteWrite              bool   `json:"remote_write"`
	PaidPossible             bool   `json:"paid_possible"`
	Destructive              bool   `json:"destructive"`
	LocalFilesystemWrite     bool   `json:"local_filesystem_write"`
	Idempotent               bool   `json:"idempotent"`
	RequiresUserConfirmation bool   `json:"requires_user_confirmation"`
}

func localReadCapability() ToolCapability {
	return ToolCapability{
		ExecutionClass: executionLocal,
		Idempotent:     true,
	}
}

func localWriteCapability() ToolCapability {
	capability := localReadCapability()
	capability.LocalFilesystemWrite = true
	capability.Idempotent = false
	return capability
}

func remoteReadCapability() ToolCapability {
	return ToolCapability{
		ExecutionClass:       executionPreflight,
		NetworkRequired:      true,
		RemoteWrite:          false,
		PaidPossible:         false,
		Destructive:          false,
		Idempotent:           true,
		LocalFilesystemWrite: false,
	}
}

func remoteWriteCapability() ToolCapability {
	return ToolCapability{
		ExecutionClass:  executionSubmit,
		NetworkRequired: true,
		RemoteWrite:     true,
		PaidPossible:    false,
		Destructive:     false,
		Idempotent:      false,
	}
}

func paidSubmitCapability() ToolCapability {
	capability := remoteWriteCapability()
	capability.PaidPossible = true
	return capability
}

func destructiveRemoteWriteCapability() ToolCapability {
	capability := remoteWriteCapability()
	capability.Destructive = true
	capability.RequiresUserConfirmation = true
	return capability
}
