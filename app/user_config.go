package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/something-that-is-cool/zutil/app/module"
	"github.com/something-that-is-cool/zutil/pkg/e"
)

type UserConfig struct {
	Modules    map[string]module.Property `json:"modules"`
	Binds      map[string]string          `json:"binds"` // module_id => char
	CharBinds  map[string]string          `json:"-"`     // char => module_id
	LightTheme bool                       `json:"light_theme"`
	ShowErrors bool                       `json:"show_errors"`
}

func DefaultUserConfig() *UserConfig {
	return &UserConfig{
		Modules:    make(map[string]module.Property),
		Binds:      make(map[string]string),
		CharBinds:  make(map[string]string),
		LightTheme: false,
		ShowErrors: true,
	}
}

const ConfigFilename = "config.json"

func (app *App) loadUserConfigUnsafe() (conf *UserConfig, err error) {
	root := app.getRootPath()
	path := filepath.Join(root, ConfigFilename)

	_ = os.MkdirAll(filepath.Dir(path), filePerm)

	d, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		conf = DefaultUserConfig()
		if err = app.writeConfig(conf, path); err != nil {
			return nil, fmt.Errorf("write default config to %q: %w", path, err)
		}
		return conf, nil
	case err != nil:
		return nil, fmt.Errorf("read config file at %q: %w", path, err)
	}
	var c UserConfig
	if err = json.Unmarshal(d, &c); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	fillDefaults(&c)
	return &c, nil
}

func fillDefaults(c *UserConfig) {
	if c.Modules == nil {
		c.Modules = make(map[string]module.Property)
	}
	if c.Binds == nil {
		c.Binds = make(map[string]string)
	}
	if c.CharBinds == nil {
		c.CharBinds = make(map[string]string)
	}
}

func (app *App) saveUserConfig() {
	root := app.getRootPath()
	path := filepath.Join(root, ConfigFilename)

	app.userConf.Lock()
	defer app.userConf.Unlock()

	if err := app.writeConfig(app.userConf.V, path); err != nil {
		app.conf.Logger.Error("cannot save config", "err", err.Error(), "path", path)
		return
	}
	app.conf.Logger.Debug("saved user config.", "path", path)
}

func (app *App) getRootPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "."
	}
	root := filepath.Join(dir, "zutil")
	_ = os.MkdirAll(root, 0755)
	return root
}

const filePerm = 0777

func (app *App) writeConfig(conf *UserConfig, path string) (err error) {
	var d []byte
	if d, err = json.MarshalIndent(conf, "", "  "); err != nil {
		return fmt.Errorf("marshal default config: %w", err)
	}
	if err = os.WriteFile(path, d, filePerm); err != nil {
		return err
	}
	return nil
}

func (app *App) onModuleToggled(id string) func(bool, e.ActionCause) {
	return func(b bool, cause e.ActionCause) {
		if e.ActionCauseIs(cause, actionCauseModuleDisabled) || e.ActionCauseIs(cause, actionCauseModuleCreated) {
			return
		}
		app.editProperty(id, func(p *module.Property) {
			p.Enabled = b
		})
		app.conf.Logger.Debug("module toggled", "module", id, "new_state", b, "cause", cause.String())
	}
}

func onModuleValueChanged[T any](app *App, id string) func(T, e.ActionCause) {
	return func(v T, cause e.ActionCause) {
		if e.ActionCauseIs(cause, actionCauseModuleDisabled) || e.ActionCauseIs(cause, actionCauseModuleCreated) {
			return
		}
		app.editProperty(id, func(p *module.Property) {
			p.Value = v
		})
		app.conf.Logger.Debug("module value changed", "module", id, "new_val", v, "cause", cause.String())
	}
}

func (app *App) editProperty(id string, fn func(*module.Property)) {
	app.userConf.Lock()
	defer app.userConf.Unlock()

	p, ok := app.userConf.V.Modules[id]
	if !ok {
		return
	}
	fn(&p)
	app.userConf.V.Modules[id] = p
}
