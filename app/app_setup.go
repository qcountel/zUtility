package app

import (
	"fmt"

	"github.com/elliotchance/orderedmap/v3"
	"github.com/something-that-is-cool/zutil/app/module"
	"github.com/something-that-is-cool/zutil/pkg/e"
	"go.uber.org/multierr"
)

type modulesMap = orderedmap.OrderedMap[module.Config, module.Module]

var actionCauseModuleCreated = e.NewActionCause("module created (init)")

func (app *App) createModulesFromConfigs(configs []module.Config) (*modulesMap, bool, error) {
	type toCreateModule struct {
		Config   module.Config
		Property module.Property
	}
	modules := orderedmap.NewOrderedMap[module.Config, module.Module]()
	toCreate := make([]toCreateModule, 0, len(configs))

	func() {
		app.userConf.Lock()
		defer app.userConf.Unlock()
		// uConf must not be nil here because it is init func
		for _, conf := range configs {
			property := app.mustExtractPropertyUnsafe(conf)
			toCreate = append(toCreate, toCreateModule{Config: conf, Property: property})
		}
	}()
	// creating all modules within of the uConfMu !!!
	var (
		ok       bool
		multiErr error
	)
	for _, m := range toCreate {
		mod, err := m.Config.Create(m.Property, actionCauseModuleCreated)
		if err != nil {
			multiErr = multierr.Append(multiErr, fmt.Errorf("create %q: %w", m.Config.Identifier(), err))
			continue
		}
		ok = true // at least one module created
		modules.Set(m.Config, mod)
	}
	return modules, ok, multiErr
}

func (app *App) mustExtractPropertyUnsafe(conf module.Config) module.Property {
	v, ok := app.userConf.V.Modules[conf.Identifier()]
	if ok {
		return v
	}
	property := conf.DefaultProperty()
	app.userConf.V.Modules[conf.Identifier()] = property
	return property
}
