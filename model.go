package main

import (
	"context"
	"strings"

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
	autoUpdate       string // "ask" | "auto" | "off"
}

// --- Tea-Nachrichten ---

type agentResponseMsg struct{ resp *AgentResponse }
type commandResultMsg struct {
	callID    string
	output    string
	err       error
	cancelled bool
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
	ti.CharLimit = 512
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

	return model{
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
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{textinput.Blink}
	if m.cfg.saveSessions {
		cmds = append(cmds, cmdLoadSession())
	}
	if m.cfg.activeProfileIdx >= 0 && m.cfg.activeProfileIdx < len(m.cfg.profiles) {
		p := m.cfg.profiles[m.cfg.activeProfileIdx]
		cmds = append(cmds, cmdHealthCheck(p.BaseURL, p.Name, m.cfg.profiles, m.cfg.activeProfileIdx))
	}
	if m.cfg.autoUpdate != "off" {
		cmds = append(cmds, cmdCheckUpdate(), cmdScheduleUpdateCheck())
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

// inputLineCount gibt zurück, wie viele Zeilen das Eingabefeld bei aktueller Terminalbreite belegt.
func (m model) inputLineCount() int {
	const promptWidth = 3
	avail := m.width - promptWidth
	if avail < 1 {
		avail = 1
	}
	n := len([]rune(m.input.Value()))
	if n == 0 {
		return 1
	}
	return (n + avail - 1) / avail
}

// bottomLines gibt die Anzahl der Zeilen im unteren Bereich zurück.
func (m *model) bottomLines() int {
	switch m.state {
	case stateConfirm:
		return 3
	case stateLoading, stateExecuting:
		return 2
	case stateConfig:
		if m.configEditing {
			return 3 // "Bearbeite <feld>:" + input + hints
		}
		return 2
	case stateEditPrompt:
		return 2 // hints
	case stateDiscover:
		switch m.discStep {
		case discEnterHost:
			if m.discEditProfile >= 0 && m.discErr != "" {
				return 2 // Fehlermeldung im View, nur Esc-Hint unten
			}
			return 3 // label+input, hint, trailing newline
		case discEnterName:
			return 3
		default:
			return 2 // spinner/nav + hint
		}
	}
	// stateIdle
	lines := 1 + m.inputLineCount() // Eingabe (1+) + Hinweiszeile
	if m.showAC && len(m.filtered) > 0 {
		visible := minInt(len(m.filtered), maxACVisible)
		lines += visible
		if len(m.filtered) > maxACVisible {
			lines++
		}
	}
	return lines
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
	const headerLines = 5 // blank + title+blank + found+blank
	vis := m.viewport.Height - headerLines
	if vis < 1 {
		vis = 1
	}
	if m.modelSel < m.discModelOffset {
		m.discModelOffset = m.modelSel
	} else if m.modelSel >= m.discModelOffset+vis {
		m.discModelOffset = m.modelSel - vis + 1
	}
}

// --- Textformatierung ---

func wrapText(text string, width int, indent string) string {
	if width <= len(indent)+4 {
		return indent + text
	}
	var sb strings.Builder
	for i, line := range strings.Split(text, "\n") {
		if i > 0 {
			sb.WriteByte('\n')
		}
		if len(line) <= width-len(indent) {
			sb.WriteString(indent + line)
			continue
		}
		words := strings.Fields(line)
		cur := indent
		for _, word := range words {
			if len(cur)+len(word)+1 > width && cur != indent {
				sb.WriteString(cur + "\n")
				cur = indent + word
			} else if cur == indent {
				cur = indent + word
			} else {
				cur = cur + " " + word
			}
		}
		sb.WriteString(cur)
	}
	return sb.String()
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
	case i == 5: // System-Prompt
		return base + 7
	case i >= 6 && i <= 14: // F1–F9
		return base + 11 + (i - 6)
	}
	return 0
}
