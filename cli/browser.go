package cli

import (
	"os/exec"
	"runtime"
)

type browserCommand struct {
	name string
	args []string
	wait bool
}

func openBrowser(url string) error {
	var lastErr error
	for _, spec := range browserCommands(runtime.GOOS, url) {
		cmd := exec.Command(spec.name, spec.args...)
		var err error
		if spec.wait {
			err = cmd.Run()
		} else {
			err = cmd.Start()
		}
		if err == nil {
			return nil
		}
		lastErr = err
	}
	return lastErr
}

func browserCommands(goos, url string) []browserCommand {
	switch goos {
	case "darwin":
		return []browserCommand{
			{name: "open", args: []string{"-a", "Google Chrome", url}, wait: true},
			{name: "open", args: []string{url}},
		}
	case "windows":
		return []browserCommand{
			{name: "cmd", args: []string{"/c", "start", "", "chrome", url}, wait: true},
			{name: "rundll32", args: []string{"url.dll,FileProtocolHandler", url}},
		}
	default:
		return []browserCommand{
			{name: "google-chrome", args: []string{url}},
			{name: "google-chrome-stable", args: []string{url}},
			{name: "chromium-browser", args: []string{url}},
			{name: "chromium", args: []string{url}},
			{name: "xdg-open", args: []string{url}},
		}
	}
}
