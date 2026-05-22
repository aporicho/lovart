package mcp

var toolCapabilityRegistry = map[string]ToolCapability{
	"lovart_auth_status":       localReadCapability(),
	"lovart_auth_login":        remoteWriteCapability(),
	"lovart_extension_status":  localReadCapability(),
	"lovart_extension_install": localWriteCapability(),
	"lovart_extension_open":    localReadCapability(),
	"lovart_setup":             localReadCapability(),
	"lovart_models":            remoteReadCapability(),
	"lovart_config":            localReadCapability(),
	"lovart_balance":           remoteReadCapability(),

	"lovart_project_current": localReadCapability(),
	"lovart_project_list":    remoteReadCapability(),
	"lovart_project_create":  remoteWriteCapability(),
	"lovart_project_select":  localWriteCapability(),
	"lovart_project_show":    remoteReadCapability(),
	"lovart_project_open":    localReadCapability(),
	"lovart_project_rename":  remoteWriteCapability(),
	"lovart_project_delete":  destructiveRemoteWriteCapability(),

	"lovart_task_list":     remoteReadCapability(),
	"lovart_task_cancel":   destructiveRemoteWriteCapability(),
	"lovart_task_status":   remoteReadCapability(),
	"lovart_task_wait":     remoteReadCapability(),
	"lovart_task_canvas":   remoteWriteCapability(),
	"lovart_task_download": localDownloadCapability(),

	"lovart_canvas_artifacts": remoteReadCapability(),
	"lovart_canvas_artifact":  remoteReadCapability(),
	"lovart_canvas_download":  localDownloadCapability(),

	"lovart_quote":         remoteReadCapability(),
	"lovart_generate":      generateCapability(),
	"lovart_jobs_run":      jobsRunCapability(),
	"lovart_jobs_status":   jobsStatusCapability(),
	"lovart_jobs_resume":   jobsRunCapability(),
	"lovart_jobs_finalize": jobsFinalizeCapability(),
}

func capabilityForTool(name string) ToolCapability {
	capability, ok := toolCapabilityRegistry[name]
	if !ok {
		return localReadCapability()
	}
	return capability
}

func localDownloadCapability() ToolCapability {
	capability := remoteReadCapability()
	capability.LocalFilesystemWrite = true
	capability.Idempotent = false
	return capability
}

func generateCapability() ToolCapability {
	capability := paidSubmitCapability()
	capability.LocalFilesystemWrite = true
	return capability
}

func jobsRunCapability() ToolCapability {
	capability := paidSubmitCapability()
	capability.LocalFilesystemWrite = true
	return capability
}

func jobsStatusCapability() ToolCapability {
	capability := remoteReadCapability()
	capability.LocalFilesystemWrite = true
	return capability
}

func jobsFinalizeCapability() ToolCapability {
	capability := remoteWriteCapability()
	capability.LocalFilesystemWrite = true
	return capability
}
