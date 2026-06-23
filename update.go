package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// configFieldCount: 0=lang, 1=autoAllow, 2=install, 3=autoUpdate, 4=saveSessions, 5=customPrompt, 6-14=F1-F9
const configFieldCount = 15

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if !m.ready {
			m.viewport = viewport.New(msg.Width, 10)
			m.ready = true
		}
		m.promptEditor.SetWidth(msg.Width)
		m.promptEditor.SetHeight(m.promptEditorHeight())
		m.recalcViewport()
		switch m.state {
		case stateConfig:
			m.viewport.SetContent(m.renderConfigContent())
		case stateEditPrompt:
			// textarea resized above
		default:
			m.updateViewport()
		}
		return m, nil

	case agentResponseMsg:
		return m.handleAgentResponse(msg.resp)

	case commandResultMsg:
		return m.handleCommandResult(msg)

	case errMsg:
		m.state = stateIdle
		if errors.Is(msg.err, context.Canceled) {
			return m, nil // Abbruch durch Ctrl+C — bereits im Handler behandelt
		}
		m.logActivity(actError, msg.err.Error())
		m.addMessage(roleError, msg.err.Error())
		m.updateViewport()
		return m, nil

	case discoveryResultMsg:
		if m.state == stateDiscover {
			if len(msg.models) == 0 {
				if m.discEditProfile >= 0 {
					// Modellsuche aus Profil-Unterfeld: Fehler im Discovery-View anzeigen,
					// Esc führt dann direkt auf das API-Key-Unterfeld
					p := m.cfg.profiles[m.discEditProfile]
					if msg.authFailed && p.APIKey != "" {
						// Key vorhanden, aber Server hat abgelehnt → Key prüfen
						m.discErr = fmt.Sprintf(L.DiscoveryAuthFailed, m.discHost)
					} else if msg.authFailed {
						// Kein Key, Server verlangt Auth → Key eintragen
						m.discErr = fmt.Sprintf(L.DiscoveryNeedsAPIKey, m.discHost)
					} else {
						m.discErr = fmt.Sprintf(L.DiscoveryNoneFmt, m.discHost)
					}
				} else {
					// Manuelle Host-Eingabe
					if msg.authFailed {
						m.discErr = fmt.Sprintf(L.DiscoveryNeedsAPIKey, m.discHost)
					} else {
						m.discErr = fmt.Sprintf(L.DiscoveryNoneFmt, m.discHost)
					}
				}
				m.discStep = discEnterHost
			} else {
				m.discModels = msg.models
				m.discStep = discPickModel
				m.modelSel = 0
				m.discModelOffset = 0
			}
			m.recalcViewport()
			m.viewport.SetContent(m.renderDiscoverContent())
		}
		return m, nil

	case sessionLoadedMsg:
		// Bereits gespeicherte Sitzung laden – nur wenn Chat noch leer ist
		if len(m.messages) == 0 {
			header := chatMessage{
				role:    roleSystem,
				content: fmt.Sprintf(L.SessionRestoredFmt, msg.savedAt),
			}
			m.messages = append([]chatMessage{header}, msg.messages...)
			m.agent.history = msg.history
			if len(msg.inputHistory) > 0 {
				m.inputHistory = msg.inputHistory
			} else {
				// Alte Sessions ohne input_history: User-Nachrichten rekonstruieren
				for _, cm := range msg.messages {
					if cm.role == roleUser && cm.content != "" {
						m.inputHistory = append(m.inputHistory, cm.content)
					}
				}
			}
			m.updateViewport()
		}
		return m, nil

	case updateCheckMsg:
		if msg.err == nil && msg.info != nil {
			switch m.cfg.autoUpdate {
			case "auto":
				m.addMessage(roleSystem, fmt.Sprintf(L.MsgUpdateDownloading, msg.info.version))
				m.updateViewport()
				return m, cmdDownloadUpdate(*msg.info)
			default: // "ask"
				m.pendingUpdate = msg.info
				m.addMessage(roleSystem, fmt.Sprintf(L.MsgUpdateAvailable, msg.info.version))
				m.updateViewport()
			}
		}
		// nächste Prüfung in 30 Minuten einplanen
		if m.cfg.autoUpdate != "off" {
			return m, cmdScheduleUpdateCheck()
		}
		return m, nil

	case updateDoneMsg:
		if msg.err != nil {
			m.addMessage(roleError, fmt.Sprintf(L.MsgUpdateError, msg.err.Error()))
			m.updateViewport()
			return m, nil
		}
		if m.cfg.autoUpdate == "auto" {
			restartAfterUpdate = true
			restartExecPath = msg.execPath
			return m, tea.Quit
		}
		m.addMessage(roleSystem, fmt.Sprintf(L.MsgUpdateDone, msg.version))
		m.updateViewport()
		return m, nil

	case scheduleUpdateCheckMsg:
		if m.cfg.autoUpdate != "off" {
			return m, cmdCheckUpdate()
		}
		return m, nil

	case healthCheckMsg:
		if msg.ok {
			m.addMessage(roleSystem, fmt.Sprintf(L.HealthOkFmt, msg.profileName))
		} else {
			text := fmt.Sprintf(L.HealthFailFmt, msg.profileName)
			if msg.suggested != "" {
				text += "\n" + fmt.Sprintf(L.HealthSuggestFmt, msg.suggested)
			} else {
				text += "\n" + L.HealthNoFallback
			}
			m.addMessage(roleSystem, text)
		}
		m.updateViewport()
		return m, nil

	case spinTickMsg:
		if m.state == stateLoading || m.state == stateExecuting ||
			(m.state == stateDiscover && m.discStep == discScanning) {
			m.spinner++
			return m, tickCmd()
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

// --- Tastenverarbeitung ---

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+q":
		if m.cfg.saveSessions {
			saveSession(m.messages, m.agent.history, m.inputHistory)
		}
		return m, tea.Quit

	case "ctrl+c":
		if m.state == stateLoading || m.state == stateExecuting {
			if m.cancelFunc != nil {
				m.cancelFunc()
				m.cancelFunc = nil
			}
			m.agent.PopLastMessage()
			m.state = stateIdle
			m.addMessage(roleSystem, L.MsgCancelled)
			m.updateViewport()
			return m, nil
		}

	// Shift+Tab: Ausführmodus global umschalten
	case "shift+tab":
		m.cfg.autoAllow = !m.cfg.autoAllow
		saveConfig(m.cfg)
		if m.state == stateConfig {
			m.viewport.SetContent(m.renderConfigContent())
		} else if m.state != stateEditPrompt {
			if m.cfg.autoAllow {
				m.addMessage(roleSystem, L.MsgModeAuto)
			} else {
				m.addMessage(roleSystem, L.MsgModeAsk)
			}
			m.updateViewport()
		}
		return m, nil

	// Alt+U: Pendentes Update installieren
	case "alt+u":
		if m.pendingUpdate != nil && m.state != stateExecuting {
			pu := m.pendingUpdate
			m.pendingUpdate = nil
			m.addMessage(roleSystem, fmt.Sprintf(L.MsgUpdateDownloading, pu.version))
			m.updateViewport()
			return m, cmdDownloadUpdate(*pu)
		}
		return m, nil

	// Alt+S: Sitzungs-Speichern global umschalten
	case "alt+s":
		if m.state != stateExecuting {
			m.cfg.saveSessions = !m.cfg.saveSessions
			saveConfig(m.cfg)
			if m.state == stateConfig {
				m.viewport.SetContent(m.renderConfigContent())
			} else if m.state != stateEditPrompt {
				if m.cfg.saveSessions {
					m.addMessage(roleSystem, L.MsgSessionOn)
				} else {
					m.addMessage(roleSystem, L.MsgSessionOff)
				}
				m.updateViewport()
			}
		}
		return m, nil
	}

	// F1–F9 oder Alt+1–9: Shortcuts auslösen (nur in stateIdle)
	if m.state == stateIdle {
		for i := 1; i <= 9; i++ {
			key := msg.String()
			if key == fmt.Sprintf("f%d", i) || key == fmt.Sprintf("alt+%d", i) {
				return m.triggerShortcut(i - 1)
			}
		}
	}

	switch m.state {
	case stateLoading, stateExecuting:
		return m, nil
	case stateConfirm:
		return m.handleConfirmKey(msg)
	case stateConfig:
		return m.handleConfigKey(msg)
	case stateEditPrompt:
		return m.handleEditPromptKey(msg)
	case stateDiscover:
		return m.handleDiscoverKey(msg)
	default:
		return m.handleIdleKey(msg)
	}
}

func (m model) triggerShortcut(idx int) (model, tea.Cmd) {
	if idx < 0 || idx >= 9 {
		return m, nil
	}
	msg := m.cfg.shortcuts[idx]
	if msg == "" {
		m.addMessage(roleSystem, fmt.Sprintf(L.MsgShortcutEmpty, idx+1))
		m.updateViewport()
		return m, nil
	}
	m.addMessage(roleUser, fmt.Sprintf("F%d: %s", idx+1, msg))
	m.updateViewport()
	m.state = stateLoading
	m.logActivity(actUser, fmt.Sprintf("[F%d] %s", idx+1, msg))
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelFunc = cancel
	return m, tea.Batch(cmdSendMessage(ctx, m.agent, msg), tickCmd())
}

func (m model) handleConfirmKey(msg tea.KeyMsg) (model, tea.Cmd) {
	if m.pendingTool == nil {
		m.state = stateIdle
		return m, nil
	}
	switch strings.ToLower(msg.String()) {
	case "j", "y", "enter":
		tool := m.pendingTool
		m.pendingTool = nil
		m.state = stateExecuting
		m.currentCmd = tool.command
		m.logActivity(actExec, tool.command)
		m.addMessage(roleSystem, "$ "+tool.command)
		m.updateViewport()
		ctx, cancel := context.WithCancel(context.Background())
		m.cancelFunc = cancel
		return m, tea.Batch(cmdRunCommand(ctx, tool.id, tool.command), tickCmd())
	case "n", "esc":
		tool := m.pendingTool
		m.pendingTool = nil
		m.toolQueue = nil
		m.state = stateLoading
		m.addMessage(roleSystem, L.MsgCancelled)
		m.updateViewport()
		ctx, cancel := context.WithCancel(context.Background())
		m.cancelFunc = cancel
		return m, tea.Batch(
			cmdSendToolResult(ctx, m.agent, tool.id, L.MsgToolRejected),
			tickCmd(),
		)
	}
	return m, nil
}

func (m model) handleIdleKey(msg tea.KeyMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if m.pendingUpdate != nil {
			m.pendingUpdate = nil
		}
		if m.showAC {
			m.showAC = false
			m.acSel = 0
			m.recalcViewport()
		}
		return m, nil

	case "up":
		if m.showAC && len(m.filtered) > 0 {
			if m.acSel > 0 {
				m.acSel--
			} else {
				m.acSel = len(m.filtered) - 1
			}
			return m, nil
		}
		m.viewport.LineUp(3)
		return m, nil

	case "down":
		if m.showAC && len(m.filtered) > 0 {
			m.acSel = (m.acSel + 1) % len(m.filtered)
			return m, nil
		}
		m.viewport.LineDown(3)
		return m, nil

	case "shift+up", "alt+up":
		if len(m.inputHistory) == 0 {
			return m, nil
		}
		if m.historyIdx == -1 {
			m.inputBeforeHistory = m.input.Value()
			m.historyIdx = len(m.inputHistory) - 1
		} else if m.historyIdx > 0 {
			m.historyIdx--
		}
		m.input.SetValue(m.inputHistory[m.historyIdx])
		m.input.CursorEnd()
		return m, nil

	case "shift+down", "alt+down":
		if m.historyIdx == -1 {
			return m, nil
		}
		if m.historyIdx < len(m.inputHistory)-1 {
			m.historyIdx++
			m.input.SetValue(m.inputHistory[m.historyIdx])
		} else {
			m.historyIdx = -1
			m.input.SetValue(m.inputBeforeHistory)
		}
		m.input.CursorEnd()
		return m, nil

	case "tab":
		if m.showAC && len(m.filtered) > 0 {
			m.acSel = (m.acSel + 1) % len(m.filtered)
			return m, nil
		}
		return m, nil

	case "enter":
		if m.showAC && len(m.filtered) > 0 {
			return m.selectCommand(m.filtered[m.acSel])
		}
		return m.submitInput()

	case "pgup":
		m.viewport.HalfViewUp()
		return m, nil

	case "pgdown":
		m.viewport.HalfViewDown()
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.updateAC()
	m.recalcViewport()
	return m, cmd
}

// --- Config-Modus ---

func (m model) handleConfigKey(msg tea.KeyMsg) (model, tea.Cmd) {
	if m.configEditing {
		return m.handleConfigEditKey(msg)
	}

	switch msg.String() {
	case "esc":
		saveConfig(m.cfg)
		m.state = stateIdle
		m.configSel = 0
		m.cfgSection = 0
		m.profileSel = 0
		m.profileSubSel = -1
		m.input.Placeholder = L.InputPlaceholder
		m.recalcViewport()
		m.updateViewport()

	case "tab":
		m.cfgSection = 1 - m.cfgSection
		m.profileSubSel = -1
		n := len(m.cfg.profiles)
		if m.cfgSection == 0 && m.profileSel > n {
			m.profileSel = n
		}
		m.viewport.SetContent(m.renderConfigContent())
		if m.cfgSection == 0 {
			m.viewport.GotoTop()
		} else {
			m.configSel = 0
			m = m.scrollToConfigField()
		}

	case "up":
		if m.cfgSection == 0 {
			if m.profileSubSel >= 0 {
				m.profileSubSel-- // 0→-1, 1→0, 2→1
			} else {
				// auf Profilzeile
				if m.profileSel > 0 {
					m.profileSel--
					if m.profileSel < len(m.cfg.profiles) {
						m.profileSubSel = 3 // letztes Unterfeld (Name) des vorherigen Profils
					}
				}
			}
		} else {
			if m.configSel > 0 {
				m.configSel--
			} else {
				m.configSel = configFieldCount - 1
			}
		}
		m.viewport.SetContent(m.renderConfigContent())
		m = m.scrollToConfigField()

	case "down":
		if m.cfgSection == 0 {
			if m.profileSubSel == -1 {
				if m.profileSel < len(m.cfg.profiles) {
					m.profileSubSel = 0 // in URL-Unterfeld wechseln
				}
				// Add-Button: keine Unterfelder, nichts tun
			} else if m.profileSubSel < 3 {
				m.profileSubSel++
			} else {
				// profileSubSel==3 (Name): zum nächsten Profil
				m.profileSel++
				m.profileSubSel = -1
			}
		} else {
			m.configSel = (m.configSel + 1) % configFieldCount
		}
		m.viewport.SetContent(m.renderConfigContent())
		m = m.scrollToConfigField()

	case "enter":
		if m.cfgSection == 0 {
			if m.profileSubSel >= 0 {
				return m.activateProfileSubField()
			}
			return m.activateProfileEntry()
		}
		return m.activateConfigField()

	case " ":
		if m.cfgSection == 0 {
			if m.profileSubSel == 1 && m.profileSel < len(m.cfg.profiles) {
				// Modell-Discovery für dieses Profil (mit API-Key falls vorhanden)
				p := m.cfg.profiles[m.profileSel]
				baseURL := p.BaseURL
				if baseURL == "" {
					baseURL = m.cfg.baseURL
				}
				if baseURL != "" {
					m.discEditProfile = m.profileSel
					m.discHost = baseURL
					m.discErr = ""
					m.discStep = discScanning
					m.state = stateDiscover
					m.recalcViewport()
					m.viewport.SetContent(m.renderDiscoverContent())
					return m, tea.Batch(cmdDiscoverModels(baseURL, p.APIKey), tickCmd())
				}
			}
		} else if m.cfgSection == 1 {
			switch m.configSel {
			case 0:
				return m.cycleLang()
			case 1:
				m.cfg.autoAllow = !m.cfg.autoAllow
				m.viewport.SetContent(m.renderConfigContent())
			case 2:
				toggleMsg, isErr := selfInstallToggle()
				if isErr {
					m.addMessage(roleError, toggleMsg)
				} else {
					m.addMessage(roleSystem, toggleMsg)
				}
				m.viewport.SetContent(m.renderConfigContent())
			case 3:
				m.cfg.autoUpdate = cycleAutoUpdate(m.cfg.autoUpdate)
				saveConfig(m.cfg)
				m.viewport.SetContent(m.renderConfigContent())
			case 4:
				m.cfg.saveSessions = !m.cfg.saveSessions
				saveConfig(m.cfg)
				m.viewport.SetContent(m.renderConfigContent())
			}
		}

	case "p", "P":
		if m.cfgSection == 0 && m.profileSubSel == -1 && m.profileSel < len(m.cfg.profiles) {
			return m.setPreferredProfile()
		}

	case "d", "D":
		if m.cfgSection == 0 && m.profileSubSel == -1 && m.profileSel < len(m.cfg.profiles) {
			return m.deleteProfile()
		}
	}

	return m, nil
}

func (m model) handleConfigEditKey(msg tea.KeyMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		value := strings.TrimSpace(m.input.Value())
		if m.cfgSection == 0 {
			return m.saveProfileSubField(value)
		}
		// cfgSection==1: nur Tastenkürzel sind Text-Felder
		if m.configSel >= 6 && m.configSel <= 14 {
			m.cfg.shortcuts[m.configSel-6] = value
		}
		m.configEditing = false
		m.input.EchoMode = textinput.EchoNormal
		m.input.SetValue("")
		m.input.Placeholder = L.InputPlaceholder
		m.recalcViewport()
		m.viewport.SetContent(m.renderConfigContent())
		return m, nil

	case "esc":
		m.configEditing = false
		m.input.EchoMode = textinput.EchoNormal
		m.input.SetValue("")
		m.input.Placeholder = L.InputPlaceholder
		m.recalcViewport()
		m.viewport.SetContent(m.renderConfigContent())
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m model) activateConfigField() (model, tea.Cmd) {
	switch m.configSel {
	case 0: // Sprache – Toggle
		return m.cycleLang()

	case 1: // Ausführmodus – Toggle
		m.cfg.autoAllow = !m.cfg.autoAllow
		m.viewport.SetContent(m.renderConfigContent())

	case 2: // Kurzbefehl (selfInstall toggle)
		toggleMsg, isErr := selfInstallToggle()
		if isErr {
			m.addMessage(roleError, toggleMsg)
		} else {
			m.addMessage(roleSystem, toggleMsg)
		}
		m.viewport.SetContent(m.renderConfigContent())

	case 3: // Auto-Update – Cycle
		m.cfg.autoUpdate = cycleAutoUpdate(m.cfg.autoUpdate)
		saveConfig(m.cfg)
		m.viewport.SetContent(m.renderConfigContent())

	case 4: // Sitzungen – Toggle
		m.cfg.saveSessions = !m.cfg.saveSessions
		saveConfig(m.cfg)
		m.viewport.SetContent(m.renderConfigContent())

	case 5: // System-Prompt → Textarea-Editor öffnen
		m.promptEditor.SetWidth(m.width)
		m.promptEditor.SetHeight(m.promptEditorHeight())
		m.promptEditor.SetValue(m.cfg.customPrompt)
		m.promptEditor.Focus()
		m.state = stateEditPrompt

	default:
		// Einzeiliges Text-Feld (nur Tastenkürzel F1–F9)
		var value string
		if m.configSel >= 6 && m.configSel <= 14 {
			value = m.cfg.shortcuts[m.configSel-6]
		}
		m.input.SetValue(value)
		m.input.Placeholder = ""
		m.input.CursorEnd()
		m.configEditing = true
		m.recalcViewport()
		m.viewport.SetContent(m.renderConfigContent())
	}
	return m, nil
}

// cycleLang schaltet zur nächsten Sprache in supportedLangs.
func (m model) cycleLang() (model, tea.Cmd) {
	next := "de"
	for i, code := range supportedLangs {
		if code == m.cfg.lang {
			next = supportedLangs[(i+1)%len(supportedLangs)]
			break
		}
	}
	m.cfg.lang = next
	setLang(m.cfg.lang)
	// Platzhalter und Textarea-Platzhalter aktualisieren
	m.input.Placeholder = L.InputPlaceholder
	m.promptEditor.Placeholder = L.TextareaPlaceholder
	saveConfig(m.cfg)
	m.viewport.SetContent(m.renderConfigContent())
	return m, nil
}

// --- Profil-Verwaltung ---

func (m model) activateProfileEntry() (model, tea.Cmd) {
	n := len(m.cfg.profiles)
	if m.profileSel == n {
		// "Add"-Button: Discovery starten
		m.discStep = discEnterHost
		m.discHost = ""
		m.discModels = nil
		m.discErr = ""
		m.modelSel = 0
		m.discModelOffset = 0
		m.tempProfile = llmProfile{}
		m.input.SetValue("")
		m.input.Placeholder = L.DiscoveryInputLabel + "…"
		m.input.Focus()
		m.state = stateDiscover
		m.recalcViewport()
		m.viewport.SetContent(m.renderDiscoverContent())
		return m, nil
	}
	// Profil aktivieren
	p := m.cfg.profiles[m.profileSel]
	m.cfg.baseURL = p.BaseURL
	m.cfg.model = p.Model
	m.cfg.apiKey = p.APIKey
	m.cfg.activeProfileIdx = m.profileSel
	m.agent.baseURL = p.BaseURL
	m.agent.model = p.Model
	m.agent.apiKey = p.APIKey
	saveConfig(m.cfg)
	m.viewport.SetContent(m.renderConfigContent())
	return m, nil
}

// activateProfileSubField öffnet das ausgewählte Unterfeld (URL/Modell/API-Key) eines Profils zur Bearbeitung.
func (m model) activateProfileSubField() (model, tea.Cmd) {
	if m.profileSel >= len(m.cfg.profiles) {
		return m, nil
	}
	p := m.cfg.profiles[m.profileSel]

	switch m.profileSubSel {
	case 0: // URL
		m.input.SetValue(p.BaseURL)
	case 1: // Modell
		m.input.SetValue(p.Model)
	case 2: // API-Key
		m.input.SetValue(p.APIKey)
		m.input.EchoMode = textinput.EchoPassword
		m.input.EchoCharacter = '•'
	case 3: // Name
		m.input.SetValue(p.Name)
	}
	m.input.Placeholder = ""
	m.input.CursorEnd()
	m.configEditing = true
	m.recalcViewport()
	m.viewport.SetContent(m.renderConfigContent())
	return m, nil
}

// saveProfileSubField speichert den bearbeiteten Wert des Profil-Unterfelds.
// URL: falls IP/Hostname (kein http/https) → Autodiscover starten.
func (m model) saveProfileSubField(value string) (model, tea.Cmd) {
	m.configEditing = false
	m.input.EchoMode = textinput.EchoNormal
	m.input.SetValue("")
	m.input.Placeholder = L.InputPlaceholder

	if m.profileSel >= len(m.cfg.profiles) {
		return m, nil
	}

	switch m.profileSubSel {
	case 0: // URL
		if value != "" && !strings.HasPrefix(strings.ToLower(value), "http") {
			// IP oder Hostname → Autodiscover starten
			m.discEditProfile = m.profileSel
			m.discHost = value
			m.discErr = ""
			m.discStep = discScanning
			m.state = stateDiscover
			m.recalcViewport()
			m.viewport.SetContent(m.renderDiscoverContent())
			return m, tea.Batch(cmdDiscover(value), tickCmd())
		}
		if value == "" {
			value = defaultBaseURL
		}
		m.cfg.profiles[m.profileSel].BaseURL = value
		if m.cfg.activeProfileIdx == m.profileSel {
			m.cfg.baseURL = value
			m.agent.baseURL = value
		}

	case 1: // Modell
		if value == "" {
			value = defaultModel
		}
		m.cfg.profiles[m.profileSel].Model = value
		if m.cfg.activeProfileIdx == m.profileSel {
			m.cfg.model = value
			m.agent.model = value
		}

	case 2: // API-Key (darf leer sein)
		m.cfg.profiles[m.profileSel].APIKey = value
		if m.cfg.activeProfileIdx == m.profileSel {
			m.cfg.apiKey = value
			m.agent.apiKey = value
		}
	case 3: // Name
		if value != "" {
			m.cfg.profiles[m.profileSel].Name = value
		}
	}

	saveConfig(m.cfg)
	m.recalcViewport()
	m.viewport.SetContent(m.renderConfigContent())
	return m, nil
}

func (m model) setPreferredProfile() (model, tea.Cmd) {
	for i := range m.cfg.profiles {
		m.cfg.profiles[i].Preferred = (i == m.profileSel)
	}
	saveConfig(m.cfg)
	m.viewport.SetContent(m.renderConfigContent())
	return m, nil
}

func (m model) deleteProfile() (model, tea.Cmd) {
	idx := m.profileSel
	m.cfg.profiles = append(m.cfg.profiles[:idx], m.cfg.profiles[idx+1:]...)
	if m.cfg.activeProfileIdx == idx {
		m.cfg.activeProfileIdx = -1
	} else if m.cfg.activeProfileIdx > idx {
		m.cfg.activeProfileIdx--
	}
	n := len(m.cfg.profiles)
	if m.profileSel > n {
		m.profileSel = n
	}
	saveConfig(m.cfg)
	m.viewport.SetContent(m.renderConfigContent())
	return m, nil
}

// --- Discovery-Handler ---

func (m model) handleDiscoverKey(msg tea.KeyMsg) (model, tea.Cmd) {
	switch m.discStep {
	case discEnterHost:
		switch msg.String() {
		case "esc":
			if m.discEditProfile >= 0 {
				// Zurück zur Config, Cursor auf API-Key-Unterfeld positionieren
				m.profileSel = m.discEditProfile
				m.profileSubSel = 2
			}
			return m.returnToConfig()
		case "enter":
			host := strings.TrimSpace(m.input.Value())
			if host == "" {
				return m, nil
			}
			m.discHost = host
			m.discErr = ""
			m.input.SetValue("")
			m.input.Placeholder = L.InputPlaceholder

			// Vollständige URL (http/https): kein Scan nötig, direkt weiter
			if strings.HasPrefix(strings.ToLower(host), "http") {
				m.tempProfile.BaseURL = host
				if m.discEditProfile >= 0 {
					// Bestehendes Profil aktualisieren
					idx := m.discEditProfile
					m.cfg.profiles[idx].BaseURL = host
					if m.cfg.activeProfileIdx == idx {
						m.cfg.baseURL = host
						m.agent.baseURL = host
					}
					saveConfig(m.cfg)
					m.profileSel = idx
					m.profileSubSel = -1
					return m.returnToConfig()
				}
				m.discStep = discEnterName
				m.input.Placeholder = L.DiscoveryNamePlaceholder
				m.input.Focus()
				m.recalcViewport()
				m.viewport.SetContent(m.renderDiscoverContent())
				return m, nil
			}

			m.discStep = discScanning
			m.recalcViewport()
			m.viewport.SetContent(m.renderDiscoverContent())
			return m, tea.Batch(cmdDiscover(host), tickCmd())
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd

	case discScanning:
		if msg.String() == "esc" {
			return m.returnToConfig()
		}
		return m, nil

	case discPickModel:
		switch msg.String() {
		case "esc":
			return m.returnToConfig()
		case "up":
			if m.modelSel > 0 {
				m.modelSel--
				m.updateModelWindow()
				m.viewport.SetContent(m.renderDiscoverContent())
			}
		case "down":
			if m.modelSel < len(m.discModels)-1 {
				m.modelSel++
				m.updateModelWindow()
				m.viewport.SetContent(m.renderDiscoverContent())
			}
		case "enter":
			if len(m.discModels) == 0 {
				return m, nil
			}
			sel := m.discModels[m.modelSel]
			m.tempProfile.BaseURL = sel.BaseURL
			m.tempProfile.Model = sel.Name

			if m.discEditProfile >= 0 {
				// Bestehendes Profil aktualisieren
				idx := m.discEditProfile
				m.cfg.profiles[idx].BaseURL = sel.BaseURL
				m.cfg.profiles[idx].Model = sel.Name
				if m.cfg.activeProfileIdx == idx {
					m.cfg.baseURL = sel.BaseURL
					m.cfg.model = sel.Name
					m.agent.baseURL = sel.BaseURL
					m.agent.model = sel.Name
				}
				saveConfig(m.cfg)
				m.profileSel = idx
				m.profileSubSel = -1
				return m.returnToConfig()
			}

			m.discStep = discEnterName
			m.input.SetValue("")
			m.input.Placeholder = L.DiscoveryNamePlaceholder
			m.input.Focus()
			m.recalcViewport()
			m.viewport.SetContent(m.renderDiscoverContent())
		}
		return m, nil

	case discEnterName:
		switch msg.String() {
		case "esc":
			m.discStep = discPickModel
			m.recalcViewport()
			m.viewport.SetContent(m.renderDiscoverContent())
			return m, nil
		case "enter":
			name := strings.TrimSpace(m.input.Value())
			if name == "" {
				name = m.tempProfile.Model
			}
			if name == "" {
				name = m.tempProfile.BaseURL
			}
			m.tempProfile.Name = name
			m.cfg.profiles = append(m.cfg.profiles, m.tempProfile)
			newIdx := len(m.cfg.profiles) - 1
			if newIdx == 0 {
				m.cfg.profiles[0].Preferred = true
			}
			// Neues Profil direkt aktivieren
			m.cfg.activeProfileIdx = newIdx
			m.cfg.baseURL = m.tempProfile.BaseURL
			m.cfg.model = m.tempProfile.Model
			m.cfg.apiKey = m.tempProfile.APIKey
			m.agent.baseURL = m.tempProfile.BaseURL
			m.agent.model = m.tempProfile.Model
			m.agent.apiKey = m.tempProfile.APIKey
			saveConfig(m.cfg)
			m.input.SetValue("")
			m.input.Placeholder = L.InputPlaceholder
			m.cfgSection = 0
			m.profileSel = newIdx
			savedName := name
			m, cmd := m.returnToConfig() // setzt Viewport auf Config-Inhalt
			m.addMessage(roleSystem, fmt.Sprintf(L.DiscoverySavedFmt, savedName))
			// kein updateViewport – sonst überschreibt Chat den Config-Viewport
			return m, cmd
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m model) returnToConfig() (model, tea.Cmd) {
	m.discEditProfile = -1
	m.state = stateConfig
	m.recalcViewport()
	m.viewport.SetContent(m.renderConfigContent())
	return m, nil
}

// configFieldLabel gibt den Anzeigenamen des aktuell bearbeiteten Feldes zurück.
func (m model) configFieldLabel() string {
	if m.cfgSection == 0 {
		switch m.profileSubSel {
		case 0:
			return L.FieldEndpoint
		case 1:
			return L.FieldModel
		case 2:
			return L.FieldAPIKey
		case 3:
			return L.FieldProfileName
		}
		return ""
	}
	if m.configSel >= 6 && m.configSel <= 14 {
		return fmt.Sprintf("F%d", m.configSel-5)
	}
	return ""
}

// --- System-Prompt Editor ---

func (m model) handleEditPromptKey(msg tea.KeyMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+s":
		m.cfg.customPrompt = m.promptEditor.Value()
		m.agent.customPrompt = m.cfg.customPrompt
		saveConfig(m.cfg)
		m.promptEditor.Blur()
		m.state = stateConfig
		m.recalcViewport()
		m.viewport.SetContent(m.renderConfigContent())
		return m, nil

	case "esc":
		m.promptEditor.Blur()
		m.state = stateConfig
		m.recalcViewport()
		m.viewport.SetContent(m.renderConfigContent())
		return m, nil
	}

	var cmd tea.Cmd
	m.promptEditor, cmd = m.promptEditor.Update(msg)
	return m, cmd
}

// --- Chat-Aktionen ---

func (m model) submitInput() (model, tea.Cmd) {
	text := strings.TrimSpace(m.input.Value())
	if text == "" {
		return m, nil
	}

	// Slash-Befehl direkt ausführen (z.B. aus History gewählt, ohne AC)
	if strings.HasPrefix(text, "/") {
		for _, cmd := range getCommands() {
			if cmd.Name == text {
				return m.selectCommand(cmd)
			}
		}
	}

	// History: doppelte aufeinanderfolgende Einträge vermeiden
	if len(m.inputHistory) == 0 || m.inputHistory[len(m.inputHistory)-1] != text {
		m.inputHistory = append(m.inputHistory, text)
	}
	m.historyIdx = -1
	m.inputBeforeHistory = ""
	m.input.SetValue("")
	m.showAC = false
	m.acSel = 0
	m.recalcViewport()
	m.addMessage(roleUser, text)
	m.logActivity(actUser, text)
	m.updateViewport()
	m.state = stateLoading
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelFunc = cancel
	return m, tea.Batch(cmdSendMessage(ctx, m.agent, text), tickCmd())
}

func (m model) selectCommand(cmd SlashCommand) (model, tea.Cmd) {
	m.showAC = false
	m.acSel = 0
	m.input.SetValue("")
	m.recalcViewport()

	// Slash-Befehl in History aufnehmen (außer /exit)
	if cmd.Action != actionExit {
		if len(m.inputHistory) == 0 || m.inputHistory[len(m.inputHistory)-1] != cmd.Name {
			m.inputHistory = append(m.inputHistory, cmd.Name)
		}
		m.historyIdx = -1
		m.inputBeforeHistory = ""
	}

	switch cmd.Action {
	case actionRun:
		m.addMessage(roleUser, cmd.Name)
		m.logActivity(actUser, cmd.Name)
		m.updateViewport()
		m.state = stateLoading
		ctx, cancel := context.WithCancel(context.Background())
		m.cancelFunc = cancel
		return m, tea.Batch(cmdSendMessage(ctx, m.agent, cmd.Message), tickCmd())
	case actionPrompt:
		m.input.SetValue(cmd.Prompt)
		m.input.CursorEnd()
	case actionClear:
		m.messages = nil
		m.agent.Reset()
		deleteSession()
		m.updateViewport()
	case actionExit:
		if m.cfg.saveSessions {
			saveSession(m.messages, m.agent.history, m.inputHistory)
		}
		return m, tea.Quit
	case actionHelp:
		m.addMessage(roleSystem, L.HelpText)
		m.updateViewport()
	case actionConfig:
		m.state = stateConfig
		m.cfgSection = 1
		m.configSel = 0
		m.configEditing = false
		m.recalcViewport()
		m.viewport.SetContent(m.renderConfigContent())
		m = m.scrollToConfigField()
	case actionActivities:
		m.addMessage(roleSystem, m.formatActivities())
		m.updateViewport()
	case actionSelfInstall:
		msg, isErr := selfInstallToggle()
		if isErr {
			m.addMessage(roleError, msg)
		} else {
			m.addMessage(roleSystem, msg)
		}
		m.updateViewport()
	case actionColors:
		msg, isErr := setupColors()
		if isErr {
			m.addMessage(roleError, msg)
		} else {
			m.addMessage(roleSystem, msg)
		}
		m.updateViewport()
	}
	return m, nil
}

// --- Antwort-Verarbeitung ---

func (m model) handleAgentResponse(resp *AgentResponse) (model, tea.Cmd) {
	if resp.Text != "" {
		m.logActivity(actAgent, resp.Text)
		m.addMessage(roleAssistant, resp.Text)
		m.updateViewport()
	}
	if len(resp.ToolCalls) > 0 {
		m.toolQueue = make([]pendingTool, len(resp.ToolCalls))
		for i, tc := range resp.ToolCalls {
			m.toolQueue[i] = pendingTool{
				id:          tc.ID,
				command:     tc.Command,
				explanation: tc.Explanation,
			}
		}
		return m.processNextTool()
	}
	m.state = stateIdle
	if m.cfg.saveSessions {
		saveSession(m.messages, m.agent.history, m.inputHistory)
	}
	return m, nil
}

func (m model) processNextTool() (model, tea.Cmd) {
	if len(m.toolQueue) == 0 {
		m.state = stateIdle
		return m, nil
	}
	tool := m.toolQueue[0]
	m.toolQueue = m.toolQueue[1:]

	if m.cfg.autoAllow {
		m.state = stateExecuting
		m.currentCmd = tool.command
		m.logActivity(actExec, tool.command)
		m.addMessage(roleSystem, fmt.Sprintf(L.MsgAutoExecFmt, tool.command, tool.explanation))
		m.updateViewport()
		ctx, cancel := context.WithCancel(context.Background())
		m.cancelFunc = cancel
		return m, tea.Batch(cmdRunCommand(ctx, tool.id, tool.command), tickCmd())
	}

	m.pendingTool = &tool
	m.state = stateConfirm
	m.addMessage(roleAssistant, fmt.Sprintf(L.MsgConfirmCmdFmt, tool.command, tool.explanation))
	m.updateViewport()
	return m, nil
}

func (m model) handleCommandResult(msg commandResultMsg) (model, tea.Cmd) {
	if msg.cancelled {
		return m, nil // Abbruch bereits durch Ctrl+C-Handler behandelt
	}
	output := strings.TrimRight(msg.output, "\n")
	if output == "" {
		output = L.MsgNoOutput
	}
	if msg.err != nil {
		errText := L.MsgExitError + msg.err.Error()
		m.logActivity(actError, m.currentCmd+": "+msg.err.Error())
		output += "\n" + errText
	} else {
		m.logActivity(actExec, "✓ "+m.currentCmd)
	}
	m.addMessage(roleSystem, output)
	m.updateViewport()

	ctx, cancel := context.WithCancel(context.Background())
	m.cancelFunc = cancel
	m.state = stateLoading
	return m, tea.Batch(cmdSendToolResult(ctx, m.agent, msg.callID, output), tickCmd())
}

// --- Tea-Befehlsfabriken ---

func cmdSendMessage(ctx context.Context, agent *Agent, msg string) tea.Cmd {
	return func() tea.Msg {
		resp, err := agent.SendMessage(ctx, msg)
		if err != nil {
			return errMsg{err}
		}
		return agentResponseMsg{resp}
	}
}

// scrollToConfigField scrollt den Viewport so, dass das aktuell ausgewählte
// Einstellungsfeld (cfgSection==1) sichtbar ist.
func (m model) scrollToConfigField() model {
	if m.cfgSection != 1 {
		return m
	}
	line := m.configFieldLine(m.configSel)
	target := line - m.viewport.Height/3
	if target < 0 {
		target = 0
	}
	m.viewport.SetYOffset(target)
	return m
}

func cmdSendToolResult(ctx context.Context, agent *Agent, callID, result string) tea.Cmd {
	return func() tea.Msg {
		resp, err := agent.SendToolResult(ctx, callID, result)
		if err != nil {
			return errMsg{err}
		}
		return agentResponseMsg{resp}
	}
}

func cmdRunCommand(ctx context.Context, callID, command string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.CommandContext(ctx, "bash", "-c", command)
		out, err := cmd.CombinedOutput()
		if ctx.Err() != nil {
			return commandResultMsg{callID: callID, cancelled: true}
		}
		return commandResultMsg{
			callID: callID,
			output: string(out),
			err:    err,
		}
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(_ time.Time) tea.Msg {
		return spinTickMsg{}
	})
}

// cycleAutoUpdate dreht "ask" → "auto" → "off" → "ask"
func cycleAutoUpdate(current string) string {
	switch current {
	case "ask":
		return "auto"
	case "auto":
		return "off"
	default:
		return "ask"
	}
}

func cmdDiscover(host string) tea.Cmd {
	return func() tea.Msg {
		models, authFailed := discoverFromInput(host)
		return discoveryResultMsg{models: models, authFailed: authFailed}
	}
}

// cmdDiscoverModels sucht Modelle an einer bekannten URL (inkl. API-Key).
func cmdDiscoverModels(baseURL, apiKey string) tea.Cmd {
	return func() tea.Msg {
		models, authFailed := probeURLWithAuth(baseURL, apiKey)
		return discoveryResultMsg{models: models, authFailed: authFailed}
	}
}

func cmdHealthCheck(baseURL, profileName string, profiles []llmProfile, activeIdx int) tea.Cmd {
	return func() tea.Msg {
		if healthCheck(baseURL) == nil {
			return healthCheckMsg{ok: true, profileName: profileName}
		}
		suggested := ""
		for i, p := range profiles {
			if i == activeIdx {
				continue
			}
			if healthCheck(p.BaseURL) == nil {
				suggested = p.Name
				break
			}
		}
		return healthCheckMsg{ok: false, profileName: profileName, suggested: suggested}
	}
}
