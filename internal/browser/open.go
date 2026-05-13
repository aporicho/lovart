// Package browser opens local browser URLs without tying callers to a CLI package.
package browser

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Command describes one OS-specific browser launch attempt.
type Command struct {
	Name string
	Args []string
	Wait bool
}

type environment struct {
	goos          string
	osRelease     string
	procVersion   string
	wslDistroName string
	lookPath      func(string) (string, error)
	glob          func(string) ([]string, error)
}

// OpenURL opens a URL in the user's browser.
func OpenURL(url string) error {
	var lastErr error
	commands := currentCommands(url)
	if len(commands) == 0 && IsWSL() {
		if isChromeInternalURL(url) {
			return fmt.Errorf("no explicit Chrome opener found in WSL; open Chrome manually and enter: %s", url)
		}
		return fmt.Errorf("no Windows browser opener found in WSL; install wslu for wslview or open manually: %s", url)
	}
	for _, spec := range commands {
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

// IsWSL reports whether the current process appears to run under Windows
// Subsystem for Linux.
func IsWSL() bool {
	return isWSLEnvironment(currentEnvironment())
}

// Commands returns browser launch commands in preference order.
func Commands(goos, url string) []Command {
	chromeInternal := isChromeInternalURL(url)
	switch goos {
	case "darwin":
		commands := []Command{
			{Name: "open", Args: []string{"-a", "Google Chrome", url}, Wait: true},
		}
		if !chromeInternal {
			commands = append(commands, Command{Name: "open", Args: []string{url}})
		}
		return commands
	case "windows":
		commands := []Command{
			{Name: "cmd", Args: []string{"/c", "start", "", "chrome", url}, Wait: true},
		}
		if !chromeInternal {
			commands = append(commands, Command{Name: "rundll32", Args: []string{"url.dll,FileProtocolHandler", url}})
		}
		return commands
	default:
		commands := []Command{
			{Name: "google-chrome", Args: []string{url}},
			{Name: "google-chrome-stable", Args: []string{url}},
			{Name: "chromium-browser", Args: []string{url}},
			{Name: "chromium", Args: []string{url}},
		}
		if !chromeInternal {
			commands = append(commands, Command{Name: "xdg-open", Args: []string{url}})
		}
		return commands
	}
}

func currentCommands(url string) []Command {
	return commandsForEnvironment(currentEnvironment(), url)
}

func currentEnvironment() environment {
	env := environment{
		goos:          runtime.GOOS,
		wslDistroName: os.Getenv("WSL_DISTRO_NAME"),
		lookPath:      exec.LookPath,
		glob:          filepath.Glob,
	}
	if data, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil {
		env.osRelease = string(data)
	}
	if data, err := os.ReadFile("/proc/version"); err == nil {
		env.procVersion = string(data)
	}
	return env
}

func commandsForEnvironment(env environment, url string) []Command {
	if env.goos == "linux" && isWSLEnvironment(env) {
		return wslCommands(env, url)
	}
	return Commands(env.goos, url)
}

func isWSLEnvironment(env environment) bool {
	if env.goos != "linux" {
		return false
	}
	if env.wslDistroName != "" {
		return true
	}
	text := strings.ToLower(env.osRelease + "\n" + env.procVersion)
	return strings.Contains(text, "microsoft") || strings.Contains(text, "wsl")
}

func wslCommands(env environment, url string) []Command {
	lookPath := env.lookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	var commands []Command
	chromeInternal := isChromeInternalURL(url)
	if chromeInternal {
		for _, path := range windowsChromePaths(env) {
			commands = append(commands, Command{Name: path, Args: []string{url}})
		}
	}
	if !chromeInternal {
		if path, err := lookPath("wslview"); err == nil {
			commands = append(commands, Command{Name: path, Args: []string{url}})
		}
	}
	if path, err := lookPath("cmd.exe"); err == nil {
		commands = append(commands, Command{Name: path, Args: []string{"/c", "start", "", "chrome", url}, Wait: true})
	}
	if path, err := lookPath("powershell.exe"); err == nil {
		if chromeInternal {
			commands = append(commands, Command{Name: path, Args: []string{"-NoProfile", "-Command", "Start-Process -FilePath chrome -ArgumentList $args[0]", url}})
		} else {
			commands = append(commands, Command{Name: path, Args: []string{"-NoProfile", "-Command", "Start-Process -FilePath $args[0]", url}})
		}
	}
	if chromeInternal {
		return commands
	}
	if path, err := lookPath("explorer.exe"); err == nil {
		commands = append(commands, Command{Name: path, Args: []string{url}})
	}
	return commands
}

func isChromeInternalURL(url string) bool {
	return strings.HasPrefix(strings.ToLower(url), "chrome://")
}

func windowsChromePaths(env environment) []string {
	glob := env.glob
	if glob == nil {
		glob = filepath.Glob
	}
	patterns := []string{
		"/mnt/c/Program Files/Google/Chrome/Application/chrome.exe",
		"/mnt/c/Program Files (x86)/Google/Chrome/Application/chrome.exe",
		"/mnt/c/Users/*/AppData/Local/Google/Chrome/Application/chrome.exe",
	}
	var paths []string
	seen := map[string]bool{}
	for _, pattern := range patterns {
		matches, err := glob(pattern)
		if err != nil {
			continue
		}
		for _, match := range matches {
			if !seen[match] {
				seen[match] = true
				paths = append(paths, match)
			}
		}
	}
	return paths
}
