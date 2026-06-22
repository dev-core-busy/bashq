package main

import "github.com/charmbracelet/lipgloss"

var (
	// Farben
	colorTitle    = lipgloss.Color("#1e3a5f")
	colorTitleFg  = lipgloss.Color("#e8f4fd")
	colorAccent   = lipgloss.Color("#4fc3f7")
	colorGreen    = lipgloss.Color("#81c784")
	colorYellow   = lipgloss.Color("#fff176")
	colorRed      = lipgloss.Color("#ef5350")
	colorDim      = lipgloss.Color("#546e7a")
	colorDivider  = lipgloss.Color("#37474f")
	colorACBg     = lipgloss.Color("#1a2332")
	colorACHlBg   = lipgloss.Color("#1e3a5f")
	colorACHlFg   = lipgloss.Color("#4fc3f7")
	colorUserBg   = lipgloss.Color("#1b3a1b")
	colorUserFg   = lipgloss.Color("#81c784")
	colorAgentBg  = lipgloss.Color("#0d2137")
	colorAgentFg  = lipgloss.Color("#4fc3f7")

	// Titelzeile
	titleBarStyle = lipgloss.NewStyle().
			Background(colorTitle).
			Foreground(colorTitleFg).
			Bold(true).
			Padding(0, 1)

	titleVersionStyle = lipgloss.NewStyle().
				Background(colorTitle).
				Foreground(colorAccent).
				Bold(false)

	// Trennlinie
	dividerStyle = lipgloss.NewStyle().
			Foreground(colorDivider)

	// Benutzer-Label
	userLabelStyle = lipgloss.NewStyle().
			Background(colorUserBg).
			Foreground(colorUserFg).
			Bold(true).
			Padding(0, 1)

	// Assistent-Label
	assistantLabelStyle = lipgloss.NewStyle().
				Background(colorAgentBg).
				Foreground(colorAgentFg).
				Bold(true).
				Padding(0, 1)

	// Systemnachricht (Befehlsausgabe, Info)
	systemMsgStyle = lipgloss.NewStyle().
			Foreground(colorYellow)

	// Fehlermeldung
	errorMsgStyle = lipgloss.NewStyle().
			Foreground(colorRed)

	// Dimmer Hilfetext
	dimStyle = lipgloss.NewStyle().
			Foreground(colorDim).
			Italic(true)

	// Eingabe-Prompt
	promptStyle = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	inputTextStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#eceff1"))

	inputCursorStyle = lipgloss.NewStyle().Reverse(true)

	placeholderStyle = lipgloss.NewStyle().
				Foreground(colorDim)

	// Autocomplete
	acItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#90a4ae")).
			Padding(0, 1)

	acItemSelectedStyle = lipgloss.NewStyle().
				Background(colorACHlBg).
				Foreground(colorACHlFg).
				Bold(true).
				Padding(0, 1)

	acDescStyle = lipgloss.NewStyle().
			Foreground(colorDim)

	// Bestätigungsdialog
	confirmCmdStyle = lipgloss.NewStyle().
			Foreground(colorYellow).
			Bold(true)

	confirmExplStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#90a4ae"))

	// Hinweiszeile
	hintStyle = lipgloss.NewStyle().
			Foreground(colorDim)

	hintKeyStyle = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	// Ladeindikator
	loadingStyle = lipgloss.NewStyle().
			Foreground(colorAccent)

	// Titelzeile: Modus-Badges
	autoModeBadgeStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#2e7d32")).
				Foreground(lipgloss.Color("#e8f5e9")).
				Bold(true).
				Padding(0, 1)

	askModeBadgeStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#bf360c")).
				Foreground(lipgloss.Color("#fbe9e7")).
				Bold(true).
				Padding(0, 1)

	sessionBadgeOnStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#1565c0")).
				Foreground(lipgloss.Color("#e3f2fd")).
				Bold(true).
				Padding(0, 1)

	sessionBadgeOffStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#37474f")).
				Foreground(lipgloss.Color("#b0bec5")).
				Bold(true).
				Padding(0, 1)

	// Bevorzugtes Profil (★ in Profilliste)
	preferredStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffd54f")).
			Bold(true)

	dimTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#78909c"))

	// Konfigurationseditor
	configTitleStyle = lipgloss.NewStyle().
				Foreground(colorAccent).
				Bold(true)

	configLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#78909c"))

	configLabelSelectedStyle = lipgloss.NewStyle().
					Foreground(colorAccent).
					Bold(true)

	configValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#cfd8dc"))

	configValueSelectedStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("#eceff1")).
					Bold(true)

	configEditingStyle = lipgloss.NewStyle().
				Foreground(colorDim).
				Italic(true)

	configAutoStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#a5d6a7")).
				Bold(true)

	configAskStyle = lipgloss.NewStyle().
			Foreground(colorYellow).
			Bold(true)

	// Abschnittsüberschriften im Config-Editor
	sectionStyle = lipgloss.NewStyle().
			Foreground(colorDim).
			Bold(true)
)

// noStyleDef gibt einen leeren Lipgloss-Style zurück (für Textarea-Rahmen entfernen)
func noStyleDef() lipgloss.Style {
	return lipgloss.NewStyle()
}
