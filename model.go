package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

type appState int

const (
	stateIdle        appState = iota
	stateLoading              // warte auf LLM-Antwort
	stateConfirm              // warte auf Benutzerbestätigung
	stateExecuting            // Befehl wird ausgeführt
	stateConfig               // Konfigurationseditor
	stateEditPrompt           // System-Prompt Multi-Line-Editor
	stateDiscover             // LLM-Profil per Portscan hinzufügen
)

// discStepType beschreibt den aktuellen Unterschritt im Discover-Flow.
type discStepType int

const (
	discEnterHost discStepType = iota // IP/Hostname eingeben
	discScanning                      // Portscan läuft
	discPickModel                     // Modell aus Liste wählen
	discEnterName                     // Profilname eingeben
)

// llmProfile beschreibt einen gespeicherten LLM-Endpunkt.
type llmProfile struct {
	Name      string
	BaseURL   string
	Model     string
	APIKey    string
	Preferred bool
}

// appConfig hält alle zur Laufzeit änderbaren Einstellungen
type appConfig struct {
	baseURL          string
	model            string
	apiKey           string
	autoAllow        bool
	customPrompt     string    // Zusatzanweisungen, die dem System-Prompt vorangestellt werden
	shortcuts        [9]string // F1–F9: Nachrichtentext der an den Agent geschickt wird
	lang             string    // "de" | "en" | "zh"
	profiles         []llmProfile
	activeProfileIdx int // -1 = kein Profil aktiv (manuelle Konfiguration)
	saveSessions     bool
	mouseTracking    bool // true = Mausrad scrollt; false = Terminal-Auswahl/Rechtsklick
	autoUpdate       string // "ask" | "auto" | "off"
	llmTimeout       int    // Sekunden, 30–300, Standard 60
}

// --- Tea-Nachrichten ---

type agentResponseMsg struct{ resp *AgentResponse }
type commandResultMsg struct {
	callID    string
	output    string
	err       error
	cancelled bool
}
type compactDoneMsg struct {
	msgCount int
}
type errMsg struct{ err error }
type spinTickMsg struct{}

// --- Modell ---

const maxACVisible = 8

type model struct {
	width, height int
	viewport      viewport.Model
	input         textinput.Model
	promptEditor  textarea.Model // für System-Prompt-Bearbeitung
	messages      []chatMessage
	state         appState

	showAC   bool
	filtered []SlashCommand
	acSel    int

	pendingTool *pendingTool
	toolQueue   []pendingTool
	currentCmd  string

	// Konfiguration
	cfg           appConfig
	configSel     int  // 0=lang,1=autoAllow,2=install,3=autoUpdate,4=saveSessions,5=customPrompt,6-14=F1-F9
	configEditing bool // einzeiliger Textmodus aktiv
	cfgSection    int  // 0=Profile, 1=Einstellungen (in stateConfig)
	profileSel    int  // Auswahl in Profilliste (cfgSection==0)
	profileSubSel int  // -1=Profilzeile, 0=URL, 1=Modell, 2=API-Key (nur cfgSection==0)

	// Profil-Discovery
	discStep        discStepType
	discHost        string
	discModels      []foundModel
	discErr         string
	modelSel        int
	discModelOffset int // Schiebefenster-Start für Modellliste
	tempProfile     llmProfile
	discEditProfile int // >=0: bestehendes Profil bearbeiten (Index), -1: neues Profil

	pendingUpdate *updateInfo // gesetzt wenn Update verfügbar und autoUpdate=="ask"
	nextUpdateAt  time.Time  // Zeitpunkt des nächsten geplanten Update-Checks

	// Aktivitätsprotokoll
	activities []activityEntry

	agent      *Agent
	ready      bool
	spinner    int
	spinFrames []string

	cancelFunc context.CancelFunc // Abbruch der laufenden LLM-Anfrage oder Befehlsausführung

	inputHistory       []string // gesendete Nutzernachrichten, neueste am Ende
	historyIdx         int      // -1 = kein History-Modus; sonst Index in inputHistory
	inputBeforeHistory string   // gespeicherte Eingabe beim Einstieg in den History-Modus

	// Retry bei LLM-Fehler
	canRetry          bool
	retryIsToolResult bool
	retryIsCompact    bool
	retryMsg          string
	retryToolID       string
	retryToolOutput   string
}

func newModel() model {
	// Sprache aus Config laden (vor allem anderen, da L.* überall genutzt wird)
	cfg := loadConfig()
	setLang(cfg.lang)

	ti := textinput.New()
	ti.Placeholder = L.InputPlaceholder
	ti.Prompt = " > "
	ti.PromptStyle = promptStyle
	ti.TextStyle = inputTextStyle
	ti.PlaceholderStyle = placeholderStyle
	ti.CharLimit = 4096
	ti.Focus()

	te := textarea.New()
	te.Placeholder = L.TextareaPlaceholder
	te.ShowLineNumbers = false
	te.CharLimit = 4000
	// Kein Rahmen – passt sich dem Layout an
	noStyle := noStyleDef()
	te.FocusedStyle.Base = noStyle
	te.BlurredStyle.Base = noStyle

	ag := newAgent()
	ag.baseURL = cfg.baseURL
	ag.model = cfg.model
	ag.apiKey = cfg.apiKey
	ag.customPrompt = cfg.customPrompt
	ag.SetTimeout(cfg.llmTimeout)

	m := model{
		input:           ti,
		promptEditor:    te,
		agent:           ag,
		state:           stateIdle,
		cfg:             cfg,
		spinFrames:      []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		historyIdx:      -1,
		profileSubSel:   -1,
		discEditProfile: -1,
	}

	// Gespeicherte Sitzung synchron laden, bevor die Bubbletea-Schleife startet.
	// Als tea.Cmd gab es hier ein Wettrennen mit dem Health-Check: kam dessen
	// Systemmeldung zuerst, war m.messages nicht mehr leer, die Sitzung wurde
	// verworfen — und beim nächsten Beenden überschrieben.
	if cfg.saveSessions {
		if s, ok := loadSession(); ok {
			header := chatMessage{
				role:    roleSystem,
				content: fmt.Sprintf(L.SessionRestoredFmt, s.savedAt),
			}
			m.messages = append([]chatMessage{header}, s.messages...)
			ag.history = s.history
			m.inputHistory = s.inputHistory
			if len(m.inputHistory) == 0 {
				// Alte Sessions ohne input_history: User-Nachrichten rekonstruieren
				for _, cm := range s.messages {
					if cm.role == roleUser && cm.content != "" {
						m.inputHistory = append(m.inputHistory, cm.content)
					}
				}
			}
		}
	}

	return m
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{textinput.Blink}
	// main.go startet mit aktivem Maus-Tracking; war es zuletzt abgeschaltet,
	// hier wieder ausschalten damit Terminal-Auswahl und Rechtsklick greifen.
	if !m.cfg.mouseTracking {
		cmds = append(cmds, tea.DisableMouse)
	}
	// Die Sitzung ist zu diesem Zeitpunkt bereits in newModel() geladen.
	if m.cfg.activeProfileIdx >= 0 && m.cfg.activeProfileIdx < len(m.cfg.profiles) {
		p := m.cfg.profiles[m.cfg.activeProfileIdx]
		if p.Name != "" {
			cmds = append(cmds, cmdHealthCheck(p.BaseURL, p.Name, m.cfg.profiles, m.cfg.activeProfileIdx))
		}
	}
	if m.cfg.autoUpdate != "off" {
		cmds = append(cmds, cmdCheckUpdate()) // Chain: checkUpdate → scheduleUpdate → checkUpdate …
	}
	return tea.Batch(cmds...)
}

// --- Hilfs-Methoden ---

type msgRole int

const (
	roleUser      msgRole = iota
	roleAssistant
	roleSystem
	roleError
	// roleOutput: rohe Befehlsausgabe. Wird NUR eingerückt, nie umgebrochen —
	// Word-Wrap würde Spaltenlayouts (df -h, ls -la, ps aux) zerreißen.
	// Neue Rollen müssen hinten angefügt werden: session.json speichert den
	// numerischen Wert, ein Einschub würde gespeicherte Verläufe umdeuten.
	roleOutput
)

type chatMessage struct {
	role    msgRole
	content string
}

type pendingTool struct {
	id          string
	command     string
	explanation string
}

func (m *model) addMessage(role msgRole, content string) {
	m.messages = append(m.messages, chatMessage{role: role, content: content})
}

func (m *model) updateViewport() {
	m.recalcViewport()
	m.viewport.SetContent(m.buildContent())
	m.viewport.GotoBottom()
}

func (m *model) buildContent() string {
	if len(m.messages) == 0 {
		return dimStyle.Render(L.WelcomeMsg)
	}
	var sb strings.Builder
	sb.WriteString("\n")
	for _, msg := range m.messages {
		switch msg.role {
		case roleUser:
			sb.WriteString(userLabelStyle.Render(L.LabelUser) + "\n")
			sb.WriteString(wrapText(msg.content, m.width-4, "  ") + "\n\n")
		case roleAssistant:
			sb.WriteString(assistantLabelStyle.Render(L.LabelAssistant) + "\n")
			sb.WriteString(wrapText(msg.content, m.width-4, "  ") + "\n\n")
		case roleSystem:
			sb.WriteString(systemMsgStyle.Render(wrapText(msg.content, m.width-4, "  ")) + "\n\n")
		case roleOutput:
			sb.WriteString(systemMsgStyle.Render(indentText(msg.content, "  ")) + "\n\n")
		case roleError:
			sb.WriteString(errorMsgStyle.Render("  ✗ "+msg.content) + "\n\n")
		}
	}
	return sb.String()
}

// recalcViewport berechnet die Viewport-Höhe neu.
// View() = strings.Join([title, viewport, divider, bottom], "\n")
// Zeilenanzahl = 1 + h + 1 + b = h + b + 2  →  h = height - 2 - bottomLines()
func (m *model) recalcViewport() {
	if m.height == 0 {
		return
	}
	h := m.height - 2 - m.bottomLines()
	if h < 1 {
		h = 1
	}
	m.viewport.Height = h
	m.viewport.Width = m.width
}

// bottomLines gibt die Anzahl der Zeilen im unteren Bereich zurück.
// Wird direkt aus renderBottom() abgeleitet, damit die berechnete Höhe nie von der
// tatsächlich gerenderten Zeilenzahl abweicht — auch wenn eine Hinweiszeile bei
// schmalem Terminal durch automatischen Zeilenumbruch mehr Platz braucht als angenommen.
func (m *model) bottomLines() int {
	return strings.Count(m.renderBottom(), "\n") + 1
}

func (m *model) updateAC() {
	val := m.input.Value()
	if strings.HasPrefix(val, "/") {
		query := val[1:]
		m.filtered = filterCommands(query)
		m.showAC = len(m.filtered) > 0
		if m.acSel >= len(m.filtered) {
			m.acSel = 0
		}
	} else {
		m.showAC = false
		m.acSel = 0
	}
	m.recalcViewport()
}

func (m model) spinnerFrame() string {
	return m.spinFrames[m.spinner%len(m.spinFrames)]
}

func (m *model) promptEditorHeight() int {
	h := m.height - 2 - 2 // title(1) + divider(1) + hints(2)
	if h < 3 {
		h = 3
	}
	return h
}

// updateModelWindow passt discModelOffset an, sodass modelSel im sichtbaren Fenster bleibt.
func (m *model) updateModelWindow() {
	vis := m.discModelRows()
	if m.modelSel < m.discModelOffset {
		m.discModelOffset = m.modelSel
	} else if m.modelSel >= m.discModelOffset+vis {
		m.discModelOffset = m.modelSel - vis + 1
	}
}

// --- Textformatierung ---

// wrapText bricht Text auf width Spalten um und rückt jede Zeile mit indent ein.
// Muss eine Zeile umgebrochen werden, erben die Fortsetzungszeilen die führende
// Einrückung der Originalzeile (hängender Einzug), damit eingerückte Strukturen
// wie der Hilfetext ihre Form behalten.
//
// Gemessen wird in Terminalspalten (cellWidth), nicht in Bytes: Umlaute und
// Zeichen wie – oder ↑↓ brauchen mehrere Bytes, eine Byte-Zählung bricht deutschen
// Text darum deutlich zu früh um. CJK-Zeichen zählen korrekt als zwei Spalten.
func wrapText(text string, width int, indent string) string {
	if width <= cellWidth(indent)+4 {
		return indentText(text, indent)
	}
	var sb strings.Builder
	for i, line := range strings.Split(text, "\n") {
		if i > 0 {
			sb.WriteByte('\n')
		}
		if cellWidth(indent+line) <= width {
			sb.WriteString(indent + line)
			continue
		}
		body := strings.TrimLeft(line, " \t")
		pfx := indent + line[:len(line)-len(body)]
		// Bei sehr schmalem Terminal bliebe hinter tiefem Einzug kein Platz für Text.
		if cellWidth(pfx)+8 > width {
			pfx = indent
		}
		cur := pfx
		empty := true
		flush := func() {
			sb.WriteString(cur + "\n")
			cur = pfx
			empty = true
		}
		avail := width - cellWidth(pfx)
		for _, tk := range splitWords(body) {
			// Ein Wort, das selbst keine ganze Zeile füllt, muss zeichenweise
			// gebrochen werden: CJK-Text kennt keine Leerzeichen, und überlange
			// Wörter (URLs, Pfade) würden sonst über den Rand hinauslaufen.
			if cellWidth(tk.word) > avail {
				if !empty {
					flush()
				}
				parts := breakWord(tk.word, avail)
				for i, p := range parts {
					cur += p
					empty = false
					if i < len(parts)-1 {
						flush()
					}
				}
				continue
			}
			switch {
			case empty:
				cur += tk.word
				empty = false
			case cellWidth(cur)+cellWidth(tk.space+tk.word) > width:
				flush()
				cur += tk.word
				empty = false
			default:
				cur += tk.space + tk.word
			}
		}
		sb.WriteString(cur)
	}
	return sb.String()
}

// breakWord teilt ein Wort, das breiter als avail ist, zeichenweise (nach
// Graphem-Clustern) auf. Nötig für Sprachen ohne Wortgrenzen (Chinesisch) und für
// überlange Token wie URLs.
func breakWord(word string, avail int) []string {
	if avail < 1 {
		avail = 1
	}
	var parts []string
	var cur string
	curW := 0
	for _, c := range graphemes(word) {
		if curW+c.w > avail && cur != "" {
			parts = append(parts, cur)
			cur, curW = "", 0
		}
		cur += c.s
		curW += c.w
	}
	if cur != "" {
		parts = append(parts, cur)
	}
	return parts
}

// wordToken ist ein Wort samt des Whitespace, der ihm vorausgeht.
type wordToken struct {
	space string
	word  string
}

// splitWords zerlegt eine Zeile in Wörter und behält den trennenden Whitespace
// bei — anders als strings.Fields, das Mehrfach-Leerzeichen kollabiert und damit
// Spaltenausrichtung (z.B. "/compact       – Beschreibung") zerstören würde.
// Die Byte-Vergleiche sind UTF-8-sicher: ASCII-Bytes treten in Mehrbyte-Sequenzen nicht auf.
func splitWords(s string) []wordToken {
	var toks []wordToken
	for i := 0; i < len(s); {
		j := i
		for j < len(s) && (s[j] == ' ' || s[j] == '\t') {
			j++
		}
		k := j
		for k < len(s) && s[k] != ' ' && s[k] != '\t' {
			k++
		}
		if j == k {
			break // nur noch Whitespace am Zeilenende
		}
		toks = append(toks, wordToken{space: s[i:j], word: s[j:k]})
		i = k
	}
	return toks
}

func indentText(text, indent string) string {
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		lines[i] = indent + l
	}
	return strings.Join(lines, "\n")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// configFieldLine berechnet die 0-basierte Zeilennummer des configSel-Felds i
// im gerendereten Config-Inhalt (cfgSection==1 ohne Sub-Felder). Für Viewport-Auto-Scroll.
func (m model) configFieldLine(i int) int {
	nP := len(m.cfg.profiles)
	// Struktur (cfgSection==1): "\n"(1) + Profil-Header"\n\n"(2) + nP Profile + Add-Button"\n\n"(2) + Einstellungen-Header"\n\n"(2) → base = 7+nP
	base := 7 + nP
	switch {
	case i == 0: // Sprache
		return base + 0
	case i == 1: // Ausführmodus
		return base + 1
	case i == 2: // Kurzbefehl
		return base + 3
	case i == 3: // Auto-Update
		return base + 4
	case i == 4: // Sitzungen
		return base + 5
	case i == 5: // Timeout (neu)
		return base + 6
	case i == 6: // System-Prompt
		return base + 8
	case i >= 7 && i <= 15: // F1–F9
		return base + 12 + (i - 7)
	}
	return 0
}
