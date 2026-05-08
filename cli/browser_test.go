package cli

import (
	"reflect"
	"testing"
)

func TestBrowserCommandsPreferGoogleChrome(t *testing.T) {
	url := "https://www.lovart.ai/"

	tests := []struct {
		name string
		goos string
		want browserCommand
	}{
		{
			name: "macos",
			goos: "darwin",
			want: browserCommand{name: "open", args: []string{"-a", "Google Chrome", url}, wait: true},
		},
		{
			name: "windows",
			goos: "windows",
			want: browserCommand{name: "cmd", args: []string{"/c", "start", "", "chrome", url}, wait: true},
		},
		{
			name: "linux",
			goos: "linux",
			want: browserCommand{name: "google-chrome", args: []string{url}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := browserCommands(tt.goos, url)
			if len(got) == 0 {
				t.Fatal("browserCommands returned no commands")
			}
			if !reflect.DeepEqual(got[0], tt.want) {
				t.Fatalf("first command = %#v, want %#v", got[0], tt.want)
			}
		})
	}
}

func TestBrowserCommandsKeepDefaultFallback(t *testing.T) {
	url := "https://www.lovart.ai/"
	tests := []struct {
		name string
		goos string
		want browserCommand
	}{
		{name: "macos", goos: "darwin", want: browserCommand{name: "open", args: []string{url}}},
		{name: "windows", goos: "windows", want: browserCommand{name: "rundll32", args: []string{"url.dll,FileProtocolHandler", url}}},
		{name: "linux", goos: "linux", want: browserCommand{name: "xdg-open", args: []string{url}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := browserCommands(tt.goos, url)
			if len(got) == 0 {
				t.Fatal("browserCommands returned no commands")
			}
			last := got[len(got)-1]
			if !reflect.DeepEqual(last, tt.want) {
				t.Fatalf("last command = %#v, want %#v", last, tt.want)
			}
		})
	}
}
