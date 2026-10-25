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

// setAmbiguousWide setzt ambiguousWide für einen Test. Ohne das hinge das
// Ergebnis von der Locale der Testmaschine ab (go-runewidth liest LANG/LC_*).
func setAmbiguousWide(t *testing.T, v bool) {
	t.Helper()
	old := ambiguousWide
	ambiguousWide = v
	t.Cleanup(func() { ambiguousWide = old })
}

// assertMaxWidth prüft, dass keine gerenderte Zeile über width hinausläuft.
func assertMaxWidth(t *testing.T, out string, width int) {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		if w := cellWidth(stripANSI(line)); w > width {
			t.Errorf("Zeile belegt %d Spalten (max %d): %q", w, width, stripANSI(line))
		}
	}
}

func TestTruncToWidth(t *testing.T) {
	setAmbiguousWide(t, false)
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
		// Graphem-Cluster: runenweise gemessen wäre ❤️ (U+2764 U+FE0F) nur eine Spalte.
		{"Emoji mit VS16", "❤️❤️❤️❤️", 5, "❤️❤️…"},
		{"Hautton bleibt am Emoji", "👍🏽👍🏽👍🏽", 4, "👍🏽…"},
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

func TestTruncLeftToWidth(t *testing.T) {
	setAmbiguousWide(t, false)
	tests := []struct {
		name  string
		in    string
		width int
		want  string
	}{
		{"passt unverändert", ":11434", 10, ":11434"},
		{"URL behält Host:Port", "http://192.168.100.200:11434", 12, "…0.200:11434"},
		{"CJK zählt doppelt", "通义千问", 5, "…千问"},
		{"Emoji bleibt ganz", "👍🏽👍🏽👍🏽", 4, "…👍🏽"},
		{"Breite 0", ":11434", 0, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := truncLeftToWidth(tc.in, tc.width)
			if got != tc.want {
				t.Errorf("truncLeftToWidth(%q, %d) = %q, erwartet %q", tc.in, tc.width, got, tc.want)
			}
			if w := lipgloss.Width(got); w > tc.width {
				t.Errorf("Ergebnis %q belegt %d Spalten, erlaubt sind %d", got, w, tc.width)
			}
		})
	}
}

// TestAmbiguousWide: In Terminals mit CJK-Locale belegen "…" und "▶" zwei Spalten.
// Kürzung und Pfeil-Einrückung müssen das einrechnen.
func TestAmbiguousWide(t *testing.T) {
	setAmbiguousWide(t, true)
	if w := cellWidth("▶ "); w != 3 {
		t.Errorf(`cellWidth("▶ ") = %d, erwartet 3`, w)
	}
	if w := cellWidth(markerPrefix(false)); w != cellWidth(markerPrefix(true)) {
		t.Errorf("unmarkierte Zeile %d Spalten eingerückt, markierte %d", w, cellWidth(markerPrefix(true)))
	}
	if got := truncToWidth("llama3-instruct", 8); got != "llama3…" {
		t.Errorf("truncToWidth = %q, erwartet %q", got, "llama3…")
	}
	// Passt nicht einmal das Auslassungszeichen, wird ohne gekürzt.
	if got := truncToWidth("llama3", 1); got != "l" {
		t.Errorf("truncToWidth(…, 1) = %q, erwartet %q", got, "l")
	}
	if got := truncLeftToWidth(":11434", 4); got != "…34" {
		t.Errorf("truncLeftToWidth = %q, erwartet %q", got, "…34")
	}
}

// TestPadToWidthColumns hält fest, warum hier nicht fmt's "%-*s" steht: dessen
// Breite zählt Runen. "Büro" wäre damit korrekt, "通义" aber nicht — zwei Runen,
// vier Spalten, also zwei Spalten zu breit.
func TestPadToWidthColumns(t *testing.T) {
	setAmbiguousWide(t, false)
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

// sourceColumns liefert je Zeile, die src enthält, die Spalte, an der src beginnt.
// LastIndex statt Index: die Quelle steht am Zeilenende, der Modellname davor darf
// dieselbe Zeichenkette enthalten.
func sourceColumns(out, src string) []int {
	var cols []int
	for _, line := range strings.Split(out, "\n") {
		plain := stripANSI(line)
		if idx := strings.LastIndex(plain, src); idx >= 0 {
			cols = append(cols, cellWidth(plain[:idx]))
		}
	}
	return cols
}

func assertSameColumn(t *testing.T, cols []int, want int) {
	t.Helper()
	if len(cols) != want {
		t.Fatalf("%d Zeilen gefunden, erwartet %d", len(cols), want)
	}
	for i, c := range cols {
		if c != cols[0] {
			t.Errorf("Spalte in Zeile %d beginnt bei %d, Zeile 0 bei %d (alle: %v)", i, c, cols[0], cols)
		}
	}
}

// TestDiscoverModelListAlignment prüft die eigentliche Regression: Modellnamen
// unterschiedlicher Byte- und Spaltenbreite müssen die Quellenspalte am selben
// Punkt beginnen lassen, und keine Zeile darf über die Terminalbreite laufen –
// auch dann nicht, wenn das Terminal mehrdeutige Zeichen doppelt breit zeigt.
func TestDiscoverModelListAlignment(t *testing.T) {
	setLang("de")

	const termWidth = 80
	const src = ":11434"

	for _, wide := range []bool{false, true} {
		setAmbiguousWide(t, wide)
		m := model{width: termWidth, discStep: discPickModel, modelSel: 0}
		m.viewport = viewport.New(termWidth, 20)
		m.discModels = []foundModel{
			{Name: "llama3", Source: src},
			{Name: "qwen3-30b-a3b-instruct", Source: src},
			{Name: "Büro-Modell-mit-Umlauten", Source: src}, // mehr Bytes als Runen
			{Name: "通义千问-7b", Source: src},                  // mehr Spalten als Runen
			{Name: "proxy:11434-model", Source: src},        // Quelle im Namen
			{Name: "❤️-chat", Source: src},                  // Graphem-Cluster
		}

		out := m.renderDiscoverContent()
		assertMaxWidth(t, out, termWidth)
		assertSameColumn(t, sourceColumns(out, src), len(m.discModels))
	}
}

// TestDiscoverModelListNarrow: Bei schmalem Terminal und langer Quelle darf
// weder eine Modellzeile noch die Fundzeile überlaufen, und das ausgewählte
// letzte Modell muss im Viewport bleiben.
func TestDiscoverModelListNarrow(t *testing.T) {
	setLang("de")
	setAmbiguousWide(t, false)

	const termWidth, viewH = 30, 9
	m := model{width: termWidth, discStep: discPickModel}
	m.viewport = viewport.New(termWidth, viewH)
	for _, name := range []string{"llama3", "qwen3-30b", "mistral-7b", "phi3", "gemma2", "letztes-modell"} {
		m.discModels = append(m.discModels, foundModel{Name: name, Source: "http://192.168.100.200:11434"})
	}
	m.modelSel = len(m.discModels) - 1
	m.updateModelWindow()

	out := m.renderDiscoverContent()
	assertMaxWidth(t, out, termWidth)
	if n := strings.Count(out, "\n"); n > viewH {
		t.Errorf("Ausgabe hat %d Zeilen, Viewport nur %d", n, viewH)
	}
	if !strings.Contains(stripANSI(out), "▶ letztes…") {
		t.Errorf("ausgewähltes Modell nicht sichtbar:\n%s", stripANSI(out))
	}
}

// TestProfileListColumns: Die URL-Spalte der Profilliste beginnt unabhängig von
// der Spaltenbreite des Profilnamens an derselben Stelle.
func TestProfileListColumns(t *testing.T) {
	setLang("de")
	setAmbiguousWide(t, false)

	const url = "http://localhost:11434/v1"
	m := model{width: 80}
	m.cfg.profiles = []llmProfile{
		{Name: "llama", BaseURL: url},
		{Name: "通义千问", BaseURL: url},
		{Name: "Büro-Server", BaseURL: url},
		{Name: "ein-sehr-langer-profilname", BaseURL: url},
	}
	m.cfg.activeProfileIdx = 1
	m.profileSel = -1

	out := m.renderConfigContent()
	assertMaxWidth(t, out, m.width)
	assertSameColumn(t, sourceColumns(out, url), len(m.cfg.profiles))

	m.width = 30
	assertMaxWidth(t, m.renderConfigContent(), m.width)
}

// TestConfigLabelsCJK: Chinesische Labels belegen je Zeichen zwei Spalten. Die
// Wertespalte muss trotzdem für alle Felder an derselben Stelle beginnen, und bei
// schmalem Terminal darf keine Zeile überlaufen.
func TestConfigLabelsCJK(t *testing.T) {
	setLang("zh")
	t.Cleanup(func() { setLang("de") })
	setAmbiguousWide(t, false)

	m := model{width: 80}
	m.cfgSection = 1
	m.configSel = -1
	m.profileSel = -1
	m.cfg.shortcuts[0] = "df -h"
	m.cfg.llmTimeout = 120

	out := m.renderConfigContent()
	assertMaxWidth(t, out, m.width)
	timeoutCol := sourceColumns(out, "120 "+L.FieldTimeoutHint)
	shortcutCol := sourceColumns(out, "df -h")
	if len(timeoutCol) != 1 || len(shortcutCol) != 1 {
		t.Fatalf("Felder nicht gefunden: timeout %v, F1 %v", timeoutCol, shortcutCol)
	}
	if timeoutCol[0] != shortcutCol[0] {
		t.Errorf("Wert hinter %q beginnt bei Spalte %d, hinter F1 bei %d",
			L.FieldTimeout, timeoutCol[0], shortcutCol[0])
	}

	m.width = 30
	assertMaxWidth(t, m.renderConfigContent(), m.width)
}
