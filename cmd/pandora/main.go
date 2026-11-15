// Command pandora is a terminal vault for secrets backed by the OS keyring.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"pandora/internal/tui"
)

func main() {
	if _, err := tea.NewProgram(tui.New(), tea.WithAltScreen(), tea.WithMouseCellMotion()).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
