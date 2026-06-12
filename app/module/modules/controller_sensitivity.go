package modules

import (
	_ "embed"
	"fmt"
	"math"

	"github.com/something-that-is-cool/zutil/app/module"
	"github.com/something-that-is-cool/zutil/app/module/modules/modulesutil"
	"github.com/something-that-is-cool/zutil/pkg/e"
	"github.com/something-that-is-cool/zutil/pkg/win"
	"github.com/something-that-is-cool/zutil/pkg/win/mem"
)

var controllerSensitivityPtr = mem.MustParsePointer(
	"019209F0",
	"10 8 8 8 28 B0 68 14",
)

var _ module.Config = (*ControllerSensitivity)(nil)

type ControllerSensitivity struct {
	Process        *win.Process
	Error          func(error)
	OnValueChanged func(float64, e.ActionCause)
}

func (conf *ControllerSensitivity) Create(p module.Property, cause e.ActionCause) (module.Module, error) {
	fc := &modulesutil.Float32Module{
		Process:        conf.Process,
		Error:          conf.Error,
		OnValueChanged: conf.OnValueChanged,
		Min:            1,
		Max:            300,
		Default:        100,
		SliderToMemory: func(f float64) float32 {
			return float32(math.Ceil(f)) / 100
		},
		MemoryToSlider: func(f float32) float64 {
			return math.Ceil(float64(f) * 100)
		},
		ResolveAddress: func() (uintptr, error) {
			return mem.ResolvePointerAddress(conf.Process, controllerSensitivityPtr)
		},
		ErrorOnInitialRead: true,
	}
	f, err := fc.New(cause)
	if err != nil {
		return nil, fmt.Errorf("create float ptr module: %w", err)
	}
	if cause == nil {
		cause = e.ActionCauseExternal
	}
	m := modulesutil.NewBaseValue(f,
		"controller sensitivity",
		"allows to modify controller sensitivity to values higher than 100",
	)
	m.Edit(p, cause)
	return m, nil
}

// DefaultProperty ...
func (*ControllerSensitivity) DefaultProperty() module.Property {
	return module.Property{
		Enabled: false,
		Value:   100.0,
	}
}

// Identifier ...
func (*ControllerSensitivity) Identifier() string {
	return "controller_sensitivity"
}
