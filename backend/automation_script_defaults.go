package backend

import (
	"os"
	"path/filepath"

	"ant-chrome/backend/internal/automation"
)

const (
	automationScriptDefaultsMarkerName = "defaults-seeded-v9"
)

var automationScriptDefaultsLegacyMarkerNames = []string{
	"defaults-seeded-v8",
	"defaults-seeded-v7",
	"defaults-seeded-v6",
	"defaults-seeded-v5",
	"defaults-seeded-v4",
	"defaults-seeded-v3",
	"defaults-seeded-v2",
}

func (a *App) automationScriptDefaultsMarkerPath(name string) string {
	return a.resolveAppPath(filepath.ToSlash(filepath.Join("data", "automation", name)))
}

func (a *App) automationScriptDefaultsInitializedByName(name string) bool {
	info, err := os.Stat(a.automationScriptDefaultsMarkerPath(name))
	return err == nil && !info.IsDir()
}

func (a *App) automationScriptDefaultsInitialized() bool {
	return a.automationScriptDefaultsInitializedByName(automationScriptDefaultsMarkerName)
}

func (a *App) automationScriptDefaultsInitializedAnyLegacy() bool {
	for _, name := range automationScriptDefaultsLegacyMarkerNames {
		if a.automationScriptDefaultsInitializedByName(name) {
			return true
		}
	}
	return false
}

func (a *App) markAutomationScriptDefaultsInitialized() error {
	markerPath := a.automationScriptDefaultsMarkerPath(automationScriptDefaultsMarkerName)
	if err := os.MkdirAll(filepath.Dir(markerPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(markerPath, []byte("ok\n"), 0o644)
}

func (a *App) ensureAutomationScriptDefaults(store *automation.ScriptStore) error {
	defaults, err := automation.DefaultScriptBundles()
	if err != nil {
		return err
	}
	items, err := store.List()
	if err != nil {
		return err
	}

	// v2 marker exists: defaults were already initialized or user intentionally removed them.
	if a.automationScriptDefaultsInitialized() {
		return nil
	}

	if len(items) == 0 {
		// Keep legacy behavior for users that had deleted all defaults under v1.
		if a.automationScriptDefaultsInitializedAnyLegacy() {
			return a.markAutomationScriptDefaultsInitialized()
		}

		for _, bundle := range defaults {
			if _, err := store.ImportBundle(bundle); err != nil {
				return err
			}
		}
		return a.markAutomationScriptDefaultsInitialized()
	}

	// Migration from v1: existing scripts are present, add any missing built-in baselines once.
	if a.automationScriptDefaultsInitializedAnyLegacy() {
		existingIDs := make(map[string]struct{}, len(items))
		for _, item := range items {
			existingIDs[item.ID] = struct{}{}
		}
		for _, bundle := range defaults {
			if _, exists := existingIDs[bundle.Record.ID]; exists {
				continue
			}
			if _, err := store.ImportBundle(bundle); err != nil {
				return err
			}
		}
	}
	return a.markAutomationScriptDefaultsInitialized()
}
