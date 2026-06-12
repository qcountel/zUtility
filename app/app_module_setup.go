package app

import (
	"errors"

	"github.com/something-that-is-cool/zutil/app/module"
	"github.com/something-that-is-cool/zutil/app/module/modules"
	"github.com/something-that-is-cool/zutil/pkg/e"
	"github.com/something-that-is-cool/zutil/pkg/win"
)

func (app *App) setupModules(proc *win.Process) []module.Config {
	return []module.Config{
		app.createControllerSensitivityModule(proc),
		app.createNoDynamicFovModule(proc),
		app.createNoHurtCamModule(proc),
		app.createAutoSprintModule(proc),
		app.createNoParticleModule(proc),
		app.createZoomModule(proc),
		app.createItemDelayFixModule(proc),
		app.createNoCamResetModule(proc),
		app.createOffVsyncModule(proc),
	}
}

func (app *App) createControllerSensitivityModule(proc *win.Process) module.Config {
	conf := &modules.ControllerSensitivity{Process: proc}
	conf.Error = app.onError(conf.Identifier())
	conf.OnValueChanged = onModuleValueChanged[float64](app, conf.Identifier())
	return conf
}

func (app *App) createNoDynamicFovModule(proc *win.Process) module.Config {
	conf := &modules.NoDynamicFov{Process: proc}
	conf.Error = app.onError(conf.Identifier())
	conf.OnToggle = app.onModuleToggled(conf.Identifier())
	return conf
}

func (app *App) createNoHurtCamModule(proc *win.Process) module.Config {
	conf := &modules.NoHurtCam{Process: proc}
	conf.Error = app.onError(conf.Identifier())
	conf.OnToggle = app.onModuleToggled(conf.Identifier())
	return conf
}

func (app *App) createAutoSprintModule(proc *win.Process) module.Config {
	conf := &modules.AutoSprint{Process: proc}
	conf.Error = app.onError(conf.Identifier())
	conf.OnToggle = app.onModuleToggled(conf.Identifier())
	return conf
}

func (app *App) createNoParticleModule(proc *win.Process) module.Config {
	conf := &modules.NoParticle{Process: proc}
	conf.Error = app.onError(conf.Identifier())
	conf.OnToggle = app.onModuleToggled(conf.Identifier())
	return conf
}

func (app *App) createZoomModule(proc *win.Process) module.Config {
	conf := &modules.Zoom{Process: proc}
	conf.Error = app.onError(conf.Identifier())
	conf.OnToggle = app.onModuleToggled(conf.Identifier())
	return conf
}

func (app *App) createItemDelayFixModule(proc *win.Process) module.Config {
	conf := &modules.ItemDelayFix{Process: proc}
	conf.Error = app.onError(conf.Identifier())
	conf.OnToggle = app.onModuleToggled(conf.Identifier())
	return conf
}

func (app *App) createNoCamResetModule(proc *win.Process) module.Config {
	conf := &modules.NoCamReset{Process: proc}
	conf.Error = app.onError(conf.Identifier())
	conf.OnToggle = app.onModuleToggled(conf.Identifier())
	return conf
}

func (app *App) createOffVsyncModule(proc *win.Process) module.Config {
	conf := &modules.OffVsync{Process: proc}
	conf.Error = app.onError(conf.Identifier())
	conf.OnToggle = app.onModuleToggled(conf.Identifier())
	return conf
}

func (app *App) onError(mod string) func(err error) {
	return func(err error) {
		if errors.As(err, new(e.ErrValuesIsAlready)) {
			return
		}
		app.conf.Logger.Error("an error occurred", "module", mod, "err", err.Error())
		app.ifUserConf(func(c *UserConfig) bool {
			return c.ShowErrors
		}, func() {
			app.showError(mod, err)
		})
	}
}

var (
	actionCauseModuleDisabled      = e.NewActionCause("module disabled")
	actionCauseModuleToggledByBind = e.NewActionCause("module toggled by bind")
)
