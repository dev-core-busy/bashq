package main

import "strings"

type cmdAction int

const (
	actionRun          cmdAction = iota // Sofort als Nachricht an Agent senden
	actionPrompt                        // Eingabefeld mit Prefix füllen
	actionClear                         // Chat leeren
	actionExit                          // Beenden
	actionHelp                          // Hilfe anzeigen
	actionConfig                        // Konfigurationseditor öffnen
	actionActivities                    // Aktivitätsprotokoll anzeigen
	actionSelfInstall                   // bashq system-weit installieren / deinstallieren
	actionColors                        // Terminal-Farben in ~/.bashrc einrichten
)

type SlashCommand struct {
	Name        string
	Description string
	Action      cmdAction
	Message     string // für actionRun: die Nachricht an den Agent
	Prompt      string // für actionPrompt: Prefix im Eingabefeld
}

// getCommands gibt die Befehlsliste in der aktiven Sprache zurück.
func getCommands() []SlashCommand {
	return []SlashCommand{
		{Name: "/install", Description: L.CmdInstallDesc, Action: actionPrompt, Prompt: L.CmdInstallPrompt},
		{Name: "/remove", Description: L.CmdRemoveDesc, Action: actionPrompt, Prompt: L.CmdRemovePrompt},
		{Name: "/update", Description: L.CmdUpdateDesc, Action: actionRun, Message: L.CmdUpdateMsg},
		{Name: "/status", Description: L.CmdStatusDesc, Action: actionRun, Message: L.CmdStatusMsg},
		{Name: "/disk", Description: L.CmdDiskDesc, Action: actionRun, Message: L.CmdDiskMsg},
		{Name: "/memory", Description: L.CmdMemoryDesc, Action: actionRun, Message: L.CmdMemoryMsg},
		{Name: "/network", Description: L.CmdNetworkDesc, Action: actionRun, Message: L.CmdNetworkMsg},
		{Name: "/services", Description: L.CmdServicesDesc, Action: actionPrompt, Prompt: L.CmdServicesPrompt},
		{Name: "/logs", Description: L.CmdLogsDesc, Action: actionRun, Message: L.CmdLogsMsg},
		{Name: "/optimize", Description: L.CmdOptimizeDesc, Action: actionRun, Message: L.CmdOptimizeMsg},
		{Name: "/config", Description: L.CmdConfigDesc, Action: actionConfig},
		{Name: "/setup", Description: L.CmdSetupDesc, Action: actionSelfInstall},
		{Name: "/colors", Description: L.CmdColorsDesc, Action: actionColors},
		{Name: "/activities", Description: L.CmdActivitiesDesc, Action: actionActivities},
		{Name: "/help", Description: L.CmdHelpDesc, Action: actionHelp},
		{Name: "/clear", Description: L.CmdClearDesc, Action: actionClear},
		{Name: "/exit", Description: L.CmdExitDesc, Action: actionExit},
	}
}

func filterCommands(query string) []SlashCommand {
	cmds := getCommands()
	if query == "" {
		return cmds
	}
	q := strings.ToLower(query)
	var result []SlashCommand
	for _, cmd := range cmds {
		if strings.Contains(strings.ToLower(cmd.Name), q) ||
			strings.Contains(strings.ToLower(cmd.Description), q) {
			result = append(result, cmd)
		}
	}
	return result
}
