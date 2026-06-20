package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type savedProfile struct {
	Name      string `json:"name"`
	BaseURL   string `json:"base_url"`
	Model     string `json:"model"`
	APIKey    string `json:"api_key"`
	Preferred bool   `json:"preferred"`
}

type savedConfig struct {
	BaseURL       string         `json:"base_url"`
	Model         string         `json:"model"`
	APIKey        string         `json:"api_key"`
	AutoAllow     bool           `json:"auto_allow"`
	CustomPrompt  string         `json:"custom_prompt"`
	Shortcuts     [9]string      `json:"shortcuts"`
	Lang          string         `json:"lang"`
	Profiles      []savedProfile `json:"profiles,omitempty"`
	ActiveProfile int            `json:"active_profile"`
	SaveSessions  *bool          `json:"save_sessions,omitempty"` // nil = default true
	AutoUpdate    string         `json:"auto_update,omitempty"`   // "" = default "ask"
}

func configFilePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "bashq", "config.json"), nil
}

func loadConfig() appConfig {
	cfg := appConfig{
		baseURL:          defaultBaseURL,
		model:            defaultModel,
		autoAllow:        false,
		saveSessions:     true,
		autoUpdate:       "ask",
		lang:             detectSystemLang(),
		activeProfileIdx: -1,
	}

	path, err := configFilePath()
	if err != nil {
		return cfg
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}

	var saved savedConfig
	if err := json.Unmarshal(data, &saved); err != nil {
		return cfg
	}

	if saved.BaseURL != "" {
		cfg.baseURL = saved.BaseURL
	}
	if saved.Model != "" {
		cfg.model = saved.Model
	}
	cfg.apiKey = saved.APIKey
	cfg.autoAllow = saved.AutoAllow
	cfg.customPrompt = saved.CustomPrompt
	cfg.shortcuts = saved.Shortcuts
	if saved.SaveSessions != nil {
		cfg.saveSessions = *saved.SaveSessions
	}
	if saved.AutoUpdate != "" {
		cfg.autoUpdate = saved.AutoUpdate
	}
	if saved.Lang != "" {
		cfg.lang = saved.Lang
	}

	cfg.profiles = make([]llmProfile, len(saved.Profiles))
	for i, sp := range saved.Profiles {
		cfg.profiles[i] = llmProfile{
			Name:      sp.Name,
			BaseURL:   sp.BaseURL,
			Model:     sp.Model,
			APIKey:    sp.APIKey,
			Preferred: sp.Preferred,
		}
	}
	if saved.ActiveProfile >= 0 && saved.ActiveProfile < len(cfg.profiles) {
		cfg.activeProfileIdx = saved.ActiveProfile
		p := cfg.profiles[cfg.activeProfileIdx]
		if p.BaseURL != "" {
			cfg.baseURL = p.BaseURL
		}
		if p.Model != "" {
			cfg.model = p.Model
		}
		if p.APIKey != "" {
			cfg.apiKey = p.APIKey
		}
	}
	return cfg
}

func saveConfig(cfg appConfig) error {
	path, err := configFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	savedProfiles := make([]savedProfile, len(cfg.profiles))
	for i, p := range cfg.profiles {
		savedProfiles[i] = savedProfile{
			Name:      p.Name,
			BaseURL:   p.BaseURL,
			Model:     p.Model,
			APIKey:    p.APIKey,
			Preferred: p.Preferred,
		}
	}
	activeIdx := cfg.activeProfileIdx
	if activeIdx < 0 {
		activeIdx = 0
	}
	saveSessions := cfg.saveSessions
	data, err := json.MarshalIndent(savedConfig{
		BaseURL:       cfg.baseURL,
		Model:         cfg.model,
		APIKey:        cfg.apiKey,
		AutoAllow:     cfg.autoAllow,
		CustomPrompt:  cfg.customPrompt,
		Shortcuts:     cfg.shortcuts,
		Lang:          cfg.lang,
		Profiles:      savedProfiles,
		ActiveProfile: activeIdx,
		SaveSessions:  &saveSessions,
		AutoUpdate:    cfg.autoUpdate,
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
