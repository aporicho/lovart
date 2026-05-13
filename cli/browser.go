package cli

import internalbrowser "github.com/aporicho/lovart/internal/browser"

type browserCommand struct {
	name string
	args []string
	wait bool
}

func openBrowser(url string) error {
	return internalbrowser.OpenURL(url)
}

func browserCommands(goos, url string) []browserCommand {
	commands := internalbrowser.Commands(goos, url)
	result := make([]browserCommand, 0, len(commands))
	for _, command := range commands {
		result = append(result, browserCommand{name: command.Name, args: command.Args, wait: command.Wait})
	}
	return result
}
