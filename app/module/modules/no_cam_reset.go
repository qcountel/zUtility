package modules

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"github.com/something-that-is-cool/zutil/app/module"
	"github.com/something-that-is-cool/zutil/app/module/modules/modulesutil"
	"github.com/something-that-is-cool/zutil/internal/pkg/win"
	w "golang.org/x/sys/windows"
)

const (
	noCamResetSigStr   = "FF 90 ? ? ? ? ? ? ? 48 8B D6 44 8B 4C 24"
	noCamResetPatchLen = 6
)

var (
	noCamResetPattern, noCamResetMask = mustParseSignatureMask(noCamResetSigStr)
	noCamResetPatch                   = win.NopSig(noCamResetPatchLen)
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
	toggle := modulesutil.NewM3Toggle(n.wantEnabled)
	toggle.OnChange = func(b bool) {
		n.wantEnabled = b
		toggler, err := n.lazyToggler()
		if err != nil {
			if n.errFn != nil {
				n.errFn(fmt.Errorf("no_cam_reset: init toggler: %w", err))
			}
			return
		}
		if !b && !n.originalKnown {
			if n.errFn != nil {
				n.errFn(errors.New("no_cam_reset: cannot disable, original bytes unknown"))
			}
			if n.uiToggle != nil {
				prev := n.uiToggle.OnChange
				n.uiToggle.OnChange = nil
				n.uiToggle.SetChecked(true)
				n.uiToggle.OnChange = prev
			}
			n.wantEnabled = true
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
	n.wantEnabled = true
	toggler, err := n.lazyToggler()
	if err != nil {
		if n.errFn != nil {
			n.errFn(fmt.Errorf("no_cam_reset: enable: %w", err))
		}
	} else {
		_ = toggler.Set(true)
	}
	if n.uiToggle != nil {
		prev := n.uiToggle.OnChange
		n.uiToggle.OnChange = nil
		n.uiToggle.SetChecked(true)
		n.uiToggle.OnChange = prev
	}
}

func (n *noCamReset) Disable() {
	n.wantEnabled = false
	if n.toggler != nil && n.toggler.Enabled() {
		if n.originalKnown {
			_ = n.toggler.Set(false)
		} else if n.errFn != nil {
			n.errFn(errors.New("no_cam_reset: cannot disable, original bytes unknown"))
			n.wantEnabled = true
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

func (n *noCamReset) IsEnabled() bool { return n.wantEnabled }

func (n *noCamReset) lazyToggler() (*win.ByteToggler, error) {
	n.mu.Lock()
	if n.toggler != nil {
		t := n.toggler
		n.mu.Unlock()
		return t, nil
	}
	n.mu.Unlock()

	addr, patched, err := findNoCamResetAddr(n.process)
	if err != nil {
		return nil, err
	}
	original, ok := originalBytesFromSignature(noCamResetPattern, noCamResetMask, len(noCamResetPatch))
	if patched {
		if !ok {
			n.originalKnown = false
			original, err = readBytes(n.process, addr, len(noCamResetPatch))
			if err != nil {
				return nil, fmt.Errorf("read patched bytes: %w", err)
			}
		} else {
			n.originalKnown = true
		}
	} else {
		original, err = readBytes(n.process, addr, len(noCamResetPatch))
		if err != nil {
			return nil, fmt.Errorf("read original bytes: %w", err)
		}
		n.originalKnown = true
	}

	t := &win.ByteToggler{
		Process:  n.process,
		Address:  addr,
		Original: original,
		Patch:    noCamResetPatch,
	}

	if patched || equalBytes(original, noCamResetPatch) {
		t.SetState(true)
	}

	n.mu.Lock()
	n.toggler = t
	n.mu.Unlock()
	return t, nil
}

func findNoCamResetAddr(p *win.Process) (uintptr, bool, error) {
	addr, err := scanSignatureMasked(p, p.Module, p.ModuleSize, noCamResetPattern, noCamResetMask)
	if err == nil {
		return addr, false, nil
	}

	patchedPattern := make([]byte, len(noCamResetPattern))
	patchedMask := make([]bool, len(noCamResetMask))
	copy(patchedPattern, noCamResetPattern)
	copy(patchedMask, noCamResetMask)
	for i := 0; i < noCamResetPatchLen && i < len(patchedPattern); i++ {
		patchedPattern[i] = 0x90
		patchedMask[i] = true
	}

	addr, err = scanSignatureMasked(p, p.Module, p.ModuleSize, patchedPattern, patchedMask)
	if err == nil {
		return addr, true, nil
	}
	return 0, false, err
}

func scanSignatureMasked(p *win.Process, base, size uintptr, pattern []byte, mask []bool) (uintptr, error) {
	if len(pattern) == 0 {
		return 0, errors.New("empty pattern")
	}
	if len(pattern) != len(mask) {
		return 0, errors.New("pattern/mask length mismatch")
	}
	moduleData := make([]byte, size)
	var bytesRead uintptr

	err := w.ReadProcessMemory(p.Handle, base, &moduleData[0], size, &bytesRead)
	if err != nil && bytesRead == 0 {
		return 0, fmt.Errorf("read: %w", err)
	}

	max := int(bytesRead) - len(pattern)
	for i := 0; i <= max; i++ {
		match := true
		for j := 0; j < len(pattern); j++ {
			if mask[j] && moduleData[i+j] != pattern[j] {
				match = false
				break
			}
		}
		if match {
			return base + uintptr(i), nil
		}
	}
	return 0, errors.New("signature not found")
}

func readBytes(p *win.Process, addr uintptr, n int) ([]byte, error) {
	if n <= 0 {
		return nil, errors.New("invalid length")
	}
	buf := make([]byte, n)
	if err := w.ReadProcessMemory(p.Handle, addr, &buf[0], uintptr(n), nil); err != nil {
		return nil, err
	}
	return buf, nil
}

func parseSignatureMask(sig string) ([]byte, []bool, error) {
	fields := strings.Fields(sig)
	if len(fields) == 0 {
		return nil, nil, errors.New("empty signature")
	}
	pattern := make([]byte, len(fields))
	mask := make([]bool, len(fields))
	for i, f := range fields {
		if f == "?" || f == "??" || strings.Contains(f, "?") {
			pattern[i] = 0
			mask[i] = false
			continue
		}
		v, err := strconv.ParseUint(f, 16, 8)
		if err != nil {
			return nil, nil, fmt.Errorf("parse hex byte %q: %w", f, err)
		}
		pattern[i] = byte(v)
		mask[i] = true
	}
	return pattern, mask, nil
}

func mustParseSignatureMask(sig string) ([]byte, []bool) {
	p, m, err := parseSignatureMask(sig)
	if err != nil {
		panic(err)
	}
	return p, m
}

func originalBytesFromSignature(pattern []byte, mask []bool, n int) ([]byte, bool) {
	if n <= 0 || n > len(pattern) || len(pattern) != len(mask) {
		return nil, false
	}
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		if !mask[i] {
			return nil, false
		}
		out[i] = pattern[i]
	}
	return out, true
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
