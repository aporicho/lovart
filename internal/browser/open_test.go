package browser

import (
	"errors"
	"reflect"
	"testing"
)

func TestCommandsForNormalLinuxUseLinuxBrowserCommands(t *testing.T) {
	url := "https://www.lovart.ai/"
	got := commandsForEnvironment(environment{goos: "linux", lookPath: missingLookPath}, url)
	want := Commands("linux", url)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("commands = %#v, want %#v", got, want)
	}
}

func TestCommandsForWSLPreferDiscoveredWindowsOpeners(t *testing.T) {
	url := "https://www.lovart.ai/"
	env := environment{
		goos:          "linux",
		osRelease:     "6.6.87.2-microsoft-standard-WSL2",
		wslDistroName: "Ubuntu",
		lookPath: func(name string) (string, error) {
			switch name {
			case "wslview":
				return "/usr/bin/wslview", nil
			case "cmd.exe":
				return "/windows/cmd.exe", nil
			default:
				return "", errors.New("missing")
			}
		},
	}
	got := commandsForEnvironment(env, url)
	want := []Command{
		{Name: "/usr/bin/wslview", Args: []string{url}},
		{Name: "/windows/cmd.exe", Args: []string{"/c", "start", "", "chrome", url}, Wait: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("commands = %#v, want %#v", got, want)
	}
}

func TestCommandsForWSLDoNotHardcodeWindowsPaths(t *testing.T) {
	got := commandsForEnvironment(environment{
		goos:      "linux",
		osRelease: "microsoft-standard-WSL2",
		lookPath:  missingLookPath,
	}, "https://www.lovart.ai/")
	if len(got) != 0 {
		t.Fatalf("expected no commands without discovered interop tools, got %#v", got)
	}
}

func TestIsWSLEnvironmentRequiresLinux(t *testing.T) {
	if isWSLEnvironment(environment{goos: "darwin", osRelease: "microsoft"}) {
		t.Fatal("non-linux platform should not be WSL")
	}
	if !isWSLEnvironment(environment{goos: "linux", procVersion: "Linux version microsoft-standard-WSL2"}) {
		t.Fatal("expected WSL detection from proc version")
	}
}

func missingLookPath(name string) (string, error) {
	return "", errors.New("missing")
}
