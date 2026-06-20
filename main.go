package main

import (
	"fmt"
	"os"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
)

// restartAfterUpdate wird gesetzt wenn auto-update eine neue Binary installiert hat.
// main() startet den Prozess dann per syscall.Exec neu.
var restartAfterUpdate bool

func main() {
	p := tea.NewProgram(
		newModel(),
		tea.WithAltScreen(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "Fehler:", err)
		os.Exit(1)
	}

	if restartAfterUpdate {
		exe, err := os.Executable()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Neustart fehlgeschlagen:", err)
			os.Exit(1)
		}
		syscall.Exec(exe, os.Args, os.Environ())
	}
}
