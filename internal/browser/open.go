// Package browser opens local browser URLs without tying callers to a CLI package.
package browser

import (
	"os/exec"
	"runtime"
)

// Command describes one OS-specific browser launch attempt.
type Command struct {
	Name string
	Args []string
	Wait bool
}

// OpenURL opens a URL in the user's browser.
func OpenURL(url string) error {
	var lastErr error
	for _, spec := range Commands(runtime.GOOS, url) {
		cmd := exec.Command(spec.Name, spec.Args...)
		var err error
		if spec.Wait {
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

// Commands returns browser launch commands in preference order.
func Commands(goos, url string) []Command {
	switch goos {
	case "darwin":
		return []Command{
			{Name: "open", Args: []string{"-a", "Google Chrome", url}, Wait: true},
			{Name: "open", Args: []string{url}},
		}
	case "windows":
		return []Command{
			{Name: "cmd", Args: []string{"/c", "start", "", "chrome", url}, Wait: true},
			{Name: "rundll32", Args: []string{"url.dll,FileProtocolHandler", url}},
		}
	default:
		return []Command{
			{Name: "google-chrome", Args: []string{url}},
			{Name: "google-chrome-stable", Args: []string{url}},
			{Name: "chromium-browser", Args: []string{url}},
			{Name: "chromium", Args: []string{url}},
			{Name: "xdg-open", Args: []string{url}},
		}
	}
}
