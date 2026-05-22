package mcp

func parseGenerateArgs(args map[string]any) (GenerateArgs, error) {
	model, err := requiredString(args, "model")
	if err != nil {
		return GenerateArgs{}, err
	}
	body, err := requiredBody(args)
	if err != nil {
		return GenerateArgs{}, err
	}
	mode, err := modeArg(args, "mode", "auto")
	if err != nil {
		return GenerateArgs{}, err
	}
	wait := boolArg(args, "wait", false)
	download := boolArg(args, "download", false)
	canvas := boolArg(args, "canvas", false)
	return GenerateArgs{
		Model:                     model,
		Body:                      body,
		Mode:                      mode,
		AllowPaid:                 boolArg(args, "allow_paid", false),
		MaxCredits:                numberArg(args, "max_credits", 0),
		ConfirmPaid:               boolArg(args, "confirm_paid", false),
		ConfirmedMaxCredits:       numberArg(args, "confirmed_max_credits", 0),
		ConfirmedModel:            stringArg(args, "confirmed_model", ""),
		ConfirmedMode:             stringArg(args, "confirmed_mode", ""),
		ConfirmedOutputs:          intArg(args, "confirmed_outputs", 0),
		ConfirmedRemoteRequests:   intArg(args, "confirmed_remote_requests", 0),
		ProjectID:                 stringArg(args, "project_id", ""),
		ConfirmedProjectID:        stringArg(args, "confirmed_project_id", ""),
		Wait:                      wait,
		Download:                  download,
		Canvas:                    canvas,
		DownloadDir:               stringArg(args, "download_dir", ""),
		DownloadDirTemplate:       stringArg(args, "download_dir_template", ""),
		DownloadFileTemplate:      stringArg(args, "download_file_template", ""),
		ConfirmedCanonicalPath:    stringArg(args, "confirmed_canonical_path", ""),
		ConfirmOverwrite:          boolArg(args, "confirm_overwrite", false),
		ConfirmOutsideDefaultRoot: boolArg(args, "confirm_outside_default_root", false),
		ConfirmCanvasWrite:        boolArg(args, "confirm_canvas_write", false),
		ConfirmedCanvasLayout:     stringArg(args, "confirmed_canvas_layout", ""),
	}, nil
}

func parseJobsRunArgs(args map[string]any) (JobsRunArgs, error) {
	jobsFile, err := requiredString(args, "jobs_file")
	if err != nil {
		return JobsRunArgs{}, err
	}
	return JobsRunArgs{
		JobsFile:                  jobsFile,
		AllowPaid:                 boolArg(args, "allow_paid", false),
		MaxTotalCredits:           numberArg(args, "max_total_credits", 0),
		ConfirmPaid:               boolArg(args, "confirm_paid", false),
		ConfirmedMaxTotalCredits:  numberArg(args, "confirmed_max_total_credits", 0),
		ConfirmedRemoteRequests:   intArg(args, "confirmed_remote_requests", 0),
		ConfirmedMaxActiveTasks:   intArg(args, "confirmed_max_active_tasks", 0),
		ConfirmedModel:            stringArg(args, "confirmed_model", ""),
		ConfirmedMode:             stringArg(args, "confirmed_mode", ""),
		ConfirmedOutputs:          intArg(args, "confirmed_outputs", 0),
		ConfirmedMaxCredits:       numberArg(args, "confirmed_max_credits", 0),
		ProjectID:                 stringArg(args, "project_id", ""),
		ConfirmedProjectID:        stringArg(args, "confirmed_project_id", ""),
		DownloadDir:               stringArg(args, "download_dir", ""),
		ConfirmedCanonicalPath:    stringArg(args, "confirmed_canonical_path", ""),
		ConfirmOverwrite:          boolArg(args, "confirm_overwrite", false),
		ConfirmOutsideDefaultRoot: boolArg(args, "confirm_outside_default_root", false),
		SubmitIntervalSeconds:     numberArg(args, "submit_interval_seconds", 2),
		SubmitLimit:               intArg(args, "submit_limit", 0),
		MaxActiveTasks:            intArg(args, "max_active_tasks", 10),
	}, nil
}

func parseJobsResumeArgs(args map[string]any) (JobsResumeArgs, error) {
	runDir, err := requiredString(args, "run_dir")
	if err != nil {
		return JobsResumeArgs{}, err
	}
	return JobsResumeArgs{
		RunDir:                    runDir,
		AllowPaid:                 boolArg(args, "allow_paid", false),
		MaxTotalCredits:           numberArg(args, "max_total_credits", 0),
		ConfirmPaid:               boolArg(args, "confirm_paid", false),
		ConfirmedMaxTotalCredits:  numberArg(args, "confirmed_max_total_credits", 0),
		ConfirmedRemoteRequests:   intArg(args, "confirmed_remote_requests", 0),
		ConfirmedMaxActiveTasks:   intArg(args, "confirmed_max_active_tasks", 0),
		ConfirmedModel:            stringArg(args, "confirmed_model", ""),
		ConfirmedMode:             stringArg(args, "confirmed_mode", ""),
		ConfirmedOutputs:          intArg(args, "confirmed_outputs", 0),
		ConfirmedMaxCredits:       numberArg(args, "confirmed_max_credits", 0),
		ConfirmedProjectID:        stringArg(args, "confirmed_project_id", ""),
		DownloadDir:               stringArg(args, "download_dir", ""),
		ConfirmedCanonicalPath:    stringArg(args, "confirmed_canonical_path", ""),
		ConfirmOverwrite:          boolArg(args, "confirm_overwrite", false),
		ConfirmOutsideDefaultRoot: boolArg(args, "confirm_outside_default_root", false),
		RetryFailed:               boolArg(args, "retry_failed", false),
		SubmitIntervalSeconds:     numberArg(args, "submit_interval_seconds", 2),
		SubmitLimit:               intArg(args, "submit_limit", 0),
		MaxActiveTasks:            intArg(args, "max_active_tasks", 10),
	}, nil
}
