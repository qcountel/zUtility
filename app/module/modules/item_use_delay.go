package modules

import (
	"bytes"
	"errors"
	"fmt"
	"sync"

	"fyne.io/fyne/v2"
	"github.com/qcountel/zUtility/app/module"
	"github.com/qcountel/zUtility/app/module/modules/modulesutil"
	win "github.com/qcountel/zUtility/pkg/winlegacy"
	"github.com/qcountel/zUtility/pkg/win/mem"
)

const (
	itemUseDelaySigStr   = "48 89 86 ? ? ? ? 48 83 7E ? 00"
	itemUseDelayPatchLen = 7
)

var (
	itemUseDelayPatch = mem.NopBytes(itemUseDelayPatchLen)
)

var _ module.Module = (*itemUseDelay)(nil)

type ItemUseDelay struct {
	Process     *win.Process
	Error       func(error)
	AfterChange func()
}

func (conf ItemUseDelay) Create() module.Module {
	return &itemUseDelay{
		process:     conf.Process,
		errFn:       conf.Error,
		afterChange: conf.AfterChange,
	}
}

type itemUseDelay struct {
	mu          sync.Mutex
	process     *win.Process
	errFn       func(error)
	afterChange func()

	wantEnabled   bool
	toggler       *win.ByteToggler
	originalKnown bool
	uiToggle      *modulesutil.M3Toggle
}

func (*itemUseDelay) Name() string { return "ItemUseDelay" }

func (*itemUseDelay) Description() string {
	return "Убирает задержку использования хотбара."
}

func (i *itemUseDelay) CreateObjects() []fyne.CanvasObject {
	i.mu.Lock()
	initEnabled := i.wantEnabled
	i.mu.Unlock()

	toggle := modulesutil.NewM3Toggle(initEnabled)
	toggle.OnChange = func(enabled bool) {
		i.mu.Lock()
		i.wantEnabled = enabled
		i.mu.Unlock()

		toggler, err := i.lazyToggler()
		if err != nil {
			if i.errFn != nil {
				i.errFn(fmt.Errorf("item_use_delay: init toggler: %w", err))
			}
			return
		}

		i.mu.Lock()
		knownOrig := i.originalKnown
		i.mu.Unlock()

		if !enabled && !knownOrig {
			if i.errFn != nil {
				i.errFn(errors.New("item_use_delay: cannot disable, original bytes unknown"))
			}
			i.mu.Lock()
			i.wantEnabled = true
			i.mu.Unlock()
			if i.uiToggle != nil {
				prev := i.uiToggle.OnChange
				i.uiToggle.OnChange = nil
				i.uiToggle.SetChecked(true)
				i.uiToggle.OnChange = prev
			}
			return
		}
		if err := toggler.Set(enabled); err != nil {
			if i.errFn != nil {
				i.errFn(fmt.Errorf("item_use_delay: update toggler: %w", err))
			}
			return
		}
		if i.afterChange != nil {
			i.afterChange()
		}
	}
	i.uiToggle = toggle
	return []fyne.CanvasObject{toggle}
}

func (i *itemUseDelay) Enable() {
	i.mu.Lock()
	if i.wantEnabled {
		i.mu.Unlock()
		return
	}
	i.wantEnabled = true
	i.mu.Unlock()

	go func() {
		toggler, err := i.lazyToggler()
		if err != nil {
			if i.errFn != nil {
				i.errFn(fmt.Errorf("item_use_delay: enable: %w", err))
			}
		} else {
			_ = toggler.Set(true)
		}
	}()

	if i.uiToggle != nil {
		prev := i.uiToggle.OnChange
		i.uiToggle.OnChange = nil
		i.uiToggle.SetChecked(true)
		i.uiToggle.OnChange = prev
	}
}

func (i *itemUseDelay) Disable() {
	i.mu.Lock()
	i.wantEnabled = false
	knownOrig := i.originalKnown
	toggler := i.toggler
	i.mu.Unlock()

	if toggler != nil && toggler.Enabled() {
		if knownOrig {
			_ = toggler.Set(false)
		} else if i.errFn != nil {
			i.errFn(errors.New("item_use_delay: cannot disable, original bytes unknown"))
			i.mu.Lock()
			i.wantEnabled = true
			i.mu.Unlock()
			if i.uiToggle != nil {
				prev := i.uiToggle.OnChange
				i.uiToggle.OnChange = nil
				i.uiToggle.SetChecked(true)
				i.uiToggle.OnChange = prev
			}
			return
		}
	}
	if i.uiToggle != nil {
		prev := i.uiToggle.OnChange
		i.uiToggle.OnChange = nil
		i.uiToggle.SetChecked(false)
		i.uiToggle.OnChange = prev
	}
}

func (i *itemUseDelay) IsEnabled() bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.wantEnabled
}

func (i *itemUseDelay) lazyToggler() (*win.ByteToggler, error) {
	i.mu.Lock()
	tog := i.toggler
	i.mu.Unlock()

	if tog != nil {
		return tog, nil
	}

	sig, err := mem.ParseSignature(itemUseDelaySigStr)
	if err != nil {
		return nil, fmt.Errorf("item_use_delay: parse signature: %w", err)
	}

	addr, err := mem.ScanSignature(i.process, sig, win.GetModuleInfoOptions{Name: i.process.Name})
	patched := false
	if err != nil {
		patchedSig := sig.ExtendFirst(mem.NopBytes(itemUseDelayPatchLen))
		addr, err = mem.ScanSignature(i.process, patchedSig, win.GetModuleInfoOptions{Name: i.process.Name})
		if err != nil {
			return nil, fmt.Errorf("item_use_delay: signature not found: %w", err)
		}
		patched = true
	}

	original, err := mem.ReadBytes(i.process, addr, itemUseDelayPatchLen)
	if err != nil {
		return nil, fmt.Errorf("read original/patched bytes: %w", err)
	}

	originalKnown := true
	if patched {
		originalBytes := make([]byte, itemUseDelayPatchLen)
		for idx := 0; idx < itemUseDelayPatchLen; idx++ {
			if sig.Mask[idx] != 'x' {
				originalKnown = false
				originalBytes = original
				break
			}
			originalBytes[idx] = sig.Data[idx]
		}
		original = originalBytes
		if originalKnown {
			originalKnown = !bytes.Equal(original, itemUseDelayPatch)
		}
	}

	t := &win.ByteToggler{
		Process:  i.process,
		Address:  addr,
		Original: original,
		Patch:    itemUseDelayPatch,
	}

	if patched || bytes.Equal(original, itemUseDelayPatch) {
		t.SetState(true)
	}

	i.mu.Lock()
	i.toggler = t
	i.originalKnown = originalKnown
	i.mu.Unlock()

	return t, nil
}
