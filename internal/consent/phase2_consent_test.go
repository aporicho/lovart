package consent

import "testing"

func TestValidateDestructiveConsentProjectID(t *testing.T) {
	scope := DestructiveScope{ProjectID: "proj_123"}

	assertDenied(t, ValidateDestructiveConsent(DestructiveConsent{}, scope), CodeStructuredConsentProjectIDMissing)
	assertDenied(t, ValidateDestructiveConsent(DestructiveConsent{ConfirmProjectID: "proj_other"}, scope), CodeStructuredConsentProjectIDMismatch)
	assertAllowed(t, ValidateDestructiveConsent(DestructiveConsent{ConfirmProjectID: "proj_123"}, scope))
}

func TestValidateDestructiveConsentTaskIDs(t *testing.T) {
	scope := DestructiveScope{TaskIDs: []string{"task_b", "task_a"}}

	assertDenied(t, ValidateDestructiveConsent(DestructiveConsent{}, scope), CodeStructuredConsentTaskIDsMissing)
	assertDenied(t, ValidateDestructiveConsent(DestructiveConsent{ConfirmTaskIDs: []string{"task_a"}}, scope), CodeStructuredConsentTaskIDsMismatch)
	assertAllowed(t, ValidateDestructiveConsent(DestructiveConsent{ConfirmTaskIDs: []string{"task_a", "task_b"}}, scope))
}

func TestValidateDestructiveConsentNewName(t *testing.T) {
	scope := DestructiveScope{ProjectID: "proj_123", NewName: "Final Name"}

	assertDenied(t, ValidateDestructiveConsent(DestructiveConsent{ConfirmProjectID: "proj_123"}, scope), CodeStructuredConsentNewNameMissing)
	assertDenied(t, ValidateDestructiveConsent(DestructiveConsent{ConfirmProjectID: "proj_123", ConfirmNewName: "Draft Name"}, scope), CodeStructuredConsentNewNameMismatch)
	assertAllowed(t, ValidateDestructiveConsent(DestructiveConsent{ConfirmProjectID: "proj_123", ConfirmNewName: "Final Name"}, scope))
}

func TestValidateRemoteWriteConsentExplicitProject(t *testing.T) {
	scope := RemoteWriteScope{ProjectID: "proj_123"}

	assertDenied(t, ValidateRemoteWriteConsent(RemoteWriteConsent{ConfirmCurrentProject: true}, scope), CodeStructuredConsentProjectIDMissing)
	assertDenied(t, ValidateRemoteWriteConsent(RemoteWriteConsent{ConfirmedProjectID: "proj_other"}, scope), CodeStructuredConsentProjectIDMismatch)
	assertAllowed(t, ValidateRemoteWriteConsent(RemoteWriteConsent{ConfirmedProjectID: "proj_123"}, scope))
}

func TestValidateRemoteWriteConsentCurrentProject(t *testing.T) {
	scope := RemoteWriteScope{CurrentProjectID: "proj_current"}

	assertDenied(t, ValidateRemoteWriteConsent(RemoteWriteConsent{}, scope), CodeStructuredConsentCurrentProjectMissing)
	assertDenied(t, ValidateRemoteWriteConsent(RemoteWriteConsent{ConfirmedProjectID: "proj_other"}, scope), CodeStructuredConsentProjectIDMismatch)
	assertAllowed(t, ValidateRemoteWriteConsent(RemoteWriteConsent{ConfirmCurrentProject: true}, scope))
	assertAllowed(t, ValidateRemoteWriteConsent(RemoteWriteConsent{ConfirmedProjectID: "proj_current"}, scope))
}

func TestValidateRemoteWriteConsentProjectCreateAndCanvasWrite(t *testing.T) {
	selectAfterCreate := true
	scope := RemoteWriteScope{ProjectName: "Project A", SelectAfterCreate: &selectAfterCreate, CanvasWrite: true}

	assertDenied(t, ValidateRemoteWriteConsent(RemoteWriteConsent{}, scope), CodeStructuredConsentProjectNameMissing)
	assertDenied(t, ValidateRemoteWriteConsent(RemoteWriteConsent{ConfirmedProjectName: "Other"}, scope), CodeStructuredConsentProjectNameMismatch)
	assertDenied(t, ValidateRemoteWriteConsent(RemoteWriteConsent{ConfirmedProjectName: "Project A"}, scope), CodeStructuredConsentSelectAfterCreateMissing)

	confirmedSelect := false
	assertDenied(t, ValidateRemoteWriteConsent(RemoteWriteConsent{ConfirmedProjectName: "Project A", ConfirmedSelectAfterCreate: &confirmedSelect}, scope), CodeStructuredConsentSelectAfterCreateMismatch)

	confirmedSelect = true
	assertDenied(t, ValidateRemoteWriteConsent(RemoteWriteConsent{ConfirmedProjectName: "Project A", ConfirmedSelectAfterCreate: &confirmedSelect}, scope), CodeStructuredConsentCanvasWriteMissing)
	assertAllowed(t, ValidateRemoteWriteConsent(RemoteWriteConsent{ConfirmedProjectName: "Project A", ConfirmedSelectAfterCreate: &confirmedSelect, ConfirmedCanvasWrite: true}, scope))
}

func TestValidateLocalWriteConsentCanonicalPath(t *testing.T) {
	scope := LocalWriteScope{CanonicalPath: "/tmp/lovart/run"}

	assertDenied(t, ValidateLocalWriteConsent(LocalWriteConsent{}, scope), CodeStructuredConsentCanonicalPathMissing)
	assertDenied(t, ValidateLocalWriteConsent(LocalWriteConsent{ConfirmedCanonicalPath: "/tmp/lovart/other"}, scope), CodeStructuredConsentCanonicalPathMismatch)
	assertAllowed(t, ValidateLocalWriteConsent(LocalWriteConsent{ConfirmedCanonicalPath: "/tmp/lovart/run"}, scope))
}

func TestValidateLocalWriteConsentOverwriteOutsideRootAndFileCount(t *testing.T) {
	scope := LocalWriteScope{CanonicalPath: "/outside/run", Overwrite: true, OutsideDefaultRoot: true, RequireFileCount: true, FileCount: 3}

	assertDenied(t, ValidateLocalWriteConsent(LocalWriteConsent{ConfirmedCanonicalPath: "/outside/run"}, scope), CodeStructuredConsentOverwriteMissing)
	assertDenied(t, ValidateLocalWriteConsent(LocalWriteConsent{ConfirmedCanonicalPath: "/outside/run", ConfirmOverwrite: true}, scope), CodeStructuredConsentOutsideRootMissing)
	assertDenied(t, ValidateLocalWriteConsent(LocalWriteConsent{ConfirmedCanonicalPath: "/outside/run", ConfirmOverwrite: true, ConfirmOutsideDefaultRoot: true}, scope), CodeStructuredConsentFileCountMissing)
	assertDenied(t, ValidateLocalWriteConsent(LocalWriteConsent{ConfirmedCanonicalPath: "/outside/run", ConfirmOverwrite: true, ConfirmOutsideDefaultRoot: true, ConfirmedFileCount: 2}, scope), CodeStructuredConsentFileCountMismatch)
	assertAllowed(t, ValidateLocalWriteConsent(LocalWriteConsent{ConfirmedCanonicalPath: "/outside/run", ConfirmOverwrite: true, ConfirmOutsideDefaultRoot: true, ConfirmedFileCount: 3}, scope))
}

func TestValidateCanvasConsentProjectAndWrite(t *testing.T) {
	scope := CanvasScope{ProjectID: "proj_123", CanvasWrite: true}

	assertDenied(t, ValidateCanvasConsent(CanvasConsent{}, scope), CodeStructuredConsentProjectIDMissing)
	assertDenied(t, ValidateCanvasConsent(CanvasConsent{ConfirmedProjectID: "proj_other"}, scope), CodeStructuredConsentProjectIDMismatch)
	assertDenied(t, ValidateCanvasConsent(CanvasConsent{ConfirmedProjectID: "proj_123"}, scope), CodeStructuredConsentCanvasWriteMissing)
	assertAllowed(t, ValidateCanvasConsent(CanvasConsent{ConfirmedProjectID: "proj_123", ConfirmCanvasWrite: true}, scope))
}

func TestValidateCanvasConsentShapeIDsAndLayout(t *testing.T) {
	scope := CanvasScope{CurrentProjectID: "proj_current", CanvasShapeIDs: []string{"shape_b", "shape_a"}, CanvasLayout: "grid-v1"}

	assertDenied(t, ValidateCanvasConsent(CanvasConsent{ConfirmCurrentProject: true}, scope), CodeStructuredConsentCanvasShapeIDsMissing)
	assertDenied(t, ValidateCanvasConsent(CanvasConsent{ConfirmCurrentProject: true, ConfirmCanvasShapeIDs: []string{"shape_a"}}, scope), CodeStructuredConsentCanvasShapeIDsMismatch)
	assertDenied(t, ValidateCanvasConsent(CanvasConsent{ConfirmCurrentProject: true, ConfirmCanvasShapeIDs: []string{"shape_a", "shape_b"}}, scope), CodeStructuredConsentCanvasLayoutMissing)
	assertDenied(t, ValidateCanvasConsent(CanvasConsent{ConfirmCurrentProject: true, ConfirmCanvasShapeIDs: []string{"shape_a", "shape_b"}, ConfirmedCanvasLayout: "freeform"}, scope), CodeStructuredConsentCanvasLayoutMismatch)
	assertAllowed(t, ValidateCanvasConsent(CanvasConsent{ConfirmCurrentProject: true, ConfirmCanvasShapeIDs: []string{"shape_a", "shape_b"}, ConfirmedCanvasLayout: "grid-v1"}, scope))
}
