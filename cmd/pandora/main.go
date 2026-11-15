// Command pandora is a terminal vault for secrets backed by the OS keyring.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"pandora/internal/cli"
	"pandora/internal/tui"
)

func main() {
	if len(os.Args) > 1 {
		os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
	}
	if _, err := tea.NewProgram(tui.New(), tea.WithAltScreen(), tea.WithMouseCellMotion()).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
