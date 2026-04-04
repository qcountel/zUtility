package app

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
)

type ModuleConfig struct {
	Name     string  `json:"name"`
	Enabled  bool    `json:"enabled"`
	Value    float64 `json:"value,omitempty"`
	MaxValue float64 `json:"max_value,omitempty"`
	Mode     int     `json:"mode,omitempty"`
	BindVK   uint32  `json:"bind_vk,omitempty"`
}

type AppSettings struct {
	Modules        []ModuleConfig `json:"modules"`
	MinimizeToTray bool           `json:"minimize_to_tray"`
	ShowHotkey     uint32         `json:"show_hotkey,omitempty"`
	Language       string         `json:"language,omitempty"`
	Version        string         `json:"version"`
}

func getConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	configDir := filepath.Join(homeDir, ".zutil")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", err
	}

	return filepath.Join(configDir, "config.json"), nil
}

func (app *App) SaveConfig() error {
	app.modulesMu.Lock()
	defer app.modulesMu.Unlock()

	settings := AppSettings{
		Version:        "1.0",
		MinimizeToTray: app.minimizeToTray,
		ShowHotkey:     app.showHotkey,
		Language:       normalizeLanguage(app.language),
		Modules:        make([]ModuleConfig, 0, len(app.modules)),
	}

	for _, m := range app.modules {
		cfg := ModuleConfig{
			Name:    canonicalModuleName(m.Name()),
			Enabled: m.IsEnabled(),
		}

		if valuer, ok := m.(interface{ Value() float64 }); ok {
			cfg.Value = valuer.Value()
		}

		if valuer, ok := m.(interface{ MaxValue() float64 }); ok {
			cfg.MaxValue = valuer.MaxValue()
		}

		if moder, ok := m.(interface{ ModeInt() int }); ok {
			cfg.Mode = moder.ModeInt()
		}

		if vker, ok := m.(interface{ BindVK() uint32 }); ok {
			cfg.BindVK = vker.BindVK()
		}

		settings.Modules = append(settings.Modules, cfg)
	}

	configPath, err := getConfigPath()
	if err != nil {
		return fmt.Errorf("get config path: %w", err)
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	app.conf.Logger.Info("configuration saved", "path", configPath)
	return nil
}

func (app *App) LoadConfig() error {
	configPath, err := getConfigPath()
	if err != nil {
		return fmt.Errorf("get config path: %w", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			app.conf.Logger.Info("config file not found, using defaults")
			return nil
		}
		return fmt.Errorf("read config: %w", err)
	}

	var settings AppSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return fmt.Errorf("unmarshal config: %w", err)
	}

	app.minimizeToTray = settings.MinimizeToTray
	app.showHotkey = settings.ShowHotkey
	app.language = normalizeLanguage(settings.Language)

	app.modulesMu.Lock()
	defer app.modulesMu.Unlock()

	for _, cfg := range settings.Modules {
		for _, m := range app.modules {
			if canonicalModuleName(m.Name()) == canonicalModuleName(cfg.Name) {
				if cfg.Enabled {
					m.Enable()
				}

				if setter, ok := m.(interface{ SetValue(float64) }); ok && cfg.Value != 0 {
					setter.SetValue(cfg.Value)
				}

				if setter, ok := m.(interface{ SetMaxValue(float64) }); ok && cfg.MaxValue != 0 {
					setter.SetMaxValue(cfg.MaxValue)
				}

				if setter, ok := m.(interface{ SetModeInt(int) }); ok {
					setter.SetModeInt(cfg.Mode)
				}

				if setter, ok := m.(interface{ SetBindVK(uint32) }); ok && cfg.BindVK != 0 {
					setter.SetBindVK(cfg.BindVK)
				}
				break
			}
		}
	}

	app.conf.Logger.Info("configuration loaded", "path", configPath)
	return nil
}

func (app *App) ExportConfig(parent fyne.Window) {
	dialog.ShowFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowError(err, parent)
			parent.Close()
			return
		}
		if writer == nil {
			parent.Close()
			return
		}
		defer writer.Close()

		app.modulesMu.Lock()
		settings := AppSettings{
			Version:        "1.0",
			MinimizeToTray: app.minimizeToTray,
			ShowHotkey:     app.showHotkey,
			Language:       normalizeLanguage(app.language),
			Modules:        make([]ModuleConfig, 0, len(app.modules)),
		}

		for _, m := range app.modules {
			cfg := ModuleConfig{
				Name:    canonicalModuleName(m.Name()),
				Enabled: m.IsEnabled(),
			}

			if valuer, ok := m.(interface{ Value() float64 }); ok {
				cfg.Value = valuer.Value()
			}

			settings.Modules = append(settings.Modules, cfg)
		}
		app.modulesMu.Unlock()

		data, err := json.MarshalIndent(settings, "", "  ")
		if err != nil {
			dialog.ShowError(err, parent)
			parent.Close()
			return
		}

		if _, err := writer.Write(data); err != nil {
			dialog.ShowError(err, parent)
			parent.Close()
			return
		}

		dialog.ShowInformation(app.t("Успешно", "Success"), app.t("Конфигурация экспортирована!", "Configuration exported!"), parent)
		parent.Close()
	}, parent)
}

func (app *App) ImportConfig(parent fyne.Window) {
	dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, parent)
			parent.Close()
			return
		}
		if reader == nil {
			parent.Close()
			return
		}
		defer reader.Close()

		data, err := io.ReadAll(reader)
		if err != nil {
			dialog.ShowError(err, parent)
			parent.Close()
			return
		}

		var settings AppSettings
		if err := json.Unmarshal(data, &settings); err != nil {
			dialog.ShowError(fmt.Errorf("%s: %w", app.t("неверный формат конфигурации", "invalid configuration format"), err), parent)
			parent.Close()
			return
		}

		app.minimizeToTray = settings.MinimizeToTray
		app.showHotkey = settings.ShowHotkey
		app.language = normalizeLanguage(settings.Language)

		app.modulesMu.Lock()
		for _, cfg := range settings.Modules {
			for _, m := range app.modules {
				if canonicalModuleName(m.Name()) == canonicalModuleName(cfg.Name) {
					if cfg.Enabled {
						m.Enable()
					} else {
						m.Disable()
					}

					if setter, ok := m.(interface{ SetValue(float64) }); ok && cfg.Value != 0 {
						setter.SetValue(cfg.Value)
					}
					break
				}
			}
		}
		app.modulesMu.Unlock()

		if err := app.SaveConfig(); err != nil {
			app.conf.Logger.Error("failed to save imported config", "err", err)
		}

		dialog.ShowInformation(app.t("Успешно", "Success"), app.t("Конфигурация импортирована и применена!", "Configuration imported and applied!"), parent)
		parent.Close()
	}, parent)
}

func (app *App) ResetConfig(parent fyne.Window) {
	dialog.ShowConfirm(app.t("Подтверждение", "Confirmation"),
		app.t("Вы уверены, что хотите сбросить все настройки?", "Are you sure you want to reset all settings?"),
		func(confirmed bool) {
			if !confirmed {
				parent.Close()
				return
			}

			app.modulesMu.Lock()
			for _, m := range app.modules {
				m.Disable()
				if setter, ok := m.(interface{ SetValue(float64) }); ok {
					setter.SetValue(1.0)
				}
			}
			app.modulesMu.Unlock()

			app.minimizeToTray = false

			if err := app.SaveConfig(); err != nil {
				app.conf.Logger.Error("failed to save reset config", "err", err)
			}

			dialog.ShowInformation(app.t("Успешно", "Success"), app.t("Настройки сброшены к значениям по умолчанию!", "Settings were reset to defaults!"), parent)
			parent.Close()
		}, parent)
}
