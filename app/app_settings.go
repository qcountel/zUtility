package app

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/something-that-is-cool/zutil/app/module"
	"github.com/something-that-is-cool/zutil/internal/misc"
	"github.com/something-that-is-cool/zutil/internal/version"
	"github.com/something-that-is-cool/zutil/pkg/e"

	"github.com/sqweek/dialog"
)

var aboutMessage = misc.JoinNewLine(
	"zutil (MC:PE 1.1.5)",
	"build "+version.Version+" "+"("+version.Commit+")",
	"",
	"made by k4ties, anx1ous",
	"",
	"Copyright (C) 2026 Ivan Z. All rights reserved.",
)



func (app *App) importConfig() {
	filename, err := dialog.File().Filter("JSON Config", "json").Load()
	if err != nil {
		if err.Error() == "Cancelled" {
			return
		}
		app.showError("import config", err)
		return
	}
	reader, err := os.Open(filename)
	if err != nil {
		app.showError("import config", err)
		return
	}
	defer reader.Close()
	if err = app.doImport(reader); err != nil {
		app.showError("import config", err)
		return
	}
	app.showInfo("Import", misc.JoinNewLine(
		"successfully imported config.",
		filename,
	))
}

var actionCauseImportedConfig = e.NewActionCause("imported config")

func (app *App) doImport(reader io.Reader) error {
	d, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("read all (reader): %w", err)
	}
	var conf *UserConfig
	if err = json.Unmarshal(d, &conf); err != nil {
		return fmt.Errorf("unmarshal json: %w", err)
	}
	toEdit := app.applyConfig(conf)
	app.doModuleUpdates(toEdit, actionCauseImportedConfig)
	return nil
}

func (app *App) exportConfig() {
	filename, err := dialog.File().Filter("JSON Config", "json").Save()
	if err != nil {
		if err.Error() == "Cancelled" {
			return
		}
		app.showError("export config", err)
		return
	}
	writer, err := os.Create(filename)
	if err != nil {
		app.showError("export config", err)
		return
	}
	defer writer.Close()
	if err = app.doExport(writer); err != nil {
		app.showError("export config", err)
		return
	}
	app.showInfo("Export config", misc.JoinNewLine(
		"successfully exported config.",
		filename,
	))
}

func (app *App) doExport(writer io.Writer) error {
	app.userConf.Lock()
	defer app.userConf.Unlock()

	d, err := json.MarshalIndent(app.userConf.V, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config to json: %w", err)
	}
	if _, err = writer.Write(d); err != nil {
		return fmt.Errorf("export: write config to file: %w", err)
	}
	return nil
}

var actionCauseResetConfig = e.NewActionCause("reset config")

func (app *App) resetConfig() {
	toEdit := app.applyConfig(DefaultUserConfig())
	app.doModuleUpdates(toEdit, actionCauseResetConfig)
}

func (app *App) showErrors(state bool, _ e.ActionCause) {
	app.userConf.Lock()
	defer app.userConf.Unlock()
	app.userConf.V.ShowErrors = state
}

func (app *App) applyConfig(newConf *UserConfig) map[module.Module]module.Property {
	app.userConf.Lock()
	defer app.userConf.Unlock()
	app.userConf.V = newConf
	app.lightTheme = app.userConf.V.LightTheme

	app.data.Lock()
	defer app.data.Unlock()

	func() {
		app.hm.Events.Lock()
		defer app.hm.Events.Unlock()
		app.hm.ClearHandlersUnsafe()
		app.applyBindsUnsafe(app.userConf.V)
	}()
	toEdit := make(map[module.Module]module.Property)
	for conf, m := range app.data.V.modules.AllFromFront() {
		property := conf.DefaultProperty()
		if p, ok := newConf.Modules[conf.Identifier()]; ok {
			property = p
		}
		app.userConf.V.Modules[conf.Identifier()] = property
		toEdit[m] = property
	}
	return toEdit
}

func (app *App) applyBindsUnsafe(conf *UserConfig) {
	for mod, char := range conf.Binds {
		m, ok := app.moduleByIDUnsafe(mod)
		if !ok {
			continue
		}
		app.hm.HandleUnsafe(char, app.bindToggleModule(mod, m))
	}
}
