package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const colorBlock = `
# Terminal-Farben (von bashq /colors gesetzt)
export TERM=xterm-256color
PS1='\[\e[1;32m\]\u@\h\[\e[0m\]:\[\e[1;34m\]\w\[\e[0m\]\$ '
alias ls='ls --color=auto'
alias ll='ls -lah --color=auto'
alias grep='grep --color=auto'
alias diff='diff --color=auto'
`

func setupColors() (msg string, isErr bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Sprintf("Homeverzeichnis nicht ermittelbar: %v", err), true
	}

	rcPath := filepath.Join(home, ".bashrc")

	existing, err := os.ReadFile(rcPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Sprintf("~/.bashrc nicht lesbar: %v", err), true
	}

	if strings.Contains(string(existing), "xterm-256color") {
		return fmt.Sprintf("✓ %s enthält bereits Farb-Einstellungen – nichts geändert.", rcPath), false
	}

	f, err := os.OpenFile(rcPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Sprintf("~/.bashrc nicht schreibbar: %v", err), true
	}
	defer f.Close()

	if _, err := f.WriteString(colorBlock); err != nil {
		return fmt.Sprintf("Schreiben fehlgeschlagen: %v", err), true
	}

	return fmt.Sprintf("✓ Farben in %s eingetragen.\n  Neues Terminal öffnen damit die Änderungen aktiv werden.", rcPath), false
}
