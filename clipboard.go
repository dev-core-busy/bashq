package main

import (
	"os/exec"
	"strings"
)

// clipboardTools listet die Kommandos zum Auslesen der Zwischenablage,
// in der Reihenfolge in der sie probiert werden.
var clipboardTools = [][]string{
	{"wl-paste", "--no-newline"},               // Wayland
	{"xclip", "-selection", "clipboard", "-o"}, // X11
	{"xsel", "--clipboard", "--output"},        // X11 (Alternative)
}

// readClipboard liest den System-Zwischenspeicher aus.
// Gibt "" zurück wenn kein Werkzeug verfügbar ist oder die Ablage leer ist.
func readClipboard() string {
	for _, tool := range clipboardTools {
		if _, err := exec.LookPath(tool[0]); err != nil {
			continue
		}
		out, err := exec.Command(tool[0], tool[1:]...).Output()
		if err != nil {
			continue
		}
		if text := string(out); text != "" {
			return text
		}
	}
	return ""
}

// flattenPaste macht aus mehrzeiligem Text eine Zeile –
// das Eingabefeld ist einzeilig.
func flattenPaste(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	fields := strings.Fields(strings.ReplaceAll(text, "\n", " "))
	return strings.Join(fields, " ")
}
