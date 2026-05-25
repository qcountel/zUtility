//go:build windows

package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	acUser32DLL   = windows.NewLazySystemDLL("user32.dll")
	acProcMouseEv = acUser32DLL.NewProc("mouse_event")
	acProcSetHook = acUser32DLL.NewProc("SetWindowsHookExW")
	acProcNext    = acUser32DLL.NewProc("CallNextHookEx")
	acProcUnhook  = acUser32DLL.NewProc("UnhookWindowsHookEx")
	acProcGetMsg  = acUser32DLL.NewProc("GetMessageW")
	acProcGetKey  = acUser32DLL.NewProc("GetAsyncKeyState")

	acWinmmDLL      = windows.NewLazySystemDLL("winmm.dll")
	acProcBeginTick = acWinmmDLL.NewProc("timeBeginPeriod")
	acProcEndTick   = acWinmmDLL.NewProc("timeEndPeriod")
)

const (
	acMouseLeftDown  = uintptr(0x0002)
	acMouseLeftUp    = uintptr(0x0004)
	acMouseRightDown = uintptr(0x0008)
	acMouseRightUp   = uintptr(0x0010)

	acWHMouseLL     = uintptr(14)
	acWMLButtonDown = uintptr(0x0201)
	acWMLButtonUp   = uintptr(0x0202)
	acWMRButtonDown = uintptr(0x0204)
	acWMRButtonUp   = uintptr(0x0205)
	acLLMHFInjected = uint32(0x00000001)

	acDefaultHotkey  = uint32(0x75)
	acPollInterval   = 30 * time.Millisecond
	acIdleSleep      = 5 * time.Millisecond
	acCaptureTimeout = 8 * time.Second
)

type clickMode int

const (
	clickLeft clickMode = iota
	clickRight
)

type mouseHookData struct {
	ptX, ptY    int32
	mouseData   uint32
	flags       uint32
	time        uint32
	dwExtraInfo uintptr
}

type winMsg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	ptX     int32
	ptY     int32
}

type persistedAutoClickerConfig struct {
	Enabled       bool    `json:"enabled"`
	MinCPS        float64 `json:"min_cps"`
	MaxCPS        float64 `json:"max_cps"`
	Mode          int     `json:"mode"`
	BindVK        uint32  `json:"bind_vk"`
	JitterEnabled bool    `json:"jitter_enabled"`
}

type autoClicker struct {
	mu sync.Mutex

	enabled       bool
	minCPS        float64
	maxCPS        float64
	mode          clickMode
	bindVK        uint32
	jitterEnabled bool

	clickCancel context.CancelFunc
	hotkeyStop  chan struct{}

	hookOnce sync.Once
	hookH    uintptr

	capturing atomic.Bool
	physLMB   atomic.Bool
	physRMB   atomic.Bool

	onEnabledChange func(bool)
}

func (app *App) getAutoClicker() *autoClicker {
	app.modulesMu.Lock()
	defer app.modulesMu.Unlock()
	if app.autoClicker != nil {
		return app.autoClicker
	}
	ac := newAutoClicker()
	if cfg, err := loadAutoClickerConfig(); err == nil {
		ac.SetRange(cfg.MinCPS, cfg.MaxCPS)
		ac.SetMode(clickMode(cfg.Mode))
		ac.SetBindVK(cfg.BindVK)
		ac.SetJitterEnabled(cfg.JitterEnabled)
		ac.SetEnabled(cfg.Enabled)
	} else {
		app.conf.Logger.Warn("failed to load autoclicker config", "err", err)
	}
	app.autoClicker = ac
	return ac
}

func newAutoClicker() *autoClicker {
	ac := &autoClicker{minCPS: 5, maxCPS: 15, mode: clickLeft, bindVK: acDefaultHotkey}
	ac.startHook()
	ac.startHotkeyLoop()
	return ac
}

func (ac *autoClicker) Close() {
	ac.SetEnabled(false)
	ac.mu.Lock()
	stop := ac.hotkeyStop
	ac.hotkeyStop = nil
	ac.mu.Unlock()
	if stop != nil {
		close(stop)
	}
}

func (ac *autoClicker) SetEnabled(v bool) {
	ac.mu.Lock()
	if ac.enabled == v {
		ac.mu.Unlock()
		return
	}
	ac.enabled = v
	ac.mu.Unlock()
	if v {
		ac.startClickLoop()
	} else {
		ac.stopClickLoop()
	}
	if ac.onEnabledChange != nil {
		ac.onEnabledChange(v)
	}
}

func (ac *autoClicker) Enabled() bool {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	return ac.enabled
}

func (ac *autoClicker) ToggleEnabled() { ac.SetEnabled(!ac.Enabled()) }

func (ac *autoClicker) SetRange(minCPS, maxCPS float64) {
	if minCPS < 1 {
		minCPS = 1
	}
	if maxCPS < minCPS {
		maxCPS = minCPS
	}
	ac.mu.Lock()
	ac.minCPS = minCPS
	ac.maxCPS = maxCPS
	ac.mu.Unlock()
}

func (ac *autoClicker) Range() (float64, float64) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	return ac.minCPS, ac.maxCPS
}

func (ac *autoClicker) SetJitterEnabled(v bool) {
	ac.mu.Lock()
	ac.jitterEnabled = v
	ac.mu.Unlock()
}

func (ac *autoClicker) JitterEnabled() bool {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	return ac.jitterEnabled
}

func (ac *autoClicker) SetMode(mode clickMode) {
	ac.mu.Lock()
	ac.mode = mode
	ac.mu.Unlock()
}

func (ac *autoClicker) Mode() clickMode {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	return ac.mode
}

func (ac *autoClicker) SetBindVK(vk uint32) {
	ac.mu.Lock()
	ac.bindVK = vk
	ac.mu.Unlock()
}

func (ac *autoClicker) BindVK() uint32 {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	return ac.bindVK
}

func (ac *autoClicker) startHook() {
	ac.hookOnce.Do(func() {
		ready := make(chan struct{})
		go func() {
			runtime.LockOSThread()
			cb := windows.NewCallback(func(nCode, wParam, lParam uintptr) uintptr {
				if int32(nCode) >= 0 {
					info := (*mouseHookData)(unsafe.Pointer(lParam))
					if info.flags&acLLMHFInjected == 0 {
						switch wParam {
						case acWMLButtonDown:
							ac.physLMB.Store(true)
						case acWMLButtonUp:
							ac.physLMB.Store(false)
						case acWMRButtonDown:
							ac.physRMB.Store(true)
						case acWMRButtonUp:
							ac.physRMB.Store(false)
						}
					}
				}
				r, _, _ := acProcNext.Call(ac.hookH, nCode, wParam, lParam)
				return r
			})
			h, _, _ := acProcSetHook.Call(acWHMouseLL, cb, 0, 0)
			ac.mu.Lock()
			ac.hookH = h
			ac.mu.Unlock()
			close(ready)
			var msg winMsg
			for {
				r, _, _ := acProcGetMsg.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
				if r == 0 || r == ^uintptr(0) {
					break
				}
			}
			if h != 0 {
				acProcUnhook.Call(h)
			}
		}()
		<-ready
	})
}

func (ac *autoClicker) isPhysicalHeld(mode clickMode) bool {
	if mode == clickRight {
		return ac.physRMB.Load()
	}
	return ac.physLMB.Load()
}

func (ac *autoClicker) startClickLoop() {
	ac.mu.Lock()
	if ac.clickCancel != nil {
		ac.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	ac.clickCancel = cancel
	ac.mu.Unlock()
	go ac.clickLoop(ctx)
}

func (ac *autoClicker) stopClickLoop() {
	ac.mu.Lock()
	cancel := ac.clickCancel
	ac.clickCancel = nil
	ac.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (ac *autoClicker) clickLoop(ctx context.Context) {
	acProcBeginTick.Call(1)
	defer acProcEndTick.Call(1)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	var oscHigh bool
	var windowEnd time.Time
	var targetCPS int
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		ac.mu.Lock()
		enabled := ac.enabled
		minCPS := ac.minCPS
		maxCPS := ac.maxCPS
		mode := ac.mode
		ac.mu.Unlock()
		if !enabled {
			return
		}
		if !ac.isPhysicalHeld(mode) {
			time.Sleep(acIdleSleep)
			windowEnd = time.Time{}
			continue
		}
		minInt := int(minCPS + 0.5)
		maxInt := int(maxCPS + 0.5)
		if minInt < 1 {
			minInt = 1
		}
		if maxInt < minInt {
			maxInt = minInt
		}
		now := time.Now()
		if windowEnd.IsZero() || now.After(windowEnd) || targetCPS < minInt || targetCPS > maxInt {
			if maxInt == minInt {
				targetCPS = minInt
			} else if oscHigh {
				targetCPS = minInt
				oscHigh = false
			} else {
				targetCPS = maxInt
				oscHigh = true
			}
			windowEnd = now.Add(time.Duration(700+rng.Intn(600)) * time.Millisecond)
		}
		interval := time.Second / time.Duration(targetCPS)
		if ac.JitterEnabled() {
			// Apply a strong dynamic 22% Jitter to simulate human variance and bypass anticheat
			jitter := time.Duration(float64(interval) * 0.22 * (rng.Float64()*2 - 1))
			interval += jitter
		} else {
			// Apply a minimal 2% micro-variance for natural Windows thread/mouse execution
			jitter := time.Duration(float64(interval) * 0.02 * (rng.Float64()*2 - 1))
			interval += jitter
		}
		if interval < time.Millisecond {
			interval = time.Millisecond
		}
		hold := 8 * time.Millisecond
		if interval <= hold+time.Millisecond {
			hold = interval / 2
			if hold < time.Millisecond {
				hold = time.Millisecond
			}
		}
		pause := interval - hold
		if pause < time.Millisecond {
			pause = time.Millisecond
		}
		switch mode {
		case clickLeft:
			acProcMouseEv.Call(acMouseLeftDown, 0, 0, 0, 0)
			time.Sleep(hold)
			acProcMouseEv.Call(acMouseLeftUp, 0, 0, 0, 0)
		case clickRight:
			acProcMouseEv.Call(acMouseRightDown, 0, 0, 0, 0)
			time.Sleep(hold)
			acProcMouseEv.Call(acMouseRightUp, 0, 0, 0, 0)
		}
		time.Sleep(pause)
	}
}

func (ac *autoClicker) startHotkeyLoop() {
	ac.mu.Lock()
	if ac.hotkeyStop != nil {
		ac.mu.Unlock()
		return
	}
	stop := make(chan struct{})
	ac.hotkeyStop = stop
	ac.mu.Unlock()
	go func() {
		ticker := time.NewTicker(acPollInterval)
		defer ticker.Stop()
		var wasDown bool
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
			}
			if ac.capturing.Load() {
				wasDown = false
				continue
			}
			vk := ac.BindVK()
			if vk == 0 {
				wasDown = false
				continue
			}
			r, _, _ := acProcGetKey.Call(uintptr(vk))
			down := r&0x8000 != 0
			if down && !wasDown {
				wasDown = true
				ac.ToggleEnabled()
			} else if !down {
				wasDown = false
			}
		}
	}()
}

func (ac *autoClicker) CaptureBind(ctx context.Context) (uint32, bool) {
	ac.capturing.Store(true)
	defer ac.capturing.Store(false)
	time.Sleep(200 * time.Millisecond)
	ticker := time.NewTicker(16 * time.Millisecond)
	defer ticker.Stop()
	skip := map[uint32]bool{0x1B: true, 0x5B: true, 0x5C: true, 0x5D: true}
	for {
		select {
		case <-ctx.Done():
			return 0, false
		case <-ticker.C:
		}
		for vk := uint32(1); vk <= 0xFE; vk++ {
			if skip[vk] {
				continue
			}
			r, _, _ := acProcGetKey.Call(uintptr(vk))
			if r&0x8000 == 0 {
				continue
			}
			for {
				r2, _, _ := acProcGetKey.Call(uintptr(vk))
				if r2&0x8000 == 0 {
					break
				}
				time.Sleep(16 * time.Millisecond)
			}
			return vk, true
		}
	}
}

func withAutoClickerCaptureTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), acCaptureTimeout)
}

func autoClickerConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".zutil")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "autoclicker.json"), nil
}

func loadAutoClickerConfig() (persistedAutoClickerConfig, error) {
	cfg := persistedAutoClickerConfig{MinCPS: 5, MaxCPS: 15, Mode: int(clickLeft), BindVK: acDefaultHotkey, JitterEnabled: false}
	path, err := autoClickerConfigPath()
	if err != nil {
		return cfg, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if cfg.MinCPS < 1 {
		cfg.MinCPS = 1
	}
	if cfg.MaxCPS < cfg.MinCPS {
		cfg.MaxCPS = cfg.MinCPS
	}
	if cfg.BindVK == 0 {
		cfg.BindVK = acDefaultHotkey
	}
	if cfg.Mode < int(clickLeft) || cfg.Mode > int(clickRight) {
		cfg.Mode = int(clickLeft)
	}
	return cfg, nil
}

func saveAutoClickerConfig(cfg persistedAutoClickerConfig) error {
	path, err := autoClickerConfigPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func persistAutoClicker(ac *autoClicker) error {
	min, max := ac.Range()
	return saveAutoClickerConfig(persistedAutoClickerConfig{
		Enabled:       ac.Enabled(),
		MinCPS:        min,
		MaxCPS:        max,
		Mode:          int(ac.Mode()),
		BindVK:        ac.BindVK(),
		JitterEnabled: ac.JitterEnabled(),
	})
}

func autoClickerVKName(vk uint32) string {
	if vk == 0 {
		return "None"
	}
	if name, ok := autoClickerVKNames[vk]; ok {
		return name
	}
	return fmt.Sprintf("VK_%02X", vk)
}

var autoClickerVKNames = map[uint32]string{
	0x01: "LMB", 0x02: "RMB", 0x04: "MMB",
	0x08: "Backspace", 0x09: "Tab", 0x0D: "Enter",
	0x10: "Shift", 0x11: "Ctrl", 0x12: "Alt",
	0x14: "CapsLock", 0x20: "Space",
	0x21: "PageUp", 0x22: "PageDown", 0x23: "End", 0x24: "Home",
	0x25: "Left", 0x26: "Up", 0x27: "Right", 0x28: "Down",
	0x2D: "Insert", 0x2E: "Delete",
	0x70: "F1", 0x71: "F2", 0x72: "F3", 0x73: "F4",
	0x74: "F5", 0x75: "F6", 0x76: "F7", 0x77: "F8",
	0x78: "F9", 0x79: "F10", 0x7A: "F11", 0x7B: "F12",
	0xA0: "LShift", 0xA1: "RShift", 0xA2: "LCtrl", 0xA3: "RCtrl",
	0xA4: "LAlt", 0xA5: "RAlt",
}

func init() {
	for vk := uint32(0x30); vk <= 0x39; vk++ {
		autoClickerVKNames[vk] = string(rune('0' + vk - 0x30))
	}
	for vk := uint32(0x41); vk <= 0x5A; vk++ {
		autoClickerVKNames[vk] = string(rune('A' + vk - 0x41))
	}
}
