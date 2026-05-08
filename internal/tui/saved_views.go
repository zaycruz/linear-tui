package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/roeyazroel/linear-tui/internal/logger"
)

// SavedView represents a named filter configuration persisted to disk.
type SavedView struct {
	Name       string `json:"name"`
	Assignee   string `json:"assignee,omitempty"`  // "me" or user ID
	State      string `json:"state,omitempty"`    // state name
	Priority   int    `json:"priority,omitempty"` // 0 = no filter
	Cycle      string `json:"cycle,omitempty"`    // "current" or cycle ID
	LabelID    string `json:"label_id,omitempty"`
}

// savedViewsFile is the JSON structure for the saved_views section of config.json.
type savedViewsFile struct {
	SavedViews []SavedView `json:"saved_views"`
}

// savedViewsConfigPath returns the path to the config.json file.
func savedViewsConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".linear-tui", "config.json"), nil
}

// loadSavedViews loads saved views from config.json.
func loadSavedViews() ([]SavedView, error) {
	path, err := savedViewsConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	// Parse as a generic map so we don't clobber other keys
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	svRaw, ok := raw["saved_views"]
	if !ok {
		return nil, nil
	}

	var views []SavedView
	if err := json.Unmarshal(svRaw, &views); err != nil {
		return nil, fmt.Errorf("parse saved_views: %w", err)
	}
	return views, nil
}

// saveSavedViews writes saved views to config.json without touching other keys.
func saveSavedViews(views []SavedView) error {
	path, err := savedViewsConfigPath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	// Read existing JSON to preserve other keys
	var raw map[string]json.RawMessage
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read config: %w", err)
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &raw); err != nil {
			raw = make(map[string]json.RawMessage)
		}
	}
	if raw == nil {
		raw = make(map[string]json.RawMessage)
	}

	viewsJSON, err := json.Marshal(views)
	if err != nil {
		return fmt.Errorf("marshal saved_views: %w", err)
	}
	raw["saved_views"] = json.RawMessage(viewsJSON)

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	out = append(out, '\n')

	if err := os.WriteFile(path, out, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	logger.Debug("tui.saved_views: saved %d views", len(views))
	return nil
}
