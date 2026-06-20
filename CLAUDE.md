# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Projekt

**bashq** – minimalistischer TUI-Linux-Agent: übersetzt natürliche Sprache direkt im Terminal in präzise Linux-Befehlsketten und führt sie lokal aus. Kommuniziert über eine OpenAI-kompatible API mit einem lokalen LLM. Erzeugt eine statisch gelinkte Binary ohne Installationsvoraussetzungen.

Config: `~/.config/bashq/config.json` · Log: `~/.config/bashq/activities.log`

## Build

```bash
bash build.sh          # statische Binary → ./bashq (empfohlen)
go build -o bashq .    # schneller Entwicklungsbuild
```

Der Produktionsbuild setzt `CGO_ENABLED=0` für vollständige Portabilität.

## Entwicklungsbefehle

```bash
go vet ./...           # Statische Analyse
go build ./...         # Kompilierung prüfen (kein Binary)
```

## Architektur

Die Anwendung ist ein einzelnes `main`-Package mit klarer Aufgabentrennung:

| Datei | Verantwortung |
|-------|--------------|
| `model.go` | Bubbletea-Modell, alle Typen, Hilfsmethoden (Layout, Text, Content-Build) |
| `update.go` | Bubbletea `Update()`-Funktion, Zustandsmaschine, `tea.Cmd`-Fabriken |
| `view.go` | Bubbletea `View()`-Funktion, alle Render-Funktionen |
| `styles.go` | Lipgloss-Styles (zentral, alle anderen Dateien greifen darauf zu) |
| `agent.go` | HTTP-Client für OpenAI-kompatible API, Tool-Calling-Logik, Gesprächsverlauf |
| `commands.go` | Slash-Befehlsdefinitionen, Filterlogik, Hilfetext |
| `activities.go` | Aktivitätsprotokoll: In-Memory-Log + Datei-Append |
| `config_persist.go` | JSON-Persistenz unter `~/.config/linux_cmd_agent/config.json` |

### Zustandsmaschine (`appState`)

```
stateIdle ──(Enter)──► stateLoading ──(LLM Text)──► stateIdle
                             │
                         (LLM Tool Call, autoAllow=false)
                             ▼
                       stateConfirm ──(J/Enter)──► stateExecuting ──► stateLoading
                                    ──(N/Esc)────► stateLoading (Abbruch an LLM senden)

                         (LLM Tool Call, autoAllow=true)
                             ▼
                       stateExecuting (direkt, ohne Bestätigung) ──► stateLoading

stateIdle ──(F1–F9)──► stateLoading (Shortcut-Text als Nachricht senden)

stateIdle ──(/config)──► stateConfig ──(Esc)────────────► stateIdle
                              │
                          (Enter auf Textfeld/Shortcut-Feld)
                              ▼
                         stateConfig, configEditing=true ──(Enter/Esc)──► stateConfig
                              │
                          (Enter auf customPrompt, Index 4)
                              ▼
                         stateEditPrompt ──(Ctrl+S)──► stateConfig (speichern)
                                         ──(Esc)─────► stateConfig (verwerfen)
```

**Shift+Tab** schaltet `cfg.autoAllow` global um (aus jedem Zustand außer stateExecuting).  
Im Titelbalken zeigt ein farbiger Badge den aktuellen Modus: **`[ Auto ]`** (grün) oder **`[ Fragen ]`** (rot).

### LLM-Anbindung (`agent.go`)

- Standardwerte: `defaultBaseURL` und `defaultModel` als Konstanten in `agent.go`
- Zur Laufzeit änderbar über `/config` → werden in `appConfig` (model.go) und direkt in `Agent.baseURL` / `Agent.model` gesetzt
- `Agent.history` hält den vollständigen Gesprächsverlauf inkl. Tool-Ergebnissen
- `Agent.Reset()` leert die History (nur System-Prompt bleibt, ausgelöst durch `/clear`)
- Qwen3-`<think>`-Tags werden in `cleanResponse()` automatisch entfernt
- `Message.Content` ist `interface{}` um JSON `null` (Tool-Call-Antworten) und Strings zu unterstützen

### Tool-Calling-Fluss

1. `cmdSendMessage` / `cmdSendToolResult` → async `tea.Cmd` → gibt `agentResponseMsg` zurück
2. `handleAgentResponse` befüllt `toolQueue` und ruft `processNextTool()` auf
3. `processNextTool()` setzt `pendingTool` → `stateConfirm`
4. Nach Bestätigung: `cmdRunCommand` (async) → `commandResultMsg` → `handleCommandResult`
5. `handleCommandResult` schickt Output an LLM (`cmdSendToolResult`) → Loop weiter

### Layout-Berechnung

`recalcViewport()` berechnet `viewport.Height` so:
```
height = 1(title) + viewport.Height + 1(divider) + bottomLines()
       + 3 Join-Newlines (strings.Join mit "\n" zwischen 4 Abschnitten)
→ viewport.Height = height - 4 - bottomLines()
```

`bottomLines()` variiert je nach Zustand: 2 (Idle), 2+N (Idle+AC), 3 (Confirm), 2 (Loading). Muss mit den tatsächlichen Zeilen in `renderBottom()` übereinstimmen, sonst verschiebt sich das Layout.

### Autovervollständigung

Wird aktiv sobald die Eingabe mit `/` beginnt. `updateAC()` wird nach jeder Tasteneingabe aufgerufen und filtert `slashCommands` (in `commands.go`). Navigation mit ↑/↓/Tab, Auswahl mit Enter, Schließen mit Esc. Nach Selektion wird `selectCommand()` in `update.go` aufgerufen.

### Konfigurationseditor (`stateConfig`)

Öffnet sich über `/config`. 14 Felder in 3 Sektionen:
- **VERBINDUNG** (Index 0–2): LLM-Endpunkt, Modell, API-Key
- **ASSISTENT** (Index 3–4): Ausführmodus (Toggle), System-Prompt (→ `stateEditPrompt`)
- **TASTENKÜRZEL** (Index 5–13): F1–F9

`configFieldCount = 14` in `update.go`. Textfelder nutzen `m.input` mit `configEditing=true`. API-Key wird mit `EchoPassword` maskiert. Der System-Prompt öffnet einen Textarea-Editor (`stateEditPrompt`) statt `m.input`. Shortcuts (Index 5–13) speichern in `cfg.shortcuts[configSel-5]`.

### System-Prompt-Editor (`stateEditPrompt`)

`m.promptEditor` (bubbles `textarea`) zeigt den mehrzeiligen System-Prompt. Ctrl+S speichert in `cfg.customPrompt` und `agent.customPrompt`, schreibt `saveConfig()`, kehrt zu `stateConfig` zurück. Esc verwirft. Die `View()` rendert bei `stateEditPrompt` direkt `m.promptEditor.View()` statt `m.viewport.View()`.

### Tastenkürzel F1–F9

In `handleKey()` (vor dem State-Switch) wird bei `stateIdle` auf `f1`–`f9` geprüft. `triggerShortcut(i)` liest `cfg.shortcuts[i]`, loggt die Aktivität und schickt den Text als Nachricht an den Agent. Leere Shortcuts zeigen eine Hinweismeldung. Belegen über `/config` → TASTENKÜRZEL-Sektion.

### Auto-Allow-Modus

`m.cfg.autoAllow` – wenn `true`, überspringt `processNextTool()` den `stateConfirm`-Schritt und startet den Befehl direkt mit `⚡ Auto:`-Prefix. Umschalten mit **Shift+Tab** von überall. Modus-Badge im Titel zeigt den aktuellen Zustand.

### Aktivitätsprotokoll (`activities.go`)

`logActivity(kind, msg)` (Pointer-Receiver auf `model`) schreibt in `m.activities` und appended an `~/.config/linux_cmd_agent/activities.log`. Aufrufe in:
- `submitInput()` / `triggerShortcut()` → `actUser`
- `handleAgentResponse()` → `actAgent`
- `handleCommandResult()` → `actExec` (Start und Erfolg)
- `errMsg`-Handler → `actError`

`/activities` zeigt die letzten 50 Einträge mit Zeitstempel, Icon und Typ.

### Layout-Berechnung

`recalcViewport()` berechnet `viewport.Height` so:
```
View() = strings.Join([title, viewport/editor, divider, bottom], "\n")
Zeilen = 1 + h + 1 + bottomLines() + 3 Newlines = h + bottomLines() + 2
→ h = m.height - 2 - bottomLines()
```

`bottomLines()` variiert je nach Zustand: 2 (Idle), 2+N (Idle+AC), 3 (Confirm), 2 (Loading/Executing), 2 oder 3 (Config), 2 (EditPrompt). Muss exakt mit `renderBottom()` übereinstimmen.

### Autovervollständigung

Wird aktiv sobald die Eingabe mit `/` beginnt. `updateAC()` wird nach jeder Tasteneingabe aufgerufen und filtert `slashCommands` (in `commands.go`). Navigation mit ↑/↓/Tab, Auswahl mit Enter, Schließen mit Esc. Nach Selektion wird `selectCommand()` in `update.go` aufgerufen.
