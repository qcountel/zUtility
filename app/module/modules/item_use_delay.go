package modules

import (
	"errors"
	"fmt"
	"sync"

	"fyne.io/fyne/v2"
	"github.com/something-that-is-cool/zutil/app/module"
	"github.com/something-that-is-cool/zutil/app/module/modules/modulesutil"
	"github.com/something-that-is-cool/zutil/internal/pkg/win"
)

const (
	itemUseDelaySigStr   = "48 89 86 ? ? ? ? 48 83 7E ? 00"
	itemUseDelayPatchLen = 7
)

var (
	itemUseDelayPattern, itemUseDelayMask = mustParseSignatureMask(itemUseDelaySigStr)
	itemUseDelayPatch                     = win.NopSig(itemUseDelayPatchLen)
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
	i.wantEnabled = true
	i.mu.Unlock()

	toggler, err := i.lazyToggler()
	if err != nil {
		if i.errFn != nil {
			i.errFn(fmt.Errorf("item_use_delay: enable: %w", err))
		}
	} else {
		_ = toggler.Set(true)
	}
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
	defer i.mu.Unlock()

	if i.toggler != nil {
		return i.toggler, nil
	}

	addr, patched, err := findItemUseDelayAddr(i.process)
	if err != nil {
		return nil, err
	}

	original, ok := originalBytesFromSignature(itemUseDelayPattern, itemUseDelayMask, len(itemUseDelayPatch))
	if patched {
		if !ok {
			i.originalKnown = false
			original, err = readBytes(i.process, addr, len(itemUseDelayPatch))
			if err != nil {
				return nil, fmt.Errorf("read patched bytes: %w", err)
			}
		} else {
			i.originalKnown = true
		}
	} else {
		original, err = readBytes(i.process, addr, len(itemUseDelayPatch))
		if err != nil {
			return nil, fmt.Errorf("read original bytes: %w", err)
		}
		i.originalKnown = true
	}

	t := &win.ByteToggler{
		Process:  i.process,
		Address:  addr,
		Original: original,
		Patch:    itemUseDelayPatch,
	}

	if patched || equalBytes(original, itemUseDelayPatch) {
		t.SetState(true)
	}

	i.toggler = t
	return t, nil
}

func findItemUseDelayAddr(p *win.Process) (uintptr, bool, error) {
	addr, err := scanSignatureMaskedUnique(p, p.Module, p.ModuleSize, itemUseDelayPattern, itemUseDelayMask)
	if err == nil {
		return addr, false, nil
	}

	patchedPattern := make([]byte, len(itemUseDelayPattern))
	patchedMask := make([]bool, len(itemUseDelayMask))
	copy(patchedPattern, itemUseDelayPattern)
	copy(patchedMask, itemUseDelayMask)

	for i := 0; i < itemUseDelayPatchLen && i < len(patchedPattern); i++ {
		patchedPattern[i] = 0x90
		patchedMask[i] = true
	}

	addr, err = scanSignatureMaskedUnique(p, p.Module, p.ModuleSize, patchedPattern, patchedMask)
	if err == nil {
		return addr, true, nil
	}
	return 0, false, err
}

func scanSignatureMaskedUnique(p *win.Process, base, size uintptr, pattern []byte, mask []bool) (uintptr, error) {
	first, err := scanSignatureMasked(p, base, size, pattern, mask)
	if err != nil {
		return 0, err
	}

	offset := first - base + 1
	if offset >= size {
		return first, nil
	}

	if _, err := scanSignatureMasked(p, base+offset, size-offset, pattern, mask); err == nil {
		return 0, errors.New("signature is ambiguous: multiple matches")
	}
	return first, nil
}
