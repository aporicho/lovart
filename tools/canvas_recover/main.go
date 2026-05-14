package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/aporicho/lovart/internal/auth"
	lovarthttp "github.com/aporicho/lovart/internal/http"
	"github.com/aporicho/lovart/internal/paths"
	"github.com/aporicho/lovart/internal/project"
	"github.com/aporicho/lovart/internal/signing"
)

type stringList []string

func (s *stringList) String() string {
	data, _ := json.Marshal([]string(*s))
	return string(data)
}

func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func main() {
	var artifactURLs stringList
	projectID := flag.String("project-id", "", "Lovart project id")
	taskID := flag.String("task-id", "", "generator task id to remove from canvas")
	apply := flag.Bool("apply", false, "write recovered canvas back to Lovart")
	flag.Var(&artifactURLs, "artifact-url", "artifact URL to match; repeatable")
	flag.Parse()

	if err := paths.PrepareRuntime(); err != nil {
		fail("prepare runtime", err)
	}
	creds, err := auth.Load()
	if err != nil {
		fail("load credentials", err)
	}
	pc, _ := auth.LoadProjectContext()
	cid := creds.WebID
	if pc != nil {
		if *projectID == "" {
			*projectID = pc.ProjectID
		}
		if pc.CID != "" {
			cid = pc.CID
		}
	}
	if *projectID == "" {
		fail("project id", fmt.Errorf("missing --project-id and no selected project"))
	}
	signer, err := signing.NewSigner()
	if err != nil {
		fail("load signer", err)
	}
	client := lovarthttp.NewClient(creds, signer)
	if err := client.SyncTime(context.Background()); err != nil {
		fail("sync time", err)
	}

	result, err := project.RecoverCanvasByTask(context.Background(), client, *projectID, cid, project.CanvasRecoverOptions{
		TaskID:       *taskID,
		ArtifactURLs: []string(artifactURLs),
		Apply:        *apply,
	})
	if err != nil {
		fail("recover canvas", err)
	}
	data, err := json.MarshalIndent(map[string]any{
		"ok":   true,
		"data": result,
	}, "", "  ")
	if err != nil {
		fail("marshal result", err)
	}
	fmt.Println(string(data))
}

func fail(message string, err error) {
	data, _ := json.MarshalIndent(map[string]any{
		"ok": false,
		"error": map[string]any{
			"message": message,
			"details": err.Error(),
		},
	}, "", "  ")
	fmt.Fprintln(os.Stderr, string(data))
	os.Exit(1)
}
