package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m model) View() string {
	if !m.ready {
		return L.Starting
	}

	// Viewport-Höhe immer frisch berechnen — State kann sich ändern ohne recalcViewport-Aufruf
	if m.height > 0 {
		h := m.height - 2 - m.bottomLines()
		if h < 1 {
			h = 1
		}
		m.viewport.Height = h
	}

	title := m.renderTitle()
	divider := dividerStyle.Render(strings.Repeat("─", m.width))
	bottom := m.renderBottom()

	var mid string
	if m.state == stateEditPrompt {
		mid = m.promptEditor.View()
	} else {
		mid = m.viewport.View()
	}

	return strings.Join([]string{title, mid, divider, bottom}, "\n")
}

// --- Titelzeile ---

func (m model) renderTitle() string {
	var modeBadge string
	if m.cfg.autoAllow {
		modeBadge = autoModeBadgeStyle.Render(L.BadgeAuto)
	} else {
		modeBadge = askModeBadgeStyle.Render(L.BadgeAsk)
	}
	var sessBadge string
	if m.cfg.saveSessions {
		sessBadge = sessionBadgeOnStyle.Render(L.BadgeSessionOn)
	} else {
		sessBadge = sessionBadgeOffStyle.Render(L.BadgeSessionOff)
	}
	badgeWidth := lipgloss.Width(modeBadge) + lipgloss.Width(sessBadge)

	var label string
	switch m.state {
	case stateConfig, stateEditPrompt, stateDiscover:
		label = L.TitleConfig
	default:
		label = L.TitleApp
	}

	titleWidth := m.width - badgeWidth
	if titleWidth < 0 {
		titleWidth = 0
	}
	titleText := "  " + label + "  " + currentVersion
	return titleBarStyle.Width(titleWidth).Render(titleText) + modeBadge + sessBadge
}

// --- Unterer Bereich ---

func (m model) renderBottom() string {
	switch m.state {
	case stateConfirm:
		return m.renderConfirmBottom()
	case stateLoading:
		return m.renderLoadingBottom()
	case stateExecuting:
		return m.renderExecutingBottom()
	case stateConfig:
		return m.renderConfigBottom()
	case stateEditPrompt:
		return m.renderEditPromptBottom()
	case stateDiscover:
		return m.renderDiscoverBottom()
	default:
		return m.renderIdleBottom()
	}
}

func (m model) renderIdleBottom() string {
	inputLine := m.input.View()

	if m.showAC && len(m.filtered) > 0 {
		// Gleitendes Fenster: acSel ist immer in der sichtbaren Auswahl
		start := 0
		if m.acSel >= maxACVisible {
			start = m.acSel - maxACVisible + 1
		}
		end := start + maxACVisible
		if end > len(m.filtered) {
			end = len(m.filtered)
		}
		visible := m.filtered[start:end]
		more := len(m.filtered) > end

		var lines []string
		for i, cmd := range visible {
			entry := fmt.Sprintf("%-14s", cmd.Name)
			if cmd.Description != "" {
				entry += "  " + acDescStyle.Render(cmd.Description)
			}
			if start+i == m.acSel {
				lines = append(lines, acItemSelectedStyle.Render(" ▶ "+entry))
			} else {
				lines = append(lines, acItemStyle.Render("   "+entry))
			}
		}
		if more {
			lines = append(lines, dimStyle.Render(fmt.Sprintf("   "+L.ACMore, len(m.filtered)-end)))
		}
		acBlock := strings.Join(lines, "\n")
		return acBlock + "\n" + inputLine + "\n" + hintStyle.Render(L.ACHint)
	}

	hint := hintStyle.Render("  ") +
		hintKeyStyle.Render(L.HintSlashKey) + hintStyle.Render(" "+L.HintSlashLabel+"  ") +
		hintKeyStyle.Render(L.HintScrollKeys) + hintStyle.Render(" "+L.HintScrollLabel+"  ") +
		hintKeyStyle.Render(L.HintHistoryKeys) + hintStyle.Render(" "+L.HintHistoryLabel+"  ") +
		hintKeyStyle.Render(L.HintModeKey) + hintStyle.Render(" "+L.HintModeLabel+"  ") +
		hintKeyStyle.Render(L.HintShortcutKeys) + hintStyle.Render(" "+L.HintShortcutLabel+"  ") +
		hintKeyStyle.Render(L.HintQuitKey) + hintStyle.Render(" "+L.HintQuitLabel)
	return inputLine + "\n" + hint
}

func (m model) renderConfirmBottom() string {
	if m.pendingTool != nil {
		t := m.pendingTool
		maxW := m.width - 6
		if maxW < 10 {
			maxW = 10
		}
		cmdText := t.command
		if len(cmdText) > maxW {
			cmdText = cmdText[:maxW] + "…"
		}
		explText := t.explanation
		if len(explText) > maxW {
			explText = explText[:maxW] + "…"
		}
		cmd := confirmCmdStyle.Render("  $ " + cmdText)
		expl := confirmExplStyle.Render("  " + explText)
		hints := "  " + hintKeyStyle.Render(L.ConfirmYesKey) + hintStyle.Render(" "+L.ConfirmRunLabel+"  ") +
			hintKeyStyle.Render(L.ConfirmNoKey) + hintStyle.Render(" "+L.ConfirmCancelLabel)
		return strings.Join([]string{cmd, expl, hints}, "\n")
	}
	return ""
}

func (m model) renderLoadingBottom() string {
	frame := loadingStyle.Render(m.spinnerFrame())
	return "  " + frame + loadingStyle.Render(L.LoadingThinking) + "\n" +
		hintStyle.Render(L.LoadingQuit)
}

func (m model) renderExecutingBottom() string {
	frame := loadingStyle.Render(m.spinnerFrame())
	cmd := m.currentCmd
	max := m.width - 20
	if max > 0 && len(cmd) > max {
		cmd = cmd[:max] + "…"
	}
	return "  " + frame + loadingStyle.Render(L.LoadingRunning) + confirmCmdStyle.Render(cmd) + "\n" +
		hintStyle.Render(L.LoadingCmdRunning)
}

func (m model) renderConfigBottom() string {
	if m.configEditing {
		label := hintStyle.Render("  "+m.configFieldLabel()+": ") + m.input.View()
		hints := "  " + hintKeyStyle.Render(L.ConfigSaveKey) + hintStyle.Render(" "+L.ConfigSaveLabel+"  ") +
			hintKeyStyle.Render(L.ConfigCancelKey) + hintStyle.Render(" "+L.ConfigCancelLabel)
		return label + "\n" + hints + "\n"
	}
	if m.cfgSection == 0 {
		hint1 := "  " + hintKeyStyle.Render("↑↓") + hintStyle.Render(" "+L.ConfigNavLabel+"  ") +
			hintKeyStyle.Render("Enter") + hintStyle.Render(" aktivieren/hinzufügen  ") +
			hintKeyStyle.Render("P") + hintStyle.Render(" "+L.ProfilePreferLabel+"  ") +
			hintKeyStyle.Render("D") + hintStyle.Render(" "+L.ProfileDeleteLabel)
		hint2 := "  " + hintKeyStyle.Render("Tab") + hintStyle.Render(" "+L.ProfileTabSettings+"  ") +
			hintKeyStyle.Render("Esc") + hintStyle.Render(" "+L.ConfigCloseLabel)
		return hint1 + "\n" + hint2
	}
	hint1 := "  " + hintKeyStyle.Render(L.ConfigNavKeys) + hintStyle.Render(" "+L.ConfigNavLabel+"  ") +
		hintKeyStyle.Render(L.ConfigEditKey) + hintStyle.Render(" "+L.ConfigEditLabel+"  ") +
		hintKeyStyle.Render(L.ConfigCloseKey) + hintStyle.Render(" "+L.ConfigCloseLabel)
	hint2 := "  " + hintKeyStyle.Render("Tab") + hintStyle.Render(" "+L.ProfileTabProfiles+"  ") +
		hintKeyStyle.Render(L.HintModeKey) + hintStyle.Render(" "+L.ConfigModeHint)
	return hint1 + "\n" + hint2
}

func (m model) renderEditPromptBottom() string {
	hint1 := "  " + hintKeyStyle.Render(L.PromptSaveKey) + hintStyle.Render(" "+L.PromptSaveLabel+"  ") +
		hintKeyStyle.Render(L.PromptCancelKey) + hintStyle.Render(" "+L.PromptCancelLabel)
	return hint1 + "\n" + hintStyle.Render(L.PromptDesc)
}

// --- Discovery-View ---

func (m model) renderDiscoverBottom() string {
	switch m.discStep {
	case discEnterHost:
		label := hintStyle.Render("  "+L.DiscoveryInputLabel+": ") + m.input.View()
		hint := "  " + hintKeyStyle.Render("Enter") + hintStyle.Render(" "+L.DiscoverySearchLabel+"  ") +
			hintKeyStyle.Render("Esc") + hintStyle.Render(" "+L.ConfigCancelLabel)
		return label + "\n" + hint + "\n"

	case discScanning:
		frame := loadingStyle.Render(m.spinnerFrame())
		return "  " + frame + loadingStyle.Render(fmt.Sprintf(" "+L.DiscoveryScanFmt, m.discHost)) + "\n" +
			hintStyle.Render("  Esc: "+L.ConfigCancelLabel)

	case discPickModel:
		hint1 := "  " + hintKeyStyle.Render("↑↓") + hintStyle.Render(" "+L.ConfigNavLabel+"  ") +
			hintKeyStyle.Render("Enter") + hintStyle.Render(" "+L.ConfigEditLabel+"  ") +
			hintKeyStyle.Render("Esc") + hintStyle.Render(" "+L.ConfigCancelLabel)
		return hint1 + "\n" + hintStyle.Render("")

	case discEnterName:
		label := hintStyle.Render("  "+L.DiscoveryNameLabel+": ") + m.input.View()
		hint := "  " + hintKeyStyle.Render("Enter") + hintStyle.Render(" "+L.ConfigSaveLabel+"  ") +
			hintKeyStyle.Render("Esc") + hintStyle.Render(" "+L.ConfigCancelLabel)
		return label + "\n" + hint + "\n"
	}
	return ""
}

func (m model) renderDiscoverContent() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString("  " + sectionStyle.Render(L.DiscoveryTitle) + "\n\n")

	switch m.discStep {
	case discEnterHost:
		if m.discErr != "" {
			sb.WriteString(errorMsgStyle.Render("  ✗ "+m.discErr) + "\n\n")
		}
		sb.WriteString("  " + dimStyle.Render(L.DiscoveryInputHint) + "\n")

	case discScanning:
		sb.WriteString("  " + dimStyle.Render(L.DiscoveryScanPorts) + "\n")

	case discPickModel:
		sb.WriteString("  " + dimStyle.Render(fmt.Sprintf(L.DiscoveryFoundFmt, len(m.discModels))) + "\n\n")
		for i, fm := range m.discModels {
			sel := i == m.modelSel
			maxN := m.width - 30
			if maxN < 10 {
				maxN = 10
			}
			name := fm.Name
			if len(name) > maxN {
				name = name[:maxN] + "…"
			}
			line := fmt.Sprintf("%-*s  %s", maxN, name, dimStyle.Render(fm.Source))
			if sel {
				sb.WriteString(configLabelSelectedStyle.Render("▶ "+line) + "\n")
			} else {
				sb.WriteString("  " + configLabelStyle.Render(line) + "\n")
			}
		}

	case discEnterName:
		info := fmt.Sprintf("  %s  %s", m.tempProfile.Model, dimStyle.Render(m.tempProfile.BaseURL))
		sb.WriteString(info + "\n\n")
		sb.WriteString("  " + dimStyle.Render(L.DiscoveryInputHint) + "\n")
	}

	return sb.String()
}

// --- Konfigurationsinhalt ---

func (m model) renderConfigContent() string {
	var sb strings.Builder
	sb.WriteString("\n")

	// LLM-PROFILE
	sb.WriteString("  " + sectionStyle.Render(L.SectionProfiles) + "\n\n")
	isProfileSec := m.cfgSection == 0
	for i, p := range m.cfg.profiles {
		sel := isProfileSec && m.profileSel == i
		cursor := "  "
		lblStyle := configLabelStyle
		if sel {
			cursor = configLabelSelectedStyle.Render("▶ ")
			lblStyle = configLabelSelectedStyle
		}
		urlShort := p.BaseURL
		maxU := m.width - 36
		if maxU < 10 {
			maxU = 10
		}
		if len(urlShort) > maxU {
			urlShort = urlShort[:maxU] + "…"
		}
		line := fmt.Sprintf("%-18s  %s", p.Name, dimStyle.Render(urlShort))
		if m.cfg.activeProfileIdx == i {
			line += dimStyle.Render(" "+L.ProfileActive)
		}
		if p.Preferred {
			line += " " + preferredStyle.Render(L.ProfilePreferred)
		}
		sb.WriteString(cursor + lblStyle.Render(line) + "\n")
	}
	// "Add"-Button
	addSel := isProfileSec && m.profileSel == len(m.cfg.profiles)
	addCursor := "  "
	addStyle := configLabelStyle
	if addSel {
		addCursor = configLabelSelectedStyle.Render("▶ ")
		addStyle = configLabelSelectedStyle
	}
	sb.WriteString(addCursor + addStyle.Render(L.ProfileAddBtn) + "\n\n")

	// VERBINDUNG / CONNECTION
	sb.WriteString("  " + sectionStyle.Render(L.SectionConnection) + "\n\n")
	if m.cfg.activeProfileIdx >= 0 && m.cfg.activeProfileIdx < len(m.cfg.profiles) {
		p := m.cfg.profiles[m.cfg.activeProfileIdx]
		profLine := configLabelStyle.Render(fmt.Sprintf("%-14s", L.FieldActiveProfile)) +
			"  " + configValueSelectedStyle.Render(p.Name)
		if p.Preferred {
			profLine += "  " + dimStyle.Render(L.ProfilePreferred)
		}
		sb.WriteString("  " + profLine + "\n\n")
	}
	sb.WriteString(m.renderConfigField(0, L.FieldEndpoint, m.cfg.baseURL, false))
	sb.WriteString(m.renderConfigField(1, L.FieldModel, m.cfg.model, false))

	apiDisplay := dimStyle.Render(L.FieldAPIKeyEmpty)
	if m.cfg.apiKey != "" {
		apiDisplay = strings.Repeat("•", minInt(len(m.cfg.apiKey), 20))
	}
	sb.WriteString(m.renderConfigField(2, L.FieldAPIKey, apiDisplay, true))
	sb.WriteString("\n")

	// Einstellungen
	sb.WriteString("  " + sectionStyle.Render(L.SectionAssistant) + "\n\n")

	// Feld 3: Sprache (Toggle)
	langDisplay := "Deutsch"
	switch m.cfg.lang {
	case "en":
		langDisplay = "English"
	case "zh":
		langDisplay = "中文"
	}
	sb.WriteString(m.renderConfigField(3, L.FieldLang, langDisplay+" "+dimStyle.Render(L.ModeToggleHint), true))

	// Feld 4: Ausführmodus (Toggle)
	modeVal := configAskStyle.Render("🛡 " + L.ModeAsk)
	if m.cfg.autoAllow {
		modeVal = configAutoStyle.Render("⚡ " + L.ModeAuto)
	}
	sb.WriteString(m.renderConfigField(4, L.FieldMode, modeVal+dimStyle.Render("  "+L.ModeToggleHint), true))

	// Feld 5: Kurzbefehl (selfInstall toggle)
	installed := selfInstallIsInstalled()
	installVal := configAskStyle.Render("○ " + L.ModeNotInstalled)
	if installed {
		installVal = configAutoStyle.Render("✓ " + L.ModeInstalled)
	}
	sb.WriteString(m.renderConfigField(5, L.FieldInstall, installVal+dimStyle.Render("  "+L.ModeToggleHint), true))

	// Feld 6: Auto-Update (Cycle)
	var updateVal string
	switch m.cfg.autoUpdate {
	case "auto":
		updateVal = configAutoStyle.Render("⟳ " + L.ModeUpdateAuto)
	case "off":
		updateVal = dimStyle.Render("○ " + L.ModeUpdateOff)
	default: // "ask"
		updateVal = configAskStyle.Render("? " + L.ModeUpdateAsk)
	}
	sb.WriteString(m.renderConfigField(6, L.FieldAutoUpdate, updateVal+" "+dimStyle.Render(L.ModeToggleHint), true))

	// Feld 7: Sitzungen speichern (Toggle)
	sessVal := configAskStyle.Render(L.ModeSessionOff)
	if m.cfg.saveSessions {
		sessVal = configAutoStyle.Render(L.ModeSessionOn)
	}
	sb.WriteString(m.renderConfigField(7, L.FieldSession, sessVal+" "+dimStyle.Render(L.ModeToggleHint), true))
	sb.WriteString("\n")

	// Feld 8: System-Prompt
	promptPreview := m.cfg.customPrompt
	if promptPreview == "" {
		promptPreview = L.FieldPromptEmpty
	} else if len(promptPreview) > 40 {
		promptPreview = promptPreview[:40] + "…"
	}
	sb.WriteString(m.renderConfigField(8, L.FieldPrompt, promptPreview, false))
	sb.WriteString("\n")

	// TASTENKÜRZEL / SHORTCUTS
	sb.WriteString("  " + sectionStyle.Render(L.SectionShortcuts) + "\n\n")
	for i := 0; i < 9; i++ {
		val := m.cfg.shortcuts[i]
		if val == "" {
			val = L.FieldShortcutEmpty
		}
		sb.WriteString(m.renderConfigField(9+i, fmt.Sprintf("F%d", i+1), val, false))
	}
	sb.WriteString("\n")

	return sb.String()
}

// renderConfigField rendert eine Konfigurationszeile.
// preStyled=true: value ist bereits mit Lipgloss formatiert.
func (m model) renderConfigField(idx int, label, value string, preStyled bool) string {
	sel := m.configSel == idx
	editing := sel && m.configEditing

	cursor := "  "
	lblStyle := configLabelStyle
	if sel {
		cursor = configLabelSelectedStyle.Render("▶ ")
		lblStyle = configLabelSelectedStyle
	}

	if editing {
		return cursor + lblStyle.Render(fmt.Sprintf("%-14s", label)) +
			"  " + configEditingStyle.Render(L.ConfigEditingBelow) + "\n"
	}

	maxVal := m.width - 22
	if maxVal < 10 {
		maxVal = 10
	}

	var valRendered string
	if preStyled {
		valRendered = value
	} else {
		v := value
		if len(v) > maxVal {
			v = v[:maxVal] + "…"
		}
		if sel {
			valRendered = configValueSelectedStyle.Render(v)
		} else {
			valRendered = configValueStyle.Render(v)
		}
	}

	return cursor + lblStyle.Render(fmt.Sprintf("%-14s", label)) + "  " + valRendered + "\n"
}
