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
	skyboxSigStr   = "0F 85 ? ? ? ? 48 8D 54 24 30 E8 ? ? ? ? 90 48 8B 5C 24 30"
	skyboxPatchLen = 6
)

var (
	skyboxPatch = mem.NopBytes(skyboxPatchLen)
)

var _ module.Module = (*skybox)(nil)

type Skybox struct {
	Process     *win.Process
	Error       func(error)
	AfterChange func()
}

func (conf Skybox) Create() module.Module {
	return &skybox{
		process:     conf.Process,
		errFn:       conf.Error,
		afterChange: conf.AfterChange,
	}
}

type skybox struct {
	mu          sync.Mutex
	process     *win.Process
	errFn       func(error)
	afterChange func()

	wantEnabled   bool
	toggler       *win.ByteToggler
	originalKnown bool
	uiToggle      *modulesutil.M3Toggle
}

func (*skybox) Name() string { return "Skybox" }

func (*skybox) Description() string {
	return "Скайбокс из ресурспаков (требуется перезаход в игру после применения ресурпака)."
}

func (s *skybox) CreateObjects() []fyne.CanvasObject {
	s.mu.Lock()
	initEnabled := s.wantEnabled
	s.mu.Unlock()

	toggle := modulesutil.NewM3Toggle(initEnabled)
	toggle.OnChange = func(enabled bool) {
		s.mu.Lock()
		s.wantEnabled = enabled
		s.mu.Unlock()

		toggler, err := s.lazyToggler()
		if err != nil {
			if s.errFn != nil {
				s.errFn(fmt.Errorf("skybox: init toggler: %w", err))
			}
			return
		}

		s.mu.Lock()
		knownOrig := s.originalKnown
		s.mu.Unlock()

		if !enabled && !knownOrig {
			if s.errFn != nil {
				s.errFn(errors.New("skybox: cannot disable, original bytes unknown"))
			}
			s.mu.Lock()
			s.wantEnabled = true
			s.mu.Unlock()
			if s.uiToggle != nil {
				prev := s.uiToggle.OnChange
				s.uiToggle.OnChange = nil
				s.uiToggle.SetChecked(true)
				s.uiToggle.OnChange = prev
			}
			return
		}
		if err := toggler.Set(enabled); err != nil {
			if s.errFn != nil {
				s.errFn(fmt.Errorf("skybox: update toggler: %w", err))
			}
			return
		}
		if s.afterChange != nil {
			s.afterChange()
		}
	}
	s.uiToggle = toggle
	return []fyne.CanvasObject{toggle}
}

func (s *skybox) Enable() {
	s.mu.Lock()
	if s.wantEnabled {
		s.mu.Unlock()
		return
	}
	s.wantEnabled = true
	s.mu.Unlock()

	go func() {
		toggler, err := s.lazyToggler()
		if err != nil {
			if s.errFn != nil {
				s.errFn(fmt.Errorf("skybox: enable: %w", err))
			}
		} else {
			_ = toggler.Set(true)
		}
	}()

	if s.uiToggle != nil {
		prev := s.uiToggle.OnChange
		s.uiToggle.OnChange = nil
		s.uiToggle.SetChecked(true)
		s.uiToggle.OnChange = prev
	}
}

func (s *skybox) Disable() {
	s.mu.Lock()
	s.wantEnabled = false
	knownOrig := s.originalKnown
	toggler := s.toggler
	s.mu.Unlock()

	if toggler != nil && toggler.Enabled() {
		if knownOrig {
			_ = toggler.Set(false)
		} else if s.errFn != nil {
			s.errFn(errors.New("skybox: cannot disable, original bytes unknown"))
			s.mu.Lock()
			s.wantEnabled = true
			s.mu.Unlock()
			if s.uiToggle != nil {
				prev := s.uiToggle.OnChange
				s.uiToggle.OnChange = nil
				s.uiToggle.SetChecked(true)
				s.uiToggle.OnChange = prev
			}
			return
		}
	}
	if s.uiToggle != nil {
		prev := s.uiToggle.OnChange
		s.uiToggle.OnChange = nil
		s.uiToggle.SetChecked(false)
		s.uiToggle.OnChange = prev
	}
}

func (s *skybox) IsEnabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.wantEnabled
}

func (s *skybox) lazyToggler() (*win.ByteToggler, error) {
	s.mu.Lock()
	tog := s.toggler
	s.mu.Unlock()

	if tog != nil {
		return tog, nil
	}

	sig, err := mem.ParseSignature(skyboxSigStr)
	if err != nil {
		return nil, fmt.Errorf("skybox: parse signature: %w", err)
	}

	addr, err := mem.ScanSignature(s.process, sig, win.GetModuleInfoOptions{Name: s.process.Name})
	patched := false
	if err != nil {
		patchedSig := sig.ExtendFirst(mem.NopBytes(skyboxPatchLen))
		addr, err = mem.ScanSignature(s.process, patchedSig, win.GetModuleInfoOptions{Name: s.process.Name})
		if err != nil {
			return nil, fmt.Errorf("skybox: signature not found: %w", err)
		}
		patched = true
	}

	original, err := mem.ReadBytes(s.process, addr, skyboxPatchLen)
	if err != nil {
		return nil, fmt.Errorf("read original/patched bytes: %w", err)
	}

	originalKnown := true
	if patched {
		originalBytes := make([]byte, skyboxPatchLen)
		for idx := 0; idx < skyboxPatchLen; idx++ {
			if sig.Mask[idx] != 'x' {
				originalKnown = false
				originalBytes = original
				break
			}
			originalBytes[idx] = sig.Data[idx]
		}
		original = originalBytes
		if originalKnown {
			originalKnown = !bytes.Equal(original, skyboxPatch)
		}
	}

	t := &win.ByteToggler{
		Process:  s.process,
		Address:  addr,
		Original: original,
		Patch:    skyboxPatch,
	}

	if patched || bytes.Equal(original, skyboxPatch) {
		t.SetState(true)
	}

	s.mu.Lock()
	s.toggler = t
	s.originalKnown = originalKnown
	s.mu.Unlock()

	return t, nil
}
