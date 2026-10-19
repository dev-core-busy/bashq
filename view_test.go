package main

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

var ansiRE = regexp.MustCompile("\x1b\\[[0-9;]*m")

func stripANSI(s string) string { return ansiRE.ReplaceAllString(s, "") }

func TestTruncToWidth(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		width int
		want  string
	}{
		{"passt unverändert", "llama3", 10, "llama3"},
		{"genau passend", "llama3", 6, "llama3"},
		{"ASCII gekürzt", "llama3-instruct", 8, "llama3-…"},
		{"Umlaute zählen einfach", "Büro-Modell", 6, "Büro-…"},
		{"CJK zählt doppelt", "通义千问", 5, "通义…"},
		{"Breite 0", "llama3", 0, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := truncToWidth(tc.in, tc.width)
			if got != tc.want {
				t.Errorf("truncToWidth(%q, %d) = %q, erwartet %q", tc.in, tc.width, got, tc.want)
			}
			if w := lipgloss.Width(got); w > tc.width {
				t.Errorf("Ergebnis %q belegt %d Spalten, erlaubt sind %d", got, w, tc.width)
			}
		})
	}
}

// TestPadToWidthColumns hält fest, warum hier nicht fmt's "%-*s" steht: dessen
// Breite zählt Runen. "Büro" wäre damit korrekt, "通义" aber nicht — zwei Runen,
// vier Spalten, also zwei Spalten zu breit.
func TestPadToWidthColumns(t *testing.T) {
	for _, s := range []string{"Server", "Büro", "通义", "", "Präsentation"} {
		got := padToWidth(s, 12)
		if w := lipgloss.Width(got); w != 12 {
			t.Errorf("padToWidth(%q, 12) belegt %d Spalten, erwartet 12", s, w)
		}
	}
	// Zu langer Text wird nicht abgeschnitten, nur nicht aufgefüllt.
	if got := padToWidth("viel-zu-langer-name", 5); got != "viel-zu-langer-name" {
		t.Errorf("padToWidth hat gekürzt: %q", got)
	}
}

// TestDiscoverModelListAlignment prüft die eigentliche Regression: Modellnamen
// unterschiedlicher Byte- und Spaltenbreite müssen die Quellenspalte am selben
// Punkt beginnen lassen, und keine Zeile darf über die Terminalbreite laufen.
func TestDiscoverModelListAlignment(t *testing.T) {
	setLang("de")

	const termWidth = 80
	const src = ":11434"

	m := model{width: termWidth, discStep: discPickModel, modelSel: 0}
	m.viewport = viewport.New(termWidth, 20)
	m.discModels = []foundModel{
		{Name: "llama3", Source: src},
		{Name: "qwen3-30b-a3b-instruct", Source: src},
		{Name: "Büro-Modell-mit-Umlauten", Source: src}, // mehr Bytes als Runen
		{Name: "通义千问-7b", Source: src},                  // mehr Spalten als Runen
	}

	var cols []int
	for _, line := range strings.Split(m.renderDiscoverContent(), "\n") {
		plain := stripANSI(line)
		idx := strings.Index(plain, src)
		if idx < 0 {
			continue
		}
		cols = append(cols, lipgloss.Width(plain[:idx]))
		if w := lipgloss.Width(plain); w > termWidth {
			t.Errorf("Zeile belegt %d Spalten (max %d): %q", w, termWidth, plain)
		}
	}

	if len(cols) != len(m.discModels) {
		t.Fatalf("%d Modellzeilen gefunden, erwartet %d", len(cols), len(m.discModels))
	}
	for i, c := range cols {
		if c != cols[0] {
			t.Errorf("Quellenspalte in Zeile %d beginnt bei Spalte %d, Zeile 0 bei %d (alle: %v)",
				i, c, cols[0], cols)
		}
	}
}
