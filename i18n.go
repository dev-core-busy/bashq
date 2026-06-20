package main

import (
	"os"
	"strings"
)

// L ist die aktive Sprachinstanz; wird in setLang() gesetzt.
var L *UIStrings

// UIStrings enthält alle benutzer­sichtbaren Texte der Anwendung.
type UIStrings struct {
	LangCode string // "de" | "en"
	LangName string // nativer Name für die Sprachauswahl

	// Titelzeile
	TitleApp    string
	TitleConfig string
	BadgeAuto   string
	BadgeAsk    string
	Starting    string

	// Untere Leiste – Idle
	InputPlaceholder  string
	HintSlashKey      string // Taste "/"
	HintSlashLabel    string // "Befehle"
	HintScrollKeys    string // "↑↓"
	HintScrollLabel   string // "scrollen"
	HintModeKey       string // "Shift+Tab"
	HintModeLabel     string // "Modus"
	HintShortcutKeys  string // "F1–F9"
	HintShortcutLabel string // "Kürzel"
	ACHint            string
	ACMore            string // "%d weitere"

	// Untere Leiste – Bestätigung
	ConfirmYesKey    string // "J/Enter"
	ConfirmNoKey     string // "N/Esc"
	ConfirmRunLabel  string // "Ausführen"
	ConfirmCancelLabel string // "Abbrechen"

	// Untere Leiste – Laden/Ausführen
	LoadingThinking   string // "Denke nach…"
	LoadingQuit       string // "Ctrl+C Beenden"
	LoadingRunning    string // "Führe aus: "
	LoadingCmdRunning string // "Befehl läuft…"

	// Untere Leiste – Konfiguration
	ConfigNavKeys     string // "↑↓"
	ConfigNavLabel    string // "navigieren"
	ConfigEditKey     string // "Enter"
	ConfigEditLabel   string // "bearbeiten"
	ConfigCloseKey    string // "Esc"
	ConfigCloseLabel  string // "schließen"
	ConfigModeHint    string // "Ausführmodus umschalten"
	ConfigSaveKey     string // "Enter"
	ConfigSaveLabel   string // "speichern"
	ConfigCancelKey   string // "Esc"
	ConfigCancelLabel string // "abbrechen"
	ConfigEditingBelow string // "[bearbeite unten]"

	// Untere Leiste – Prompt-Editor
	PromptSaveKey    string // "Ctrl+S"
	PromptSaveLabel  string // "speichern"
	PromptCancelKey  string // "Esc"
	PromptCancelLabel string // "abbrechen"
	PromptDesc       string // "System-Prompt wird dem Assistenten vorangestellt"

	// Textarea-Platzhalter
	TextareaPlaceholder string

	// Konfigurations-Abschnitte
	SectionProfiles   string
	SectionConnection string
	SectionAssistant  string
	SectionShortcuts  string
	SectionLanguage   string

	// Konfigurations-Feldbezeichnungen
	FieldActiveProfile string // "Aktives Profil"
	FieldEndpoint      string
	FieldModel         string
	FieldAPIKey        string
	FieldMode          string
	FieldPrompt        string
	FieldLang          string
	FieldSession       string // "Sitzungen speichern"
	SectionSession     string // "── SITZUNGEN ────────────────────────"
	FieldAutoUpdate    string // "Auto-Update"
	SectionUpdate      string // "── AUTO-UPDATE ──────────────────────"

	// Konfigurationswerte
	FieldAPIKeyEmpty   string // "(leer – für lokale LLMs)"
	FieldPromptEmpty   string // "(leer – Enter zum Bearbeiten)"
	FieldShortcutEmpty string // "(nicht belegt)"
	ModeAuto           string // "⚡ Auto-Ausführen"
	ModeAsk            string // "🛡 Fragen"
	ModeToggleHint     string // "(Space/Enter)"
	ModeSessionOn      string // "An"
	ModeSessionOff     string // "Aus"
	ModeUpdateAsk      string // "Nachfragen"
	ModeUpdateAuto     string // "Automatisch"
	ModeUpdateOff      string // "Deaktiviert"

	// Titel-Badges
	BadgeSessionOn  string // " SESSION "
	BadgeSessionOff string // " SESSION "

	// Systemnachrichten (update.go)
	MsgUpdateAvailable  string // "🔄 Neue Version %s verfügbar…"
	MsgUpdateDownloading string // "⬇ Lade Update %s herunter…"
	MsgUpdateDone       string // "✓ bashq %s installiert. Bitte neu starten."
	MsgUpdateUpToDate   string // "✓ bashq ist aktuell (%s)"
	MsgUpdateError      string // "⚠ Update fehlgeschlagen: %s"
	MsgModeAuto       string // "⚡ Modus: Auto-Ausführen ..."
	MsgModeAsk        string // "🛡 Modus: Fragen ..."
	MsgSessionOn      string // "💾 Sitzungen werden gespeichert …"
	MsgSessionOff     string // "🚫 Sitzungen werden nicht gespeichert …"
	MsgShortcutEmpty  string // "F%d ist nicht belegt..."
	MsgCancelled      string // "✗ Abgebrochen"
	MsgAutoExecFmt    string // "⚡ Auto: $ %s\n  %s"
	MsgConfirmCmdFmt  string // "Ich möchte folgenden Befehl ausführen:..."
	MsgNoOutput       string // "(kein Output)"
	MsgExitError      string // "⚠ Exit-Fehler: "
	MsgToolRejected   string // Tool-Ablehnung an LLM

	// Chat-Bereich
	WelcomeMsg             string
	LabelUser              string // " Du "
	LabelAssistant         string // " Assistent "
	SessionRestoredFmt string // "📂 Sitzung vom %s fortgesetzt"

	// Aktivitätsprotokoll
	ActivitiesTitle   string
	ActivitiesEmpty   string
	ActivitiesMoreFmt string // "(zeige letzte %d von %d Einträgen)"
	ActivitiesLogPath string
	KindUser          string
	KindAgent         string
	KindExec          string
	KindError         string

	// System-Prompt für das LLM
	SystemPrompt string

	// Tool-Definitionen
	ToolDesc       string
	ToolArgCmd     string
	ToolArgExpl    string
	ExtraPromptFmt string // Präfix für customPrompt

	// Profil-Verwaltung
	ProfileActive      string // "● aktiv"
	ProfilePreferred   string // "★"
	ProfileAddBtn      string // "[ + Neues LLM-Profil ]"
	ProfileTabSettings string // "Tab: Einstellungen"
	ProfileTabProfiles string // "Tab: Profile"
	ProfilePreferLabel string // "bevorzugen"
	ProfileDeleteLabel string // "löschen"
	ProfileActivatedFmt string // "Profil '%s' aktiviert"

	// Discovery (stateDiscover)
	DiscoveryTitle           string
	DiscoveryInputLabel      string
	DiscoveryInputHint       string
	DiscoverySearchLabel     string
	DiscoveryScanFmt         string
	DiscoveryScanPorts       string
	DiscoveryNoneFmt         string
	DiscoveryFoundFmt        string
	DiscoveryNameLabel       string
	DiscoveryNamePlaceholder string
	DiscoverySavedFmt        string

	// Health-Check
	HealthOkFmt      string
	HealthFailFmt    string
	HealthSuggestFmt string
	HealthNoFallback string

	// Slash-Befehlsbeschreibungen
	CmdInstallDesc    string
	CmdInstallPrompt  string
	CmdRemoveDesc     string
	CmdRemovePrompt   string
	CmdUpdateDesc     string
	CmdUpdateMsg      string
	CmdStatusDesc     string
	CmdStatusMsg      string
	CmdDiskDesc       string
	CmdDiskMsg        string
	CmdMemoryDesc     string
	CmdMemoryMsg      string
	CmdNetworkDesc    string
	CmdNetworkMsg     string
	CmdServicesDesc   string
	CmdServicesPrompt string
	CmdLogsDesc       string
	CmdLogsMsg        string
	CmdOptimizeDesc   string
	CmdOptimizeMsg    string
	CmdConfigDesc     string
	CmdSetupDesc      string // "/setup"
	CmdColorsDesc     string // "/colors"
	CmdActivitiesDesc string
	CmdHelpDesc       string
	CmdClearDesc      string
	CmdExitDesc       string
	HelpText          string
}

var de = UIStrings{
	LangCode: "de",
	LangName: "Deutsch",

	TitleApp:    "bashq",
	TitleConfig: "Einstellungen",
	BadgeAuto:   " AUTO ",
	BadgeAsk:    " ASK ",
	Starting:    "Starte…",

	InputPlaceholder:  "Nachricht eingeben oder / für Befehle…",
	HintSlashKey:      "/",
	HintSlashLabel:    "Befehle",
	HintScrollKeys:    "↑↓",
	HintScrollLabel:   "scrollen",
	HintModeKey:       "Shift+Tab",
	HintModeLabel:     "Modus",
	HintShortcutKeys:  "F1–F9/Alt+1–9",
	HintShortcutLabel: "Kürzel",
	ACHint:            "  ↑↓/Tab: wählen · Enter: bestätigen · Esc: schließen",
	ACMore:            "… %d weitere",

	ConfirmYesKey:      "J/Enter",
	ConfirmNoKey:       "N/Esc",
	ConfirmRunLabel:    "Ausführen",
	ConfirmCancelLabel: "Abbrechen",

	LoadingThinking:   " Denke nach…",
	LoadingQuit:       "  Ctrl+C Beenden",
	LoadingRunning:    " Führe aus: ",
	LoadingCmdRunning: "  Befehl läuft…",

	ConfigNavKeys:      "↑↓",
	ConfigNavLabel:     "navigieren",
	ConfigEditKey:      "Enter",
	ConfigEditLabel:    "bearbeiten",
	ConfigCloseKey:     "Esc",
	ConfigCloseLabel:   "schließen",
	ConfigModeHint:     "Ausführmodus umschalten",
	ConfigSaveKey:      "Enter",
	ConfigSaveLabel:    "speichern",
	ConfigCancelKey:    "Esc",
	ConfigCancelLabel:  "abbrechen",
	ConfigEditingBelow: "[bearbeite unten]",

	PromptSaveKey:       "Ctrl+S",
	PromptSaveLabel:     "speichern",
	PromptCancelKey:     "Esc",
	PromptCancelLabel:   "abbrechen",
	PromptDesc:          "  System-Prompt wird dem Assistenten vorangestellt",
	TextareaPlaceholder: "Zusätzliche Anweisungen für den Assistenten… (Ctrl+S speichern)",

	SectionProfiles:   "── LLM-PROFILE ──────────────────────",
	SectionConnection: "── VERBINDUNG ───────────────────────",
	SectionAssistant:  "── ASSISTENT ────────────────────────",
	SectionShortcuts:  "── TASTENKÜRZEL (F1–F9) ─────────────",
	SectionLanguage:   "── SPRACHE ──────────────────────────",

	ProfileActive:       "● aktiv",
	ProfilePreferred:    "★",
	ProfileAddBtn:       "[ + Neues LLM-Profil ]",
	ProfileTabSettings:  "Tab: Einstellungen",
	ProfileTabProfiles:  "Tab: Profile",
	ProfilePreferLabel:  "bevorzugen",
	ProfileDeleteLabel:  "löschen",
	ProfileActivatedFmt: "Profil '%s' aktiviert",

	DiscoveryTitle:           "Neues LLM-Profil",
	DiscoveryInputLabel:      "IP/Hostname",
	DiscoveryInputHint:       "IP  ·  IP:Port  ·  http://IP:Port/v1",
	DiscoverySearchLabel:     "Suchen",
	DiscoveryScanFmt:         "Scanne %s …",
	DiscoveryScanPorts:       "Ports: 11434  1234  8080  8000  9081  7860  5000  3000",
	DiscoveryNoneFmt:         "Keine LLMs gefunden auf '%s'",
	DiscoveryFoundFmt:        "%d Modell(e) gefunden – ↑↓ navigieren, Enter auswählen",
	DiscoveryNameLabel:       "Profilname",
	DiscoveryNamePlaceholder: "(leer = Modellname als Name verwenden)",
	DiscoverySavedFmt:        "LLM-Profil '%s' gespeichert",

	HealthOkFmt:      "✓ LLM-Profil '%s' ist erreichbar",
	HealthFailFmt:    "⚠ LLM-Profil '%s' ist nicht erreichbar",
	HealthSuggestFmt: "  → Nächste Option: '%s'  (in /config wechseln)",
	HealthNoFallback: "  → Kein weiteres erreichbares Profil. Bitte /config prüfen.",

	FieldActiveProfile: "Aktives Profil",
	FieldEndpoint:      "LLM-Endpunkt",
	FieldModel:         "Modell",
	FieldAPIKey:        "API-Key",
	FieldMode:          "Ausführmodus",
	FieldPrompt:        "System-Prompt",
	FieldLang:          "Sprache",
	FieldSession:       "Sitzungen speichern",
	SectionSession:     "── SITZUNGEN ────────────────────────",
	FieldAutoUpdate:    "Auto-Update",
	SectionUpdate:      "── AUTO-UPDATE ──────────────────────",

	FieldAPIKeyEmpty:   "(leer – für lokale LLMs)",
	FieldPromptEmpty:   "(leer – Enter zum Bearbeiten)",
	FieldShortcutEmpty: "(nicht belegt)",
	ModeAuto:           "Auto-Ausführen",
	ModeAsk:            "Fragen",
	ModeToggleHint:     "(Space/Enter)",
	ModeSessionOn:      "An",
	ModeSessionOff:     "Aus",
	ModeUpdateAsk:      "Nachfragen",
	ModeUpdateAuto:     "Automatisch",
	ModeUpdateOff:      "Deaktiviert",
	BadgeSessionOn:     " SESSION ",
	BadgeSessionOff:    " SESSION ",

	MsgUpdateAvailable:   "🔄 Neue Version %s verfügbar. Alt+U installieren · Esc ignorieren",
	MsgUpdateDownloading: "⬇ Lade bashq %s herunter…",
	MsgUpdateDone:        "✓ bashq %s installiert — bitte neu starten",
	MsgUpdateUpToDate:    "✓ bashq ist aktuell (%s)",
	MsgUpdateError:       "⚠ Update fehlgeschlagen: %s",
	MsgModeAuto:      "⚡ Modus: Auto-Ausführen (Befehle werden ohne Rückfrage ausgeführt)",
	MsgModeAsk:       "🛡 Modus: Fragen (Befehle werden vor Ausführung bestätigt)",
	MsgSessionOn:     "💾 Sitzungen werden gespeichert (Alt+S zum Umschalten)",
	MsgSessionOff:    "🚫 Sitzungen werden nicht gespeichert (Alt+S zum Umschalten)",
	MsgShortcutEmpty: "F%d ist nicht belegt. Belegen über /config → Tastenkürzel.",
	MsgCancelled:     "✗ Abgebrochen",
	MsgAutoExecFmt:   "⚡ Auto: $ %s\n  %s",
	MsgConfirmCmdFmt: "Ich möchte folgenden Befehl ausführen:\n\n  $ %s\n\n%s",
	MsgNoOutput:      "(kein Output)",
	MsgExitError:     "⚠ Exit-Fehler: ",
	MsgToolRejected:  "FEHLER: Der Benutzer hat die Ausführung abgelehnt.",

	WelcomeMsg:         "\n  bashq bereit – das Q-Kontinuum deines Terminals.\n  Stell mir eine Frage oder tippe / für verfügbare Befehle.\n",
	LabelUser:          " Du ",
	LabelAssistant:     " Assistent ",
	SessionRestoredFmt: "📂 Sitzung vom %s fortgesetzt",

	ActivitiesTitle:   "📋 Aktivitätsprotokoll",
	ActivitiesEmpty:   "  (noch keine Einträge in dieser Sitzung)",
	ActivitiesMoreFmt: "  (zeige letzte %d von %d Einträgen)\n\n",
	ActivitiesLogPath: "  Vollständiges Log: ",
	KindUser:          "Benutzer  ",
	KindAgent:         "Agent     ",
	KindExec:          "Ausführung",
	KindError:         "Fehler    ",

	SystemPrompt: `Du bist ein freundlicher und geduldiger Linux-Assistent für Benutzer ohne Linux-Kenntnisse.

DEINE AUFGABE:
- Hilf dem Benutzer bei der Verwaltung seines Debian-Linux-Systems
- Erkläre alles in einfacher, verständlicher deutscher Sprache
- Führe Systembefehle über das execute_command Tool aus

WICHTIGE REGELN:
- Antworte IMMER auf Deutsch
- Erkläre vor jedem Befehl kurz und verständlich, was er tut
- Bei gefährlichen Operationen (Löschen, Formatieren) besonders deutlich warnen
- Halte Antworten kurz und präzise – kein technisches Fachwissen voraussetzen
- Wenn du mehrere Befehle brauchst, führe sie nacheinander aus
- Zeige Ergebnisse in einer lesbaren Form

TOOL-NUTZUNG:
- Nutze execute_command für alle Systembefehle
- Erkläre im "explanation"-Feld was der Befehl macht (einfache Sprache)
- Fasse Befehle wenn möglich zusammen (z.B. mit &&)`,

	ToolDesc:       "Führt einen Shell-Befehl auf dem Linux-System aus und gibt die Ausgabe zurück",
	ToolArgCmd:     "Der auszuführende Shell-Befehl (bash)",
	ToolArgExpl:    "Kurze, einfache Erklärung was dieser Befehl tut (für den Benutzer)",
	ExtraPromptFmt: "## Zusätzliche Anweisungen (haben Vorrang):\n%s\n\n---\n\n%s",

	CmdInstallDesc:    "Software installieren",
	CmdInstallPrompt:  "Installiere bitte: ",
	CmdRemoveDesc:     "Software entfernen",
	CmdRemovePrompt:   "Entferne bitte: ",
	CmdUpdateDesc:     "System aktualisieren",
	CmdUpdateMsg:      "Aktualisiere bitte mein System (apt update && apt upgrade)",
	CmdStatusDesc:     "Systemstatus anzeigen",
	CmdStatusMsg:      "Zeig mir einen kompakten Überblick über den Systemstatus (CPU, RAM, Festplatte, Uptime)",
	CmdDiskDesc:       "Festplattennutzung",
	CmdDiskMsg:        "Wie viel Festplattenplatz habe ich noch frei?",
	CmdMemoryDesc:     "Arbeitsspeicher anzeigen",
	CmdMemoryMsg:      "Zeig mir die aktuelle Arbeitsspeicher-Auslastung",
	CmdNetworkDesc:    "Netzwerkinformationen",
	CmdNetworkMsg:     "Zeig mir meine Netzwerkkonfiguration und IP-Adressen",
	CmdServicesDesc:   "Systemdienste verwalten",
	CmdServicesPrompt: "Systemdienst: ",
	CmdLogsDesc:       "Systemlogs anzeigen",
	CmdLogsMsg:        "Zeig mir die wichtigsten aktuellen Systemlogs (letzte 20 Zeilen)",
	CmdOptimizeDesc:   "System optimieren",
	CmdOptimizeMsg:    "Was kann ich tun um mein Debian-System zu optimieren und schneller zu machen?",
	CmdConfigDesc:     "Einstellungen bearbeiten",
	CmdSetupDesc:      "bashq systemweit installieren / deinstallieren",
	CmdColorsDesc:     "Terminal-Farben in ~/.bashrc einrichten",
	CmdActivitiesDesc: "Aktivitätsprotokoll anzeigen",
	CmdHelpDesc:       "Hilfe anzeigen",
	CmdClearDesc:      "Chat-Verlauf leeren",
	CmdExitDesc:       "Programm beenden",

	HelpText: `bashq – Hilfe

Schreibe auf Deutsch was du tun möchtest.
Ich erkläre jeden Schritt und frage vor gefährlichen Aktionen nach.

SLASH-BEFEHLE (tippe / für Autovervollständigung):
  /install    – Software installieren
  /remove     – Software entfernen
  /update     – System aktualisieren
  /status     – Systemstatus anzeigen
  /disk       – Festplattennutzung
  /memory     – Arbeitsspeicher
  /network    – Netzwerkinformationen
  /services   – Dienste verwalten
  /logs       – Systemlogs
  /optimize   – Optimierungstipps
  /config     – Einstellungen (LLM, System-Prompt, Tastenkürzel, Sprache)
  /setup      – bashq systemweit installieren / deinstallieren
  /activities – Aktivitätsprotokoll dieser Sitzung
  /clear      – Chat leeren
  /exit       – Beenden

TASTENKÜRZEL:
  Enter       – Nachricht senden / Auswahl bestätigen
  ↑ / ↓      – Liste navigieren / Chat scrollen
  F1–F9       – Benutzerdefinierte Kürzel (in /config belegen)
  Shift+Tab   – Ausführmodus umschalten (Fragen ↔ Auto)
  Esc         – Autovervollständigung schließen
  Ctrl+C      – Beenden`,
}

var en = UIStrings{
	LangCode: "en",
	LangName: "English",

	TitleApp:    "bashq",
	TitleConfig: "Settings",
	BadgeAuto:   " AUTO ",
	BadgeAsk:    " ASK ",
	Starting:    "Starting…",

	InputPlaceholder:  "Enter a message or type / for commands…",
	HintSlashKey:      "/",
	HintSlashLabel:    "commands",
	HintScrollKeys:    "↑↓",
	HintScrollLabel:   "scroll",
	HintModeKey:       "Shift+Tab",
	HintModeLabel:     "mode",
	HintShortcutKeys:  "F1–F9/Alt+1–9",
	HintShortcutLabel: "shortcuts",
	ACHint:            "  ↑↓/Tab: select · Enter: confirm · Esc: close",
	ACMore:            "… %d more",

	ConfirmYesKey:      "Y/Enter",
	ConfirmNoKey:       "N/Esc",
	ConfirmRunLabel:    "Execute",
	ConfirmCancelLabel: "Cancel",

	LoadingThinking:   " Thinking…",
	LoadingQuit:       "  Ctrl+C Quit",
	LoadingRunning:    " Running: ",
	LoadingCmdRunning: "  Command running…",

	ConfigNavKeys:      "↑↓",
	ConfigNavLabel:     "navigate",
	ConfigEditKey:      "Enter",
	ConfigEditLabel:    "edit",
	ConfigCloseKey:     "Esc",
	ConfigCloseLabel:   "close",
	ConfigModeHint:     "toggle execution mode",
	ConfigSaveKey:      "Enter",
	ConfigSaveLabel:    "save",
	ConfigCancelKey:    "Esc",
	ConfigCancelLabel:  "cancel",
	ConfigEditingBelow: "[editing below]",

	PromptSaveKey:       "Ctrl+S",
	PromptSaveLabel:     "save",
	PromptCancelKey:     "Esc",
	PromptCancelLabel:   "cancel",
	PromptDesc:          "  System prompt is prepended to the assistant instructions",
	TextareaPlaceholder: "Additional instructions for the assistant… (Ctrl+S to save)",

	SectionProfiles:   "── LLM PROFILES ─────────────────────",
	SectionConnection: "── CONNECTION ───────────────────────",
	SectionAssistant:  "── ASSISTANT ────────────────────────",
	SectionShortcuts:  "── SHORTCUTS (F1–F9) ────────────────",
	SectionLanguage:   "── LANGUAGE ─────────────────────────",

	ProfileActive:       "● active",
	ProfilePreferred:    "★",
	ProfileAddBtn:       "[ + New LLM Profile ]",
	ProfileTabSettings:  "Tab: Settings",
	ProfileTabProfiles:  "Tab: Profiles",
	ProfilePreferLabel:  "prefer",
	ProfileDeleteLabel:  "delete",
	ProfileActivatedFmt: "Profile '%s' activated",

	DiscoveryTitle:           "New LLM Profile",
	DiscoveryInputLabel:      "IP/Hostname",
	DiscoveryInputHint:       "IP  ·  IP:Port  ·  http://IP:Port/v1",
	DiscoverySearchLabel:     "Search",
	DiscoveryScanFmt:         "Scanning %s …",
	DiscoveryScanPorts:       "Ports: 11434  1234  8080  8000  9081  7860  5000  3000",
	DiscoveryNoneFmt:         "No LLMs found on '%s'",
	DiscoveryFoundFmt:        "%d model(s) found – ↑↓ navigate, Enter to select",
	DiscoveryNameLabel:       "Profile name",
	DiscoveryNamePlaceholder: "(empty = use model name)",
	DiscoverySavedFmt:        "LLM profile '%s' saved",

	HealthOkFmt:      "✓ LLM profile '%s' is reachable",
	HealthFailFmt:    "⚠ LLM profile '%s' is not reachable",
	HealthSuggestFmt: "  → Next option: '%s'  (switch in /config)",
	HealthNoFallback: "  → No other reachable profile available. Check /config.",

	FieldActiveProfile: "Active profile",
	FieldEndpoint:      "LLM Endpoint",
	FieldModel:         "Model",
	FieldAPIKey:        "API Key",
	FieldMode:          "Exec mode",
	FieldPrompt:        "System prompt",
	FieldLang:          "Language",
	FieldSession:       "Save sessions",
	SectionSession:     "── SESSIONS ─────────────────────────",
	FieldAutoUpdate:    "Auto-update",
	SectionUpdate:      "── AUTO-UPDATE ──────────────────────",

	FieldAPIKeyEmpty:   "(empty – for local LLMs)",
	FieldPromptEmpty:   "(empty – press Enter to edit)",
	FieldShortcutEmpty: "(not assigned)",
	ModeAuto:           "Auto-execute",
	ModeAsk:            "Ask",
	ModeToggleHint:     "(Space/Enter)",
	ModeSessionOn:      "On",
	ModeSessionOff:     "Off",
	ModeUpdateAsk:      "Ask",
	ModeUpdateAuto:     "Auto",
	ModeUpdateOff:      "Off",
	BadgeSessionOn:     " SESSION ",
	BadgeSessionOff:    " SESSION ",

	MsgUpdateAvailable:   "🔄 New version %s available. Alt+U to install · Esc to skip",
	MsgUpdateDownloading: "⬇ Downloading bashq %s…",
	MsgUpdateDone:        "✓ bashq %s installed — please restart",
	MsgUpdateUpToDate:    "✓ bashq is up to date (%s)",
	MsgUpdateError:       "⚠ Update failed: %s",
	MsgModeAuto:      "⚡ Mode: Auto-execute (commands run without confirmation)",
	MsgModeAsk:       "🛡 Mode: Ask (commands require confirmation before running)",
	MsgSessionOn:     "💾 Sessions will be saved (Alt+S to toggle)",
	MsgSessionOff:    "🚫 Sessions will not be saved (Alt+S to toggle)",
	MsgShortcutEmpty: "F%d is not assigned. Configure it in /config → Shortcuts.",
	MsgCancelled:     "✗ Cancelled",
	MsgAutoExecFmt:   "⚡ Auto: $ %s\n  %s",
	MsgConfirmCmdFmt: "I would like to run the following command:\n\n  $ %s\n\n%s",
	MsgNoOutput:      "(no output)",
	MsgExitError:     "⚠ Exit error: ",
	MsgToolRejected:  "ERROR: The user rejected the execution.",

	WelcomeMsg:         "\n  bashq ready — the Q Continuum of your terminal.\n  Ask me anything or type / for available commands.\n",
	LabelUser:          " You ",
	LabelAssistant:     " Assistant ",
	SessionRestoredFmt: "📂 Session resumed from %s",

	ActivitiesTitle:   "📋 Activity Log",
	ActivitiesEmpty:   "  (no entries in this session)",
	ActivitiesMoreFmt: "  (showing last %d of %d entries)\n\n",
	ActivitiesLogPath: "  Full log: ",
	KindUser:          "User      ",
	KindAgent:         "Agent     ",
	KindExec:          "Execution ",
	KindError:         "Error     ",

	SystemPrompt: `You are a friendly and patient Linux assistant for users without Linux knowledge.

YOUR TASK:
- Help the user manage their Debian Linux system
- Explain everything in simple, clear English
- Execute system commands using the execute_command tool

IMPORTANT RULES:
- ALWAYS respond in English
- Before each command, briefly explain what it does in plain terms
- For dangerous operations (deleting, formatting), warn clearly
- Keep answers short and precise – do not assume technical knowledge
- If you need multiple commands, run them one at a time
- Display results in a readable format

TOOL USAGE:
- Use execute_command for all system commands
- Explain in the "explanation" field what the command does (plain language)
- Combine commands where possible (e.g. with &&)`,

	ToolDesc:       "Executes a shell command on the Linux system and returns the output",
	ToolArgCmd:     "The shell command to execute (bash)",
	ToolArgExpl:    "Brief, plain-language explanation of what this command does (for the user)",
	ExtraPromptFmt: "## Additional instructions (take precedence):\n%s\n\n---\n\n%s",

	CmdInstallDesc:    "Install software",
	CmdInstallPrompt:  "Please install: ",
	CmdRemoveDesc:     "Remove software",
	CmdRemovePrompt:   "Please remove: ",
	CmdUpdateDesc:     "Update system",
	CmdUpdateMsg:      "Please update my system (apt update && apt upgrade)",
	CmdStatusDesc:     "Show system status",
	CmdStatusMsg:      "Show me a compact overview of the system status (CPU, RAM, disk, uptime)",
	CmdDiskDesc:       "Disk usage",
	CmdDiskMsg:        "How much free disk space do I have?",
	CmdMemoryDesc:     "Show memory usage",
	CmdMemoryMsg:      "Show me the current memory usage",
	CmdNetworkDesc:    "Network information",
	CmdNetworkMsg:     "Show me my network configuration and IP addresses",
	CmdServicesDesc:   "Manage system services",
	CmdServicesPrompt: "Service: ",
	CmdLogsDesc:       "Show system logs",
	CmdLogsMsg:        "Show me the most important recent system logs (last 20 lines)",
	CmdOptimizeDesc:   "Optimize system",
	CmdOptimizeMsg:    "What can I do to optimize and speed up my Debian system?",
	CmdConfigDesc:     "Edit settings",
	CmdSetupDesc:      "Install / uninstall bashq system-wide",
	CmdColorsDesc:     "Set up terminal colors in ~/.bashrc",
	CmdActivitiesDesc: "Show activity log",
	CmdHelpDesc:       "Show help",
	CmdClearDesc:      "Clear chat history",
	CmdExitDesc:       "Exit program",

	HelpText: `bashq – Help

Write in English to tell me what you need.
I explain every step and ask before dangerous actions.

SLASH COMMANDS (type / for autocomplete):
  /install    – Install software
  /remove     – Remove software
  /update     – Update system
  /status     – Show system status
  /disk       – Disk usage
  /memory     – Memory usage
  /network    – Network information
  /services   – Manage services
  /logs       – System logs
  /optimize   – Optimization tips
  /config     – Settings (LLM, system prompt, shortcuts, language)
  /setup      – Install / uninstall bashq system-wide
  /activities – Activity log for this session
  /clear      – Clear chat
  /exit       – Exit

KEYBOARD SHORTCUTS:
  Enter       – Send message / confirm selection
  ↑ / ↓      – Navigate list / scroll chat
  F1–F9       – Custom shortcuts (configure in /config)
  Shift+Tab   – Toggle execution mode (Ask ↔ Auto)
  Esc         – Close autocomplete
  Ctrl+C      – Quit`,
}

var zh = UIStrings{
	LangCode: "zh",
	LangName: "中文",

	TitleApp:    "bashq",
	TitleConfig: "设置",
	BadgeAuto:   " AUTO ",
	BadgeAsk:    " ASK ",
	Starting:    "启动中…",

	InputPlaceholder:  "输入消息或键入 / 查看命令…",
	HintSlashKey:      "/",
	HintSlashLabel:    "命令",
	HintScrollKeys:    "↑↓",
	HintScrollLabel:   "滚动",
	HintModeKey:       "Shift+Tab",
	HintModeLabel:     "模式",
	HintShortcutKeys:  "F1–F9/Alt+1–9",
	HintShortcutLabel: "快捷键",
	ACHint:            "  ↑↓/Tab: 选择 · Enter: 确认 · Esc: 关闭",
	ACMore:            "… 还有 %d 个",

	ConfirmYesKey:      "Y/Enter",
	ConfirmNoKey:       "N/Esc",
	ConfirmRunLabel:    "执行",
	ConfirmCancelLabel: "取消",

	LoadingThinking:   " 思考中…",
	LoadingQuit:       "  Ctrl+C 退出",
	LoadingRunning:    " 正在执行：",
	LoadingCmdRunning: "  命令运行中…",

	ConfigNavKeys:      "↑↓",
	ConfigNavLabel:     "导航",
	ConfigEditKey:      "Enter",
	ConfigEditLabel:    "编辑",
	ConfigCloseKey:     "Esc",
	ConfigCloseLabel:   "关闭",
	ConfigModeHint:     "切换执行模式",
	ConfigSaveKey:      "Enter",
	ConfigSaveLabel:    "保存",
	ConfigCancelKey:    "Esc",
	ConfigCancelLabel:  "取消",
	ConfigEditingBelow: "[在下方编辑]",

	PromptSaveKey:       "Ctrl+S",
	PromptSaveLabel:     "保存",
	PromptCancelKey:     "Esc",
	PromptCancelLabel:   "取消",
	PromptDesc:          "  系统提示词将附加在助手指令之前",
	TextareaPlaceholder: "助手的附加指令… (Ctrl+S 保存)",

	SectionProfiles:   "── LLM 配置文件 ──────────────────────",
	SectionConnection: "── 连接 ─────────────────────────────",
	SectionAssistant:  "── 助手 ─────────────────────────────",
	SectionShortcuts:  "── 快捷键 (F1–F9) ───────────────────",
	SectionLanguage:   "── 语言 ─────────────────────────────",

	ProfileActive:       "● 激活",
	ProfilePreferred:    "★",
	ProfileAddBtn:       "[ + 新建 LLM 配置文件 ]",
	ProfileTabSettings:  "Tab: 设置",
	ProfileTabProfiles:  "Tab: 配置文件",
	ProfilePreferLabel:  "首选",
	ProfileDeleteLabel:  "删除",
	ProfileActivatedFmt: "配置文件 '%s' 已激活",

	DiscoveryTitle:           "新建 LLM 配置文件",
	DiscoveryInputLabel:      "IP/主机名",
	DiscoveryInputHint:       "IP  ·  IP:端口  ·  http://IP:端口/v1",
	DiscoverySearchLabel:     "搜索",
	DiscoveryScanFmt:         "正在扫描 %s …",
	DiscoveryScanPorts:       "端口: 11434  1234  8080  8000  9081  7860  5000  3000",
	DiscoveryNoneFmt:         "在 '%s' 上未找到 LLM",
	DiscoveryFoundFmt:        "找到 %d 个模型 – ↑↓ 导航，Enter 选择",
	DiscoveryNameLabel:       "配置文件名称",
	DiscoveryNamePlaceholder: "（空 = 使用模型名称）",
	DiscoverySavedFmt:        "LLM 配置文件 '%s' 已保存",

	HealthOkFmt:      "✓ LLM 配置文件 '%s' 可访问",
	HealthFailFmt:    "⚠ LLM 配置文件 '%s' 无法访问",
	HealthSuggestFmt: "  → 下一个选项：'%s'  （在 /config 中切换）",
	HealthNoFallback: "  → 没有其他可访问的配置文件。请检查 /config。",

	FieldActiveProfile: "当前配置文件",
	FieldEndpoint:      "LLM 端点",
	FieldModel:         "模型",
	FieldAPIKey:        "API 密钥",
	FieldMode:          "执行模式",
	FieldPrompt:        "系统提示词",
	FieldLang:          "语言",
	FieldSession:       "保存会话",
	SectionSession:     "── 会话 ─────────────────────────────",
	FieldAutoUpdate:    "自动更新",
	SectionUpdate:      "── 自动更新 ──────────────────────────",

	FieldAPIKeyEmpty:   "（空 – 本地 LLM 不需要）",
	FieldPromptEmpty:   "（空 – 按 Enter 编辑）",
	FieldShortcutEmpty: "（未设置）",
	ModeAuto:           "自动执行",
	ModeAsk:            "询问",
	ModeToggleHint:     "（空格/Enter）",
	ModeSessionOn:      "开",
	ModeSessionOff:     "关",
	ModeUpdateAsk:      "询问",
	ModeUpdateAuto:     "自动",
	ModeUpdateOff:      "关闭",
	BadgeSessionOn:     " SESSION ",
	BadgeSessionOff:    " SESSION ",

	MsgUpdateAvailable:   "🔄 新版本 %s 可用。Alt+U 安装 · Esc 跳过",
	MsgUpdateDownloading: "⬇ 正在下载 bashq %s…",
	MsgUpdateDone:        "✓ bashq %s 已安装 — 请重启",
	MsgUpdateUpToDate:    "✓ bashq 已是最新版本（%s）",
	MsgUpdateError:       "⚠ 更新失败：%s",
	MsgModeAuto:      "⚡ 模式：自动执行（命令无需确认直接运行）",
	MsgModeAsk:       "🛡 模式：询问（执行命令前需要确认）",
	MsgSessionOn:     "💾 会话将被保存（Alt+S 切换）",
	MsgSessionOff:    "🚫 会话不会被保存（Alt+S 切换）",
	MsgShortcutEmpty: "F%d 未设置。请在 /config → 快捷键 中配置。",
	MsgCancelled:     "✗ 已取消",
	MsgAutoExecFmt:   "⚡ 自动: $ %s\n  %s",
	MsgConfirmCmdFmt: "我想执行以下命令：\n\n  $ %s\n\n%s",
	MsgNoOutput:      "（无输出）",
	MsgExitError:     "⚠ 退出错误：",
	MsgToolRejected:  "错误：用户拒绝了此操作。",

	WelcomeMsg:         "\n  bashq 就绪 — 终端的 Q 连续体。\n  请提问或键入 / 查看可用命令。\n",
	LabelUser:          " 你 ",
	LabelAssistant:     " 助手 ",
	SessionRestoredFmt: "📂 已从 %s 恢复会话",

	ActivitiesTitle:   "📋 活动日志",
	ActivitiesEmpty:   "  （本次会话暂无记录）",
	ActivitiesMoreFmt: "  （显示最近 %d 条，共 %d 条）\n\n",
	ActivitiesLogPath: "  完整日志：",
	KindUser:          "用户      ",
	KindAgent:         "助手      ",
	KindExec:          "执行      ",
	KindError:         "错误      ",

	SystemPrompt: `你是一位友善、耐心的 Linux 助手，专门帮助没有 Linux 使用经验的用户。

你的任务：
- 帮助用户管理其 Debian Linux 系统
- 用简单易懂的中文解释所有操作
- 通过 execute_command 工具执行系统命令

重要规则：
- 始终用中文回答
- 执行每条命令前，简洁地解释其作用
- 对危险操作（删除、格式化）必须明确警告
- 回答简短精准，不假设用户具备技术知识
- 如需多条命令，逐一执行
- 以清晰易读的格式展示结果

工具使用：
- 所有系统命令均通过 execute_command 执行
- 在 "explanation" 字段中用简单语言说明命令的作用
- 尽可能合并命令（例如使用 &&）`,

	ToolDesc:       "在 Linux 系统上执行 Shell 命令并返回输出",
	ToolArgCmd:     "要执行的 Shell 命令（bash）",
	ToolArgExpl:    "简单说明此命令的作用（给用户看的）",
	ExtraPromptFmt: "## 附加指令（优先级更高）：\n%s\n\n---\n\n%s",

	CmdInstallDesc:    "安装软件",
	CmdInstallPrompt:  "请安装：",
	CmdRemoveDesc:     "卸载软件",
	CmdRemovePrompt:   "请卸载：",
	CmdUpdateDesc:     "更新系统",
	CmdUpdateMsg:      "请更新我的系统（apt update && apt upgrade）",
	CmdStatusDesc:     "显示系统状态",
	CmdStatusMsg:      "请简要显示系统状态（CPU、内存、磁盘、运行时间）",
	CmdDiskDesc:       "磁盘使用情况",
	CmdDiskMsg:        "我还有多少可用磁盘空间？",
	CmdMemoryDesc:     "显示内存使用情况",
	CmdMemoryMsg:      "请显示当前内存使用情况",
	CmdNetworkDesc:    "网络信息",
	CmdNetworkMsg:     "请显示我的网络配置和 IP 地址",
	CmdServicesDesc:   "管理系统服务",
	CmdServicesPrompt: "服务名称：",
	CmdLogsDesc:       "显示系统日志",
	CmdLogsMsg:        "请显示最近的重要系统日志（最后 20 行）",
	CmdOptimizeDesc:   "系统优化",
	CmdOptimizeMsg:    "我能做什么来优化和加速我的 Debian 系统？",
	CmdConfigDesc:     "编辑设置",
	CmdSetupDesc:      "系统级安装 / 卸载 bashq",
	CmdColorsDesc:     "配置终端颜色到 ~/.bashrc",
	CmdActivitiesDesc: "显示活动日志",
	CmdHelpDesc:       "显示帮助",
	CmdClearDesc:      "清除聊天记录",
	CmdExitDesc:       "退出程序",

	HelpText: `bashq – 帮助

用中文告诉我你需要什么。
我会解释每个步骤，并在危险操作前征求确认。

斜杠命令（键入 / 自动补全）：
  /install    – 安装软件
  /remove     – 卸载软件
  /update     – 更新系统
  /status     – 显示系统状态
  /disk       – 磁盘使用情况
  /memory     – 内存使用情况
  /network    – 网络信息
  /services   – 管理服务
  /logs       – 系统日志
  /optimize   – 优化建议
  /config     – 设置（LLM、系统提示词、快捷键、语言）
  /setup      – 系统级安装 / 卸载 bashq
  /activities – 本次会话的活动日志
  /clear      – 清除聊天
  /exit       – 退出

快捷键：
  Enter       – 发送消息 / 确认选择
  ↑ / ↓      – 导航列表 / 滚动聊天
  F1–F9       – 自定义快捷键（在 /config 中设置）
  Shift+Tab   – 切换执行模式（询问 ↔ 自动）
  Esc         – 关闭自动补全
  Ctrl+C      – 退出`,
}

// supportedLangs definiert die Reihenfolge beim Durchschalten in /config.
var supportedLangs = []string{"de", "en", "zh"}

// setLang setzt die aktive Sprache und aktualisiert den LLM-Tool-Katalog.
func setLang(code string) {
	switch code {
	case "en":
		L = &en
	case "zh":
		L = &zh
	default:
		L = &de
	}
	rebuildTools()
}

// detectSystemLang ermittelt die Systemsprache aus Umgebungsvariablen.
func detectSystemLang() string {
	for _, env := range []string{"LANGUAGE", "LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(env); v != "" {
			lower := strings.ToLower(v)
			switch {
			case strings.HasPrefix(lower, "zh"):
				return "zh"
			case strings.HasPrefix(lower, "en"):
				return "en"
			case strings.HasPrefix(lower, "de"):
				return "de"
			}
		}
	}
	return "de"
}
