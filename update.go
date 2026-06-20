package main

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// configFieldCount: 0=baseURL, 1=model, 2=apiKey, 3=autoAllow, 4=customPrompt,
// 5-13=F1-F9, 14=lang, 15=saveSessions, 16=autoUpdate
const configFieldCount = 17

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
		m.logActivity(actError, msg.err.Error())
		m.addMessage(roleError, msg.err.Error())
		m.updateViewport()
		return m, nil

	case discoveryResultMsg:
		if m.state == stateDiscover {
			if len(msg.models) == 0 {
				m.discErr = fmt.Sprintf(L.DiscoveryNoneFmt, m.discHost)
				m.discStep = discEnterHost
			} else {
				m.discModels = msg.models
				m.discStep = discPickModel
				m.modelSel = 0
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
	case "ctrl+c":
		if m.cfg.saveSessions {
			saveSession(m.messages, m.agent.history)
		}
		return m, tea.Quit

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
	return m, tea.Batch(cmdSendMessage(m.agent, msg), tickCmd())
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
		return m, tea.Batch(cmdRunCommand(tool.id, tool.command), tickCmd())
	case "n", "esc":
		tool := m.pendingTool
		m.pendingTool = nil
		m.toolQueue = nil
		m.state = stateLoading
		m.addMessage(roleSystem, L.MsgCancelled)
		m.updateViewport()
		return m, tea.Batch(
			cmdSendToolResult(m.agent, tool.id, L.MsgToolRejected),
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
		m.input.Placeholder = L.InputPlaceholder
		m.recalcViewport()
		m.updateViewport()

	case "tab":
		m.cfgSection = 1 - m.cfgSection
		n := len(m.cfg.profiles)
		if m.cfgSection == 0 && m.profileSel > n {
			m.profileSel = n
		}
		m.viewport.SetContent(m.renderConfigContent())
		m = m.scrollToConfigField()

	case "up":
		if m.cfgSection == 0 {
			if m.profileSel > 0 {
				m.profileSel--
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
			n := len(m.cfg.profiles)
			if m.profileSel < n {
				m.profileSel++
			}
		} else {
			m.configSel = (m.configSel + 1) % configFieldCount
		}
		m.viewport.SetContent(m.renderConfigContent())
		m = m.scrollToConfigField()

	case "enter":
		if m.cfgSection == 0 {
			return m.activateProfileEntry()
		}
		return m.activateConfigField()

	case " ":
		if m.cfgSection == 1 {
			switch m.configSel {
			case 3:
				m.cfg.autoAllow = !m.cfg.autoAllow
				m.viewport.SetContent(m.renderConfigContent())
			case 14:
				return m.cycleLang()
			case 15:
				m.cfg.saveSessions = !m.cfg.saveSessions
				saveConfig(m.cfg)
				m.viewport.SetContent(m.renderConfigContent())
			case 16:
				m.cfg.autoUpdate = cycleAutoUpdate(m.cfg.autoUpdate)
				saveConfig(m.cfg)
				m.viewport.SetContent(m.renderConfigContent())
			}
		}

	case "p", "P":
		if m.cfgSection == 0 && m.profileSel < len(m.cfg.profiles) {
			return m.setPreferredProfile()
		}

	case "d", "D":
		if m.cfgSection == 0 && m.profileSel < len(m.cfg.profiles) {
			return m.deleteProfile()
		}
	}

	return m, nil
}

func (m model) handleConfigEditKey(msg tea.KeyMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		value := strings.TrimSpace(m.input.Value())
		switch {
		case m.configSel == 0:
			if value == "" {
				value = defaultBaseURL
			}
			m.cfg.baseURL = value
			m.agent.baseURL = value
		case m.configSel == 1:
			if value == "" {
				value = defaultModel
			}
			m.cfg.model = value
			m.agent.model = value
		case m.configSel == 2: // API-Key darf leer sein
			m.cfg.apiKey = value
			m.agent.apiKey = value
		case m.configSel >= 5 && m.configSel <= 13:
			m.cfg.shortcuts[m.configSel-5] = value
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
	case 3: // autoAllow toggle
		m.cfg.autoAllow = !m.cfg.autoAllow
		m.viewport.SetContent(m.renderConfigContent())

	case 4: // System-Prompt → Textarea-Editor öffnen
		m.promptEditor.SetWidth(m.width)
		m.promptEditor.SetHeight(m.promptEditorHeight())
		m.promptEditor.SetValue(m.cfg.customPrompt)
		m.promptEditor.Focus()
		m.state = stateEditPrompt

	case 14: // Sprache – Toggle
		return m.cycleLang()

	case 15: // Sitzungen – Toggle
		m.cfg.saveSessions = !m.cfg.saveSessions
		saveConfig(m.cfg)
		m.viewport.SetContent(m.renderConfigContent())

	case 16: // Auto-Update – Cycle
		m.cfg.autoUpdate = cycleAutoUpdate(m.cfg.autoUpdate)
		saveConfig(m.cfg)
		m.viewport.SetContent(m.renderConfigContent())

	default:
		// Einzeiliges Text-Feld
		var value string
		switch {
		case m.configSel == 0:
			value = m.cfg.baseURL
		case m.configSel == 1:
			value = m.cfg.model
		case m.configSel == 2:
			value = m.cfg.apiKey
			m.input.EchoMode = textinput.EchoPassword
			m.input.EchoCharacter = '•'
		case m.configSel >= 5 && m.configSel <= 13:
			value = m.cfg.shortcuts[m.configSel-5]
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
			return m.returnToConfig()
		case "enter":
			host := strings.TrimSpace(m.input.Value())
			if host == "" {
				return m, nil
			}
			m.discHost = host
			m.discErr = ""
			m.discStep = discScanning
			m.input.SetValue("")
			m.input.Placeholder = L.InputPlaceholder
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
				m.viewport.SetContent(m.renderDiscoverContent())
			}
		case "down":
			if m.modelSel < len(m.discModels)-1 {
				m.modelSel++
				m.viewport.SetContent(m.renderDiscoverContent())
			}
		case "enter":
			if len(m.discModels) == 0 {
				return m, nil
			}
			sel := m.discModels[m.modelSel]
			m.tempProfile.BaseURL = sel.BaseURL
			m.tempProfile.Model = sel.Name
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
	m.state = stateConfig
	m.recalcViewport()
	m.viewport.SetContent(m.renderConfigContent())
	return m, nil
}

// configFieldLabel gibt den Anzeigenamen des aktuell gewählten Feldes zurück.
func (m model) configFieldLabel() string {
	switch {
	case m.configSel == 0:
		return L.FieldEndpoint
	case m.configSel == 1:
		return L.FieldModel
	case m.configSel == 2:
		return L.FieldAPIKey
	case m.configSel >= 5 && m.configSel <= 13:
		return fmt.Sprintf("F%d", m.configSel-4)
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
	m.input.SetValue("")
	m.showAC = false
	m.acSel = 0
	m.recalcViewport()
	m.addMessage(roleUser, text)
	m.logActivity(actUser, text)
	m.updateViewport()
	m.state = stateLoading
	return m, tea.Batch(cmdSendMessage(m.agent, text), tickCmd())
}

func (m model) selectCommand(cmd SlashCommand) (model, tea.Cmd) {
	m.showAC = false
	m.acSel = 0
	m.input.SetValue("")
	m.recalcViewport()

	switch cmd.Action {
	case actionRun:
		m.addMessage(roleUser, cmd.Name)
		m.logActivity(actUser, cmd.Name)
		m.updateViewport()
		m.state = stateLoading
		return m, tea.Batch(cmdSendMessage(m.agent, cmd.Message), tickCmd())
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
			saveSession(m.messages, m.agent.history)
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
		saveSession(m.messages, m.agent.history)
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
		return m, tea.Batch(cmdRunCommand(tool.id, tool.command), tickCmd())
	}

	m.pendingTool = &tool
	m.state = stateConfirm
	m.addMessage(roleAssistant, fmt.Sprintf(L.MsgConfirmCmdFmt, tool.command, tool.explanation))
	m.updateViewport()
	return m, nil
}

func (m model) handleCommandResult(msg commandResultMsg) (model, tea.Cmd) {
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

	m.state = stateLoading
	return m, tea.Batch(cmdSendToolResult(m.agent, msg.callID, output), tickCmd())
}

// --- Tea-Befehlsfabriken ---

func cmdSendMessage(agent *Agent, msg string) tea.Cmd {
	return func() tea.Msg {
		resp, err := agent.SendMessage(msg)
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

func cmdSendToolResult(agent *Agent, callID, result string) tea.Cmd {
	return func() tea.Msg {
		resp, err := agent.SendToolResult(callID, result)
		if err != nil {
			return errMsg{err}
		}
		return agentResponseMsg{resp}
	}
}

func cmdRunCommand(callID, command string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("bash", "-c", command)
		out, err := cmd.CombinedOutput()
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
		return discoveryResultMsg{models: discoverFromInput(host)}
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
