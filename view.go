package main

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
	"github.com/rivo/uniseg"
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

	// Discover-View: Content mit aktueller Viewport-Höhe frisch generieren,
	// damit vis-Berechnung immer konsistent mit der angezeigten Höhe ist.
	if m.state == stateDiscover {
		m.viewport.SetContent(m.renderDiscoverContent())
		m.viewport.SetYOffset(0)
	}

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
	var mouseBadge string
	if m.cfg.mouseTracking {
		mouseBadge = mouseBadgeOnStyle.Render(L.BadgeMouseOn)
	} else {
		mouseBadge = mouseBadgeOffStyle.Render(L.BadgeMouseOff)
	}
	badgeWidth := lipgloss.Width(modeBadge) + lipgloss.Width(mouseBadge) + lipgloss.Width(sessBadge)

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
	return titleBarStyle.Width(titleWidth).Render(titleText) + modeBadge + mouseBadge + sessBadge
}

// wrapToWidth bricht einen (ggf. ANSI-gestylten) Text explizit bei width um, statt
// das dem Terminal zu überlassen. Ein unkontrollierter Terminal-Umbruch würde eine
// zusätzliche Bildschirmzeile erzeugen, die bottomLines() nicht kennt — Folge wäre
// eine um eine Zeile zu große Gesamtausgabe, die Zeilen am unteren Rand verschluckt.
func wrapToWidth(s string, width int) string {
	if width < 1 {
		return s
	}
	wrapped := lipgloss.NewStyle().Width(width).Render(s)
	lines := strings.Split(wrapped, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return strings.Join(lines, "\n")
}

// ambiguousWide gibt an, ob das Terminal East-Asian-Ambiguous-Zeichen (…, ▶, –, •)
// doppelt breit darstellt – in Terminals mit CJK-Locale üblich. go-runewidth
// leitet das aus RUNEWIDTH_EASTASIAN bzw. der Locale ab (unter Windows aus der
// Konsolen-Codepage). lipgloss.Width zählt diese Zeichen immer einfach.
var ambiguousWide = runewidth.EastAsianWidth

// cluster ist ein Graphem-Cluster samt seiner Breite in Terminalspalten.
type cluster struct {
	s string
	w int
}

// graphemes zerlegt s in Graphem-Cluster. Nicht runenweise messen: ❤️ ist
// U+2764 + U+FE0F, runenweise 1+0 Spalten, als Cluster aber 2 – und 👍🏽 darf
// beim Kürzen nicht vom Hautton-Modifier getrennt werden.
func graphemes(s string) []cluster {
	var cs []cluster
	g := uniseg.NewGraphemes(s)
	for g.Next() {
		cs = append(cs, cluster{g.Str(), clusterWidth(g.Str(), g.Width())})
	}
	return cs
}

// clusterWidth korrigiert die uniseg-Breite w eines Clusters um ambiguousWide.
func clusterWidth(c string, w int) int {
	if w == 1 && ambiguousWide {
		if r, _ := utf8.DecodeRuneInString(c); runewidth.IsAmbiguousWidth(r) {
			return 2
		}
	}
	return w
}

// cellWidth misst eine einzeilige (ggf. ANSI-gestylte) Zeichenkette in
// Terminalspalten wie lipgloss.Width, berücksichtigt aber ambiguousWide.
// ANSI-Sequenzen bestehen aus ASCII und tragen nichts zum Aufschlag bei.
func cellWidth(s string) int {
	w := lipgloss.Width(s)
	if ambiguousWide {
		g := uniseg.NewGraphemes(s)
		for g.Next() {
			w += clusterWidth(g.Str(), g.Width()) - g.Width()
		}
	}
	return w
}

const ellipsis = "…"

// truncToWidth kürzt s (ungestylt) auf höchstens width Terminalspalten und
// markiert die Kürzung mit "…". Gemessen wird in Spalten, nicht in Runen:
// CJK-Zeichen belegen zwei Spalten, eine runenbasierte Grenze ließe die Zeile
// über die Terminalbreite hinauslaufen.
func truncToWidth(s string, width int) string {
	return truncWithTail(s, width, ellipsis)
}

// truncWithTail kürzt wie truncToWidth, markiert die Kürzung aber mit tail
// (leer: ohne Markierung). Passt tail selbst nicht, entfällt es.
func truncWithTail(s string, width int, tail string) string {
	if width < 1 {
		return ""
	}
	if cellWidth(s) <= width {
		return s
	}
	if cellWidth(tail) > width {
		tail = ""
	}
	budget := width - cellWidth(tail)
	var b strings.Builder
	w := 0
	for _, c := range graphemes(s) {
		if w+c.w > budget {
			break
		}
		b.WriteString(c.s)
		w += c.w
	}
	return b.String() + tail
}

// truncLeftToWidth kürzt s (ungestylt) von links auf höchstens width
// Terminalspalten – für URLs, bei denen das Ende (Host:Port) die Information trägt.
func truncLeftToWidth(s string, width int) string {
	if width < 1 {
		return ""
	}
	if cellWidth(s) <= width {
		return s
	}
	head := ellipsis
	if cellWidth(head) > width {
		head = ""
	}
	budget := width - cellWidth(head)
	cs := graphemes(s)
	i, w := len(cs), 0
	for i > 0 && w+cs[i-1].w <= budget {
		i--
		w += cs[i].w
	}
	var b strings.Builder
	b.WriteString(head)
	for _, c := range cs[i:] {
		b.WriteString(c.s)
	}
	return b.String()
}

// padToWidth füllt s rechts auf width Terminalspalten auf. Nicht über fmt's
// "%-*s": dessen Breite zählt Runen, nicht Spalten. Für Umlaute ist beides
// gleich, für Breitzeichen nicht — "通义" sind zwei Runen, belegen aber vier
// Spalten und liefen damit zwei Spalten zu weit.
func padToWidth(s string, width int) string {
	if pad := width - cellWidth(s); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}

// fitColumns richtet name links auf nameCols Spalten aus und kürzt rest so, dass
// name + "  " + rest höchstens avail Spalten belegt. Reicht der Platz nicht, wird
// zuerst der Name gekürzt (nicht unter minName), danach rest; bleibt für rest
// nichts übrig, entfallen rest und Trenner (restOut == ""). rest wird von rechts
// gekürzt, bei fromLeft von links. Beide Teile kommen ungestylt zurück, damit der
// Aufrufer sie getrennt einfärben kann.
func fitColumns(name string, nameCols, minName int, rest string, fromLeft bool, avail int) (nameOut, restOut string) {
	const sep = 2
	nameCols = minInt(nameCols, maxInt(avail-sep-cellWidth(rest), minName))
	restCols := avail - nameCols - sep
	if restCols < 1 {
		return truncToWidth(name, avail), ""
	}
	if fromLeft {
		restOut = truncLeftToWidth(rest, restCols)
	} else {
		restOut = truncToWidth(rest, restCols)
	}
	return padToWidth(truncToWidth(name, nameCols), nameCols), restOut
}

// selMarker ist der Auswahlpfeil vor Listenzeilen. "▶" ist East-Asian-Ambiguous;
// markerWidth liefert seine tatsächliche Breite, damit unmarkierte Zeilen genauso
// weit eingerückt werden.
const selMarker = "▶ "

func markerWidth() int { return cellWidth(selMarker) }

// markerPrefix liefert für die ausgewählte Zeile den Pfeil, sonst gleich breiten Leerraum.
func markerPrefix(sel bool) string {
	if sel {
		return selMarker
	}
	return strings.Repeat(" ", markerWidth())
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

// renderInputWrapped rendert das Eingabefeld mit Zeilenumbruch bei Terminalbrei.
func (m model) renderInputWrapped() string {
	const promptStr = " > "
	const promptWidth = 3

	value := m.input.Value()
	valueRunes := []rune(value)
	cursorPos := m.input.Position()

	avail := m.width - promptWidth
	if avail < 1 {
		avail = 1
	}

	if len(valueRunes) == 0 {
		if m.input.Placeholder != "" {
			phRunes := []rune(m.input.Placeholder)
			if len(phRunes) > 0 {
				return promptStyle.Render(promptStr) +
					inputCursorStyle.Render(string(phRunes[:1])) +
					placeholderStyle.Render(string(phRunes[1:]))
			}
		}
		return promptStyle.Render(promptStr) + inputCursorStyle.Render(" ")
	}

	var chunks [][]rune
	for i := 0; i < len(valueRunes); i += avail {
		end := i + avail
		if end > len(valueRunes) {
			end = len(valueRunes)
		}
		chunks = append(chunks, valueRunes[i:end])
	}

	var lines []string
	for i, chunk := range chunks {
		pfx := "   "
		if i == 0 {
			pfx = promptStyle.Render(promptStr)
		}
		lineStart := i * avail
		isLast := i == len(chunks)-1
		onThisLine := (cursorPos >= lineStart && cursorPos < lineStart+len(chunk)) ||
			(isLast && cursorPos == len(valueRunes))

		var rendered string
		if onThisLine {
			col := cursorPos - lineStart
			if col < len(chunk) {
				rendered = pfx +
					inputTextStyle.Render(string(chunk[:col])) +
					inputCursorStyle.Render(string(chunk[col:col+1])) +
					inputTextStyle.Render(string(chunk[col+1:]))
			} else {
				rendered = pfx + inputTextStyle.Render(string(chunk)) + inputCursorStyle.Render(" ")
			}
		} else {
			rendered = pfx + inputTextStyle.Render(string(chunk))
		}
		lines = append(lines, rendered)
	}
	return strings.Join(lines, "\n")
}

func (m model) renderIdleBottom() string {
	inputLine := m.renderInputWrapped()

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
		// Spalten in Terminalspalten ausrichten und auf m.width kürzen: renderBottom
		// wird nicht umgebrochen, eine überlange Beschreibung bräche das Terminal
		// selbst um – und bottomLines() läge eine Zeile daneben.
		avail := m.width - 1 - markerWidth()
		for i, cmd := range visible {
			name, desc := fitColumns(cmd.Name, 18, 8, cmd.Description, false, avail)
			entry := name
			if desc != "" {
				entry += "  " + acDescStyle.Render(desc)
			}
			if start+i == m.acSel {
				lines = append(lines, acItemSelectedStyle.Render(" "+markerPrefix(true)+entry))
			} else {
				lines = append(lines, acItemStyle.Render(" "+markerPrefix(false)+entry))
			}
		}
		if more {
			lines = append(lines, dimStyle.Render(fmt.Sprintf("   "+L.ACMore, len(m.filtered)-end)))
		}
		acBlock := strings.Join(lines, "\n")
		return acBlock + "\n" + inputLine + "\n" + hintStyle.Render(L.ACHint)
	}

	if m.canRetry {
		return inputLine + "\n" + hintStyle.Render(L.RetryHint)
	}
	hint := hintStyle.Render("  ") +
		hintKeyStyle.Render(L.HintSlashKey) + hintStyle.Render(" "+L.HintSlashLabel+"  ") +
		hintKeyStyle.Render(L.HintMouseKey) + hintStyle.Render(" "+L.HintMouseLabel+"  ") +
		hintKeyStyle.Render(L.HintHistoryKeys) + hintStyle.Render(" "+L.HintHistoryLabel+"  ") +
		hintKeyStyle.Render(L.HintModeKey) + hintStyle.Render(" "+L.HintModeLabel+"  ") +
		hintKeyStyle.Render(L.HintShortcutKeys) + hintStyle.Render(" "+L.HintShortcutLabel+"  ") +
		hintKeyStyle.Render(L.HintQuitKey) + hintStyle.Render(" "+L.HintQuitLabel)
	return inputLine + "\n" + wrapToWidth(hint, m.width)
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
		if m.profileSubSel >= 0 {
			var hint1 string
			if m.profileSubSel == 1 {
				hint1 = "  " + hintKeyStyle.Render("↑↓") + hintStyle.Render(" "+L.ConfigNavLabel+"  ") +
					hintKeyStyle.Render("Enter") + hintStyle.Render(" "+L.ConfigEditLabel+"  ") +
					hintKeyStyle.Render("Space") + hintStyle.Render(" "+L.ProfileModelDiscover)
			} else {
				hint1 = "  " + hintKeyStyle.Render("↑↓") + hintStyle.Render(" "+L.ConfigNavLabel+"  ") +
					hintKeyStyle.Render("Enter") + hintStyle.Render(" "+L.ConfigEditLabel)
			}
			hint2 := "  " + hintKeyStyle.Render("Tab") + hintStyle.Render(" "+L.ProfileTabSettings+"  ") +
				hintKeyStyle.Render("Esc") + hintStyle.Render(" "+L.ConfigCloseLabel)
			return hint1 + "\n" + hint2
		}
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
		if m.discEditProfile >= 0 && m.discErr != "" {
			// Fehler bei Profil-Modellsuche: nur Esc anbieten
			hint := "  " + hintKeyStyle.Render("Esc") + hintStyle.Render(" "+L.ConfigCancelLabel)
			return hint + "\n"
		}
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

// sectionHeading rendert eine Abschnittsüberschrift ("── TITEL ────"). Die
// Zierlinie ist fest lang und wird bei schmalem Terminal ohne "…" abgeschnitten.
func (m model) sectionHeading(title string) string {
	return "  " + sectionStyle.Render(truncWithTail(title, m.width-2, "")) + "\n\n"
}

// discTitle ist der Kopf aller Discover-Schritte.
func (m model) discTitle() string {
	return "\n" + m.sectionHeading(L.DiscoveryTitle)
}

// discFoundLine meldet die Zahl der gefundenen Modelle. Sie wird auf m.width
// umgebrochen: liefe sie über, bräche der Viewport sie selbst um, und die
// Zeilenrechnung in discModelRows ginge nicht mehr auf.
func (m model) discFoundLine() string {
	found := wrapText(fmt.Sprintf(L.DiscoveryFoundFmt, len(m.discModels)), m.width, "  ")
	return dimStyle.Render(found) + "\n\n"
}

// discModelRows ist die Zahl der Modellzeilen, die unter Titel und Fundzeile in
// den Viewport passen.
func (m model) discModelRows() int {
	vis := m.viewport.Height - strings.Count(m.discTitle()+m.discFoundLine(), "\n")
	if vis < 1 {
		vis = 1
	}
	return vis
}

func (m model) renderDiscoverContent() string {
	var sb strings.Builder
	sb.WriteString(m.discTitle())

	switch m.discStep {
	case discEnterHost:
		if m.discErr != "" {
			sb.WriteString(errorMsgStyle.Render("  ✗ "+m.discErr) + "\n\n")
		}
		if m.discEditProfile < 0 {
			sb.WriteString("  " + dimStyle.Render(L.DiscoveryInputHint) + "\n")
		}

	case discScanning:
		sb.WriteString("  " + dimStyle.Render(L.DiscoveryScanPorts) + "\n")

	case discPickModel:
		sb.WriteString(m.discFoundLine())
		windowEnd := m.discModelOffset + m.discModelRows()
		if windowEnd > len(m.discModels) {
			windowEnd = len(m.discModels)
		}
		// Spaltenbudget der Zeile: Pfeil + Name + 2 (Separator) + Quelle. Die Quelle
		// steht rechtsbündig und wird von links gekürzt – bei langen URLs trägt das
		// Ende (Host:Port) die Information. Ist das Terminal zu schmal, wird zuerst
		// der Name bis auf 8 Spalten gekürzt, dann die Quelle.
		const maxSrcCols, minNameCols = 22, 8
		avail := m.width - markerWidth()
		for i := m.discModelOffset; i < windowEnd; i++ {
			fm := m.discModels[i]
			sel := i == m.modelSel
			// nameCols = avail: fitColumns lässt dem Namen alles, was die Quelle nicht braucht.
			name, source := fitColumns(fm.Name, avail, minNameCols,
				truncLeftToWidth(fm.Source, maxSrcCols), true, avail)
			line := name
			if source != "" {
				line += "  " + dimStyle.Render(source)
			}
			if sel {
				sb.WriteString(configLabelSelectedStyle.Render(markerPrefix(true)+line) + "\n")
			} else {
				sb.WriteString(markerPrefix(false) + configLabelStyle.Render(line) + "\n")
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
	sb.WriteString(m.sectionHeading(L.SectionProfiles))
	isProfileSec := m.cfgSection == 0
	for i, p := range m.cfg.profiles {
		sel := isProfileSec && m.profileSel == i && m.profileSubSel == -1
		cursor := markerPrefix(false)
		lblStyle := configLabelStyle
		if sel {
			cursor = configLabelSelectedStyle.Render(markerPrefix(true))
			lblStyle = configLabelSelectedStyle
		}
		// Name auf 18 Spalten, dahinter die URL und die Marker. Gekürzt wird in
		// Terminalspalten: Profilnamen dürfen Breitzeichen enthalten.
		const profileNameCols, minNameCols = 18, 8
		var suffix, suffixPlain string
		if m.cfg.activeProfileIdx == i {
			suffix += dimStyle.Render(" " + L.ProfileActive)
			suffixPlain += " " + L.ProfileActive
		}
		if p.Preferred {
			suffix += " " + preferredStyle.Render(L.ProfilePreferred)
			suffixPlain += " " + L.ProfilePreferred
		}
		avail := m.width - markerWidth()
		if avail-cellWidth(suffixPlain) < minNameCols {
			suffix = "" // zu schmal: lieber die Marker opfern als den Namen
		} else {
			avail -= cellWidth(suffixPlain)
		}
		name, url := fitColumns(p.Name, profileNameCols, minNameCols, p.BaseURL, false, avail)
		line := name
		if url != "" {
			line += "  " + dimStyle.Render(url)
		}
		sb.WriteString(cursor + lblStyle.Render(line+suffix) + "\n")

		// Unterfelder (URL / Modell / API-Key) nur bei cfgSection==0 und ausgewähltem Profil
		if isProfileSec && m.profileSel == i {
			// URL
			apiDisp := dimStyle.Render(L.FieldAPIKeyEmpty)
			if p.APIKey != "" {
				apiDisp = strings.Repeat("•", minInt(len(p.APIKey), 20))
			}
			sb.WriteString(m.renderProfileSubField(L.FieldEndpoint, p.BaseURL, 0, false))
			sb.WriteString(m.renderProfileSubField(L.FieldModel, p.Model, 1, false))
			sb.WriteString(m.renderProfileSubField(L.FieldAPIKey, apiDisp, 2, true))
			sb.WriteString(m.renderProfileSubField(L.FieldProfileName, p.Name, 3, false))
		}
	}
	// "Add"-Button
	addSel := isProfileSec && m.profileSel == len(m.cfg.profiles)
	addCursor := markerPrefix(false)
	addStyle := configLabelStyle
	if addSel {
		addCursor = configLabelSelectedStyle.Render(markerPrefix(true))
		addStyle = configLabelSelectedStyle
	}
	sb.WriteString(addCursor + addStyle.Render(L.ProfileAddBtn) + "\n\n")

	// Einstellungen (nur in cfgSection==1 sichtbar)
	sb.WriteString(m.sectionHeading(L.SectionAssistant))

	// Feld 0: Sprache (Toggle)
	langDisplay := "Deutsch"
	switch m.cfg.lang {
	case "en":
		langDisplay = "English"
	case "zh":
		langDisplay = "中文"
	}
	sb.WriteString(m.renderConfigField(0, L.FieldLang, langDisplay+" "+dimStyle.Render(L.ModeToggleHint), true))

	// Feld 1: Ausführmodus (Toggle)
	modeVal := configAskStyle.Render("🛡 " + L.ModeAsk)
	if m.cfg.autoAllow {
		modeVal = configAutoStyle.Render("⚡ " + L.ModeAuto)
	}
	sb.WriteString(m.renderConfigField(1, L.FieldMode, modeVal+dimStyle.Render("  "+L.ModeToggleHint), true))
	sb.WriteString("\n")

	// Feld 2: Kurzbefehl (selfInstall toggle)
	installed := selfInstallIsInstalled()
	installVal := configAskStyle.Render("○ " + L.ModeNotInstalled)
	if installed {
		installVal = configAutoStyle.Render("✓ " + L.ModeInstalled)
	}
	sb.WriteString(m.renderConfigField(2, L.FieldInstall, installVal+dimStyle.Render("  "+L.ModeToggleHint), true))

	// Feld 3: Auto-Update (Cycle)
	var updateVal string
	switch m.cfg.autoUpdate {
	case "auto":
		updateVal = configAutoStyle.Render("⟳ " + L.ModeUpdateAuto)
	case "off":
		updateVal = dimStyle.Render("○ " + L.ModeUpdateOff)
	default: // "ask"
		updateVal = configAskStyle.Render("? " + L.ModeUpdateAsk)
	}
	if m.cfg.autoUpdate != "off" && !m.nextUpdateAt.IsZero() {
		remaining := time.Until(m.nextUpdateAt)
		var hint string
		if remaining <= 0 {
			hint = "prüft…"
		} else if remaining < time.Minute {
			hint = fmt.Sprintf("in %ds", int(remaining.Seconds()))
		} else {
			hint = fmt.Sprintf("in %dmin", int(remaining.Minutes()))
		}
		updateVal += dimStyle.Render("  " + hint)
	}
	sb.WriteString(m.renderConfigField(3, L.FieldAutoUpdate, updateVal+" "+dimStyle.Render(L.ModeToggleHint), true))

	// Feld 4: Sitzungen speichern (Toggle)
	sessVal := configAskStyle.Render(L.ModeSessionOff)
	if m.cfg.saveSessions {
		sessVal = configAutoStyle.Render(L.ModeSessionOn)
	}
	sb.WriteString(m.renderConfigField(4, L.FieldSession, sessVal+" "+dimStyle.Render(L.ModeToggleHint), true))

	// Feld 5: LLM-Timeout
	timeoutVal := fmt.Sprintf("%d %s", m.cfg.llmTimeout, L.FieldTimeoutHint)
	sb.WriteString(m.renderConfigField(5, L.FieldTimeout, timeoutVal, false))
	sb.WriteString("\n")

	// Feld 6: System-Prompt
	// Mehrzeiliger Prompt: Zeilenumbrüche zu Leerzeichen, sonst sprengt die
	// Vorschau die einzeilige Konfigzeile.
	promptPreview := strings.Join(strings.Fields(m.cfg.customPrompt), " ")
	if promptPreview == "" {
		promptPreview = L.FieldPromptEmpty
	} else {
		promptPreview = truncToWidth(promptPreview, 40)
	}
	sb.WriteString(m.renderConfigField(6, L.FieldPrompt, promptPreview, false))
	sb.WriteString("\n")

	// TASTENKÜRZEL / SHORTCUTS
	sb.WriteString(m.sectionHeading(L.SectionShortcuts))
	for i := 0; i < 9; i++ {
		val := m.cfg.shortcuts[i]
		if val == "" {
			val = L.FieldShortcutEmpty
		}
		sb.WriteString(m.renderConfigField(7+i, fmt.Sprintf("F%d", i+1), val, false))
	}
	sb.WriteString("\n")

	return sb.String()
}

// renderProfileSubField rendert ein Unterfeld (URL/Modell/API-Key) des ausgewählten Profils.
// preStyled=true: value ist bereits mit Lipgloss formatiert (z.B. für API-Key als Punkte).
func (m model) renderProfileSubField(label, value string, subIdx int, preStyled bool) string {
	sel := m.profileSubSel == subIdx
	editing := sel && m.configEditing

	cursor := "  " + markerPrefix(false)
	if sel {
		cursor = "  " + configLabelSelectedStyle.Render(markerPrefix(true))
	}
	return cursor + m.renderFieldBody(label, value, sel, editing, preStyled, 2+markerWidth())
}

// renderConfigField rendert eine Konfigurationszeile (nur für cfgSection==1).
// preStyled=true: value ist bereits mit Lipgloss formatiert.
func (m model) renderConfigField(idx int, label, value string, preStyled bool) string {
	sel := m.cfgSection == 1 && m.configSel == idx
	editing := sel && m.configEditing

	cursor := markerPrefix(false)
	if sel {
		cursor = configLabelSelectedStyle.Render(markerPrefix(true))
	}
	return cursor + m.renderFieldBody(label, value, sel, editing, preStyled, markerWidth())
}

// configLabelCols ist die Breite der Label-Spalte im Konfigurationseditor.
const configLabelCols = 14

// renderFieldBody rendert Label und Wert einer Konfigzeile hinter einem Cursor
// von indent Spalten. Das Label wird in Terminalspalten aufgefüllt (padToWidth),
// nicht mit "%-14s": die Labels kommen aus i18n, und chinesische Labels belegen
// je Zeichen zwei Spalten. Der Wert wird auf die Restbreite gekürzt.
func (m model) renderFieldBody(label, value string, sel, editing, preStyled bool, indent int) string {
	lblStyle := configLabelStyle
	valStyle := configValueStyle
	if sel {
		lblStyle = configLabelSelectedStyle
		valStyle = configValueSelectedStyle
	}
	// Ein Label, das breiter als die Spalte ist ("Sitzungen speichern"), belegt
	// seine volle Breite; der Wert bekommt dann entsprechend weniger Platz.
	label = truncToWidth(label, m.width-indent)
	lbl := lblStyle.Render(padToWidth(label, configLabelCols))

	if editing {
		return lbl + "  " + configEditingStyle.Render(L.ConfigEditingBelow) + "\n"
	}

	valW := m.width - indent - maxInt(cellWidth(label), configLabelCols) - 2
	if valW < 1 {
		return lbl + "\n"
	}
	var valRendered string
	if preStyled {
		// Bereits gestylt: ANSI-fest kürzen, statt die Escape-Sequenzen zu zerschneiden.
		valRendered = ansi.Truncate(value, valW, ellipsis)
	} else {
		valRendered = valStyle.Render(truncToWidth(value, valW))
	}
	return lbl + "  " + valRendered + "\n"
}
