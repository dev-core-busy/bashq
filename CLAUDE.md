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
- `Agent.Reset()` leert die History (nur System-Prompt bleibt)

### Verlauf leeren

| Befehl | Wirkung |
|--------|---------|
| `/clear-history` (`actionClearHistory`) | `m.messages`, `agent.Reset()`, `m.activities`, `m.inputHistory` **und** die Dateien `session.json` (`deleteSession()`) + `activities.log` (`clearActivityLog()`) |
| `/clear-llm-context` (`actionClearLLM`) | nur `agent.Reset()` – die Chat-Anzeige bleibt erhalten, das LLM startet ohne Vorwissen |
| `/compact` (`actionCompact`) | ersetzt die LLM-History durch eine vom LLM erzeugte Zusammenfassung |

### Sitzungspersistenz (`session_persist.go`)

`saveSession()` filtert `roleSystem` und `roleOutput` heraus. Bleibt danach nichts übrig, schreibt es nicht – **löscht die Datei aber auch nicht**. Ein `os.Remove()` an dieser Stelle hatte eine intakte Sitzung vernichtet, sobald `m.messages` nur Systemmeldungen enthielt (z.B. nur die Health-Check-Meldung). Das Aufräumen gehört allein zu `deleteSession()`, das `/clear-history` selbst aufruft.

`loadSession() (loadedSession, bool)` ist **synchron** und wird in `newModel()` aufgerufen, nicht als `tea.Cmd` in `Init()`. Asynchron gab es ein Wettrennen mit `cmdHealthCheck`: kam dessen Systemmeldung zuerst, war `m.messages` nicht mehr leer, die geladene Sitzung wurde verworfen – und beim nächsten Beenden überschrieben. `newModel()` setzt `m.messages`, `agent.history` und `m.inputHistory` direkt; das Rendern übernimmt das erste `WindowSizeMsg` (`default`-Zweig ruft `updateViewport()`).

`session_persist_test.go` deckt beide Punkte ab.

### Nachrichtenrollen (`msgRole`)

| Rolle | Rendering in `buildContent()` | persistiert |
|-------|-------------------------------|-------------|
| `roleUser`, `roleAssistant` | `wrapText()` – auf `m.width-4` umgebrochen | ja |
| `roleSystem` | `wrapText()` – Systemmeldungen, Hilfetext, `/activities` | nein |
| `roleOutput` | `indentText()` – **nie** umgebrochen | nein |
| `roleError` | einzeilig mit `✗`-Prefix | ja |

`roleOutput` trägt rohe Befehlsausgaben. Word-Wrap würde dort Spaltenlayouts (`df -h`, `ls -la`, `ps aux`) zerreißen, deshalb wird nur eingerückt. Neue Rollen **hinten** anfügen: `session.json` speichert den numerischen Wert, ein Einschub würde gespeicherte Verläufe umdeuten.

`wrapText()` misst in Terminalspalten (`lipgloss.Width`), nicht in Bytes – sonst bricht Text mit Umlauten oder `–`/`↑↓` zu früh um. Fortsetzungszeilen erben die Einrückung der Originalzeile (hängender Einzug), und `splitWords()` erhält Mehrfach-Leerzeichen, damit Spaltenausrichtung im Hilfetext überlebt.

Passt ein einzelnes Wort in keine Zeile, bricht `breakWord()` es zeichenweise. Das ist kein Randfall: Chinesisch kennt keine Leerzeichen, ein ganzer Satz ist für `splitWords()` ein Wort. Ohne diesen Fallback liefe die Zeile über `m.width` hinaus, das Terminal würde sie umbrechen – und damit im Viewport genau den Zeilenverlust auslösen, den `bottomLines()` unten verhindert.

Dasselbe gilt für Spaltenlayouts: `truncToWidth()`/`truncLeftToWidth()` und `padToWidth()` (`view.go`) kürzen bzw. füllen in Terminalspalten, `fitColumns()` baut daraus eine zweispaltige Zeile, die nie über die verfügbare Breite läuft (erst den Namen bis auf eine Mindestbreite kürzen, dann die zweite Spalte, notfalls weglassen). **Nicht** `fmt.Sprintf("%-*s", …)` zum Ausrichten benutzen – dessen Breite zählt Runen. Bei Umlauten stimmt das zufällig, bei Breitzeichen nicht: `通义` sind zwei Runen, belegen aber vier Spalten. Ebenso wenig byteweise kürzen (`s[:n]`) – das zerschneidet UTF-8. Gemessen wird nach Graphem-Clustern (`graphemes()`, uniseg), nicht runenweise: `❤️` ist U+2764 + U+FE0F, runenweise eine Spalte, tatsächlich zwei; und `👍🏽` darf beim Kürzen nicht vom Hautton getrennt werden.

`cellWidth()` ersetzt `lipgloss.Width()` überall, wo selbst gerechnet wird: Es berücksichtigt `ambiguousWide` (aus `runewidth.EastAsianWidth`, also der Locale bzw. unter Windows der Konsolen-Codepage). In Terminals mit CJK-Locale belegen East-Asian-Ambiguous-Zeichen wie `…`, `▶`, `–`, `─` zwei Spalten; lipgloss zählt sie immer einfach. Den Auswahlpfeil darum über `markerPrefix(sel)` setzen, nicht als `"▶ "`-Literal – unmarkierte Zeilen werden dann gleich weit eingerückt. Bereits gestylte Werte kürzt `ansi.Truncate()` (zerschneidet keine Escape-Sequenzen). Tests setzen `ambiguousWide` per `setAmbiguousWide()`, sonst hinge das Ergebnis von der Locale der Testmaschine ab.

Die Discover-Modellliste leitet ihre sichtbaren Zeilen aus dem tatsächlich gerenderten Kopf ab (`discModelRows()`, auch von `updateModelWindow()` benutzt) statt aus einer Konstante – die Fundzeile wird umgebrochen und kann bei schmalem Terminal zwei Zeilen belegen. `view_test.go` prüft Ausrichtung und Maximalbreite am gerenderten Text (Discover-Liste, Profilliste, Konfig-Labels auf Chinesisch, schmale Terminals).
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

`bottomLines()` zählt die Zeilen des tatsächlich gerenderten `renderBottom()` (`strings.Count(…, "\n")+1`) und kann darum nicht mehr von der Anzeige abweichen. Render-Funktionen müssen ihre Ausgabe aber selbst auf `m.width` umbrechen (`wrapToWidth()` in `view.go`): Ein unkontrollierter Terminal-Umbruch erzeugt eine Bildschirmzeile, die in keinem `\n` auftaucht – die Gesamtausgabe wird zu hoch und Zeilen am unteren Rand werden verschluckt.

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

`bottomLines()` wird aus `renderBottom()` abgeleitet statt pro Zustand hartcodiert – siehe Abschnitt »Layout-Berechnung« weiter oben.

### Autovervollständigung

Wird aktiv sobald die Eingabe mit `/` beginnt. `updateAC()` wird nach jeder Tasteneingabe aufgerufen und filtert `slashCommands` (in `commands.go`). Navigation mit ↑/↓/Tab, Auswahl mit Enter, Schließen mit Esc. Nach Selektion wird `selectCommand()` in `update.go` aufgerufen.

## Daueraufgabe: bashq ↔ winq synchron halten

Schwesterprojekt: `../windows_cmd_agent` (**winq**) – derselbe Agent für Windows.

**Die Sync-Pflicht gilt in BEIDE Richtungen.** Beide CLAUDE.md beschrieben früher nur `bashq → winq`; dadurch blieben dort entstandene Fixes hier liegen (das synchrone Sitzungsladen und die Spaltenausrichtung der Profilliste wurden deshalb erst nachträglich portiert). Wer an einem der beiden Projekte etwas ändert, prüft also immer auch das andere:

- **bashq → winq**: jede Windows-kompatible Änderung dort nachziehen.
- **winq → bashq**: Bugfixes, die in winq zuerst entstanden sind, hierher zurückportieren. Praktischer Einstieg ist ein Dateidiff zwischen beiden Bäumen: `diff -u <datei> ../windows_cmd_agent/<datei>`.

Gemeinsame Dateien (Diff sollte nur Plattformdetails zeigen):
`view.go`, `i18n.go`, `model.go`, `styles.go`, `agent.go`, `commands.go`, `activities.go`, `session_persist.go`, `config_persist.go`, `discovery.go`, `updater.go`

Gemeinsame Tests (byte-identisch in beiden Bäumen):
`session_persist_test.go`, `view_test.go`

`updater_test.go` ist in beiden Bäumen **unterschiedlich** gewachsen (hier `TestCompareVersions` für `compareVersions()`, in winq `TestSemverGT` für `semverGT()` – verschiedene Funktionen). Nicht blind überschreiben.

`session_persist_test.go` lenkt `os.UserConfigDir()` in ein Temp-Verzeichnis um und setzt dazu **`XDG_CONFIG_HOME` und `AppData`**, weil Go je Plattform eine andere Variable liest (`%AppData%` auf Windows). `redirectConfigDir()` prüft danach, dass `sessionFilePath()` wirklich im Temp-Verzeichnis liegt – ohne diese Absicherung würde der Test unter Windows die echte `session.json` beschreiben und am Ende löschen. Die Variable also nicht „aufräumen", nur weil sie auf Linux nichts tut.

Linux-spezifische Dateien (**nicht** aus winq übernehmen):
- `main.go` — nutzt `syscall.Exec`
- `update.go` — führt über `bash -c` aus (winq: `powershell`)
- `selfinstall.go` — Symlinks (winq: Windows-PATH-Eintrag)
- `clipboard.go` — `wl-paste`/`xclip`/`xsel` (in winq steckt das in `model.go`/`terminal_windows.go`)
- `colors.go` — in winq nur ein No-op-Stub

Erwartete Rest-Diffs in den gemeinsamen Dateien: der Konfigverzeichnisname (`bashq` ↔ `winq`), Asset-Namen im Updater, Beispielbefehle in Kommentaren und `cmdInitSize()` (nur winq, umgeht einen Startup-Hänger auf Windows).
