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
	noCamResetSigStr   = "FF 90 ? ? ? ? ? ? ? 48 8B D6 44 8B 4C 24"
	noCamResetPatchLen = 2
)

var (
	noCamResetPatch = mem.NopBytes(noCamResetPatchLen)
)

var _ module.Module = (*noCamReset)(nil)

type NoCamReset struct {
	Process     *win.Process
	Error       func(error)
	AfterChange func()
}

func (conf NoCamReset) Create() module.Module {
	return &noCamReset{
		process:     conf.Process,
		errFn:       conf.Error,
		afterChange: conf.AfterChange,
	}
}

type noCamReset struct {
	mu          sync.Mutex
	process     *win.Process
	errFn       func(error)
	afterChange func()

	wantEnabled   bool
	toggler       *win.ByteToggler
	originalKnown bool
	uiToggle      *modulesutil.M3Toggle
}

func (*noCamReset) Name() string { return "NoCamReset" }
func (*noCamReset) Description() string {
	return "Отключает сброс камеры при телепорте."
}

func (n *noCamReset) CreateObjects() []fyne.CanvasObject {
	n.mu.Lock()
	initEnabled := n.wantEnabled
	n.mu.Unlock()

	toggle := modulesutil.NewM3Toggle(initEnabled)
	toggle.OnChange = func(b bool) {
		n.mu.Lock()
		n.wantEnabled = b
		n.mu.Unlock()

		toggler, err := n.lazyToggler()
		if err != nil {
			if n.errFn != nil {
				n.errFn(fmt.Errorf("no_cam_reset: init toggler: %w", err))
			}
			return
		}
		n.mu.Lock()
		knownOrig := n.originalKnown
		n.mu.Unlock()
		if !b && !knownOrig {
			if n.errFn != nil {
				n.errFn(errors.New("no_cam_reset: cannot disable, original bytes unknown"))
			}
			n.mu.Lock()
			n.wantEnabled = true
			n.mu.Unlock()
			if n.uiToggle != nil {
				prev := n.uiToggle.OnChange
				n.uiToggle.OnChange = nil
				n.uiToggle.SetChecked(true)
				n.uiToggle.OnChange = prev
			}
			return
		}
		if err := toggler.Set(b); err != nil {
			if n.errFn != nil {
				n.errFn(fmt.Errorf("no_cam_reset: update toggler: %w", err))
			}
			return
		}
		if n.afterChange != nil {
			n.afterChange()
		}
	}
	n.uiToggle = toggle
	return []fyne.CanvasObject{toggle}
}

func (n *noCamReset) Enable() {
	n.mu.Lock()
	if n.wantEnabled {
		n.mu.Unlock()
		return
	}
	n.wantEnabled = true
	n.mu.Unlock()

	go func() {
		toggler, err := n.lazyToggler()
		if err != nil {
			if n.errFn != nil {
				n.errFn(fmt.Errorf("no_cam_reset: enable: %w", err))
			}
		} else {
			_ = toggler.Set(true)
		}
	}()

	if n.uiToggle != nil {
		prev := n.uiToggle.OnChange
		n.uiToggle.OnChange = nil
		n.uiToggle.SetChecked(true)
		n.uiToggle.OnChange = prev
	}
}

func (n *noCamReset) Disable() {
	n.mu.Lock()
	n.wantEnabled = false
	knownOrig := n.originalKnown
	toggler := n.toggler
	n.mu.Unlock()

	if toggler != nil && toggler.Enabled() {
		if knownOrig {
			_ = toggler.Set(false)
		} else if n.errFn != nil {
			n.errFn(errors.New("no_cam_reset: cannot disable, original bytes unknown"))
			n.mu.Lock()
			n.wantEnabled = true
			n.mu.Unlock()
			if n.uiToggle != nil {
				prev := n.uiToggle.OnChange
				n.uiToggle.OnChange = nil
				n.uiToggle.SetChecked(true)
				n.uiToggle.OnChange = prev
			}
			return
		}
	}
	if n.uiToggle != nil {
		prev := n.uiToggle.OnChange
		n.uiToggle.OnChange = nil
		n.uiToggle.SetChecked(false)
		n.uiToggle.OnChange = prev
	}
}

func (n *noCamReset) IsEnabled() bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.wantEnabled
}

func (n *noCamReset) lazyToggler() (*win.ByteToggler, error) {
	n.mu.Lock()
	tog := n.toggler
	n.mu.Unlock()

	if tog != nil {
		return tog, nil
	}

	sig, err := mem.ParseSignature(noCamResetSigStr)
	if err != nil {
		return nil, fmt.Errorf("no_cam_reset: parse signature: %w", err)
	}

	addr, err := mem.ScanSignature(n.process, sig, win.GetModuleInfoOptions{Name: n.process.Name})
	patched := false
	if err != nil {
		patchedSig := sig.ExtendFirst(mem.NopBytes(noCamResetPatchLen))
		addr, err = mem.ScanSignature(n.process, patchedSig, win.GetModuleInfoOptions{Name: n.process.Name})
		if err != nil {
			return nil, fmt.Errorf("no_cam_reset: signature not found: %w", err)
		}
		patched = true
	}

	original, err := mem.ReadBytes(n.process, addr, noCamResetPatchLen)
	if err != nil {
		return nil, fmt.Errorf("read original/patched bytes: %w", err)
	}

	originalKnown := true
	if patched {
		originalBytes := make([]byte, noCamResetPatchLen)
		for i := 0; i < noCamResetPatchLen; i++ {
			if sig.Mask[i] != 'x' {
				originalKnown = false
				originalBytes = original
				break
			}
			originalBytes[i] = sig.Data[i]
		}
		original = originalBytes
		if originalKnown {
			originalKnown = !bytes.Equal(original, noCamResetPatch)
		}
	}

	t := &win.ByteToggler{
		Process:  n.process,
		Address:  addr,
		Original: original,
		Patch:    noCamResetPatch,
	}

	if patched || bytes.Equal(original, noCamResetPatch) {
		t.SetState(true)
	}

	n.mu.Lock()
	n.toggler = t
	n.originalKnown = originalKnown
	n.mu.Unlock()

	return t, nil
}
