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
	Name        string  `json:"name"`
	Enabled     bool    `json:"enabled"`
	Value       float64 `json:"value,omitempty"`
	MaxValue    float64 `json:"max_value,omitempty"`
	Mode        int     `json:"mode,omitempty"`
	BindVK      uint32  `json:"bind_vk,omitempty"`
	BindVKRight uint32  `json:"bind_vk_right,omitempty"`
}

type AppSettings struct {
	Modules            []ModuleConfig `json:"modules"`
	MinimizeToTray     bool           `json:"minimize_to_tray"`
	AnimationsEnabled  bool           `json:"animations_enabled"`
	UseClonedMinecraft bool           `json:"use_cloned_minecraft"`
	AccentColor        string         `json:"accent_color"`
	Version            string         `json:"version"`
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
		Version:            CurrentVersion,
		MinimizeToTray:     app.minimizeToTray,
		AnimationsEnabled:  app.animationsEnabled,
		UseClonedMinecraft: app.useClonedMinecraft,
		AccentColor:        CurrentPaletteName(),
		Modules:            make([]ModuleConfig, 0, len(app.modules)),
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

		if vker, ok := m.(interface{ BindVKRight() uint32 }); ok {
			cfg.BindVKRight = vker.BindVKRight()
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
	app.animationsEnabled = settings.AnimationsEnabled
	AnimationsEnabled = app.animationsEnabled
	app.useClonedMinecraft = settings.UseClonedMinecraft
	if settings.AccentColor != "" {
		SetPalette(settings.AccentColor)
		app.rebuildAfterPaletteChange()
	}
	app.modulesMu.Lock()
	defer app.modulesMu.Unlock()

	for _, cfg := range settings.Modules {
		for _, m := range app.modules {
			if canonicalModuleName(m.Name()) == canonicalModuleName(cfg.Name) {
				if cfg.Enabled {
					m.Enable()
				}

				if setter, ok := m.(interface{ SetValue(float64) }); ok {
					setter.SetValue(cfg.Value)
				}

				if setter, ok := m.(interface{ SetMaxValue(float64) }); ok {
					setter.SetMaxValue(cfg.MaxValue)
				}

				if setter, ok := m.(interface{ SetModeInt(int) }); ok {
					setter.SetModeInt(cfg.Mode)
				}

				if setter, ok := m.(interface{ SetBindVK(uint32) }); ok {
					setter.SetBindVK(cfg.BindVK)
				}

				if setter, ok := m.(interface{ SetBindVKRight(uint32) }); ok {
					setter.SetBindVKRight(cfg.BindVKRight)
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
			Version:            CurrentVersion,
			MinimizeToTray:     app.minimizeToTray,
			AnimationsEnabled:  app.animationsEnabled,
			UseClonedMinecraft: app.useClonedMinecraft,
			AccentColor:        CurrentPaletteName(),
			Modules:            make([]ModuleConfig, 0, len(app.modules)),
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

		dialog.ShowInformation("Успешно", "Конфигурация экспортирована!", parent)
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
			dialog.ShowError(fmt.Errorf("%s: %w", "неверный формат конфигурации", err), parent)
			parent.Close()
			return
		}

		app.minimizeToTray = settings.MinimizeToTray
	if settings.AccentColor != "" {
		SetPalette(settings.AccentColor)
		app.rebuildAfterPaletteChange()
	}

	app.modulesMu.Lock()
		for _, cfg := range settings.Modules {
			for _, m := range app.modules {
				if canonicalModuleName(m.Name()) == canonicalModuleName(cfg.Name) {
					if cfg.Enabled {
						m.Enable()
					} else {
						m.Disable()
					}

					if setter, ok := m.(interface{ SetValue(float64) }); ok {
						setter.SetValue(cfg.Value)
					}

					if setter, ok := m.(interface{ SetBindVK(uint32) }); ok {
						setter.SetBindVK(cfg.BindVK)
					}

					if setter, ok := m.(interface{ SetBindVKRight(uint32) }); ok {
						setter.SetBindVKRight(cfg.BindVKRight)
					}
					break
				}
			}
		}
		app.modulesMu.Unlock()

		if err := app.SaveConfig(); err != nil {
			app.conf.Logger.Error("failed to save imported config", "err", err)
		}

		dialog.ShowInformation("Успешно", "Конфигурация импортирована и применена!", parent)
		parent.Close()
	}, parent)
}

func (app *App) ResetConfig(parent fyne.Window) {
	dialog.ShowConfirm("Подтверждение",
		"Вы уверены, что хотите сбросить все настройки?",
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
				if setter, ok := m.(interface{ SetModeInt(int) }); ok {
					setter.SetModeInt(0)
				}
				if setter, ok := m.(interface{ SetBindVK(uint32) }); ok {
					if m.Name() == "InstantHotbar" {
						setter.SetBindVK(5)
					} else {
						setter.SetBindVK(0)
					}
				}
				if setter, ok := m.(interface{ SetBindVKRight(uint32) }); ok {
					if m.Name() == "InstantHotbar" {
						setter.SetBindVKRight(6)
					} else {
						setter.SetBindVKRight(0)
					}
				}
			}
			app.modulesMu.Unlock()

		app.minimizeToTray = false
		app.animationsEnabled = true
		AnimationsEnabled = true
		app.useClonedMinecraft = false
		SetPalette("red")
		app.rebuildAfterPaletteChange()

			if err := app.SaveConfig(); err != nil {
				app.conf.Logger.Error("failed to save reset config", "err", err)
			}

			dialog.ShowInformation("Успешно", "Настройки сброшены к значениям по умолчанию!", parent)
			parent.Close()
		}, parent)
}
