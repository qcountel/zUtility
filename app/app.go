package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
	"os"
	_ "image/png"
	"image"
	"gioui.org/op/paint"

	w "golang.org/x/sys/windows"

	gioapp "gioui.org/app"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/something-that-is-cool/zutil/app/module"
	"github.com/something-that-is-cool/zutil/internal/misc"
	"github.com/something-that-is-cool/zutil/pkg/e"
	"github.com/something-that-is-cool/zutil/pkg/win"
	"github.com/something-that-is-cool/zutil/pkg/win/hotkey"
)

const Name = "zutil"

var ActionCauseUserInput = e.NewActionCause("user input")

type App struct {
	ctx    context.Context
	cancel context.CancelFunc

	conf Config

	wg sync.WaitGroup

	tr *win.ProcessTracker
	hm *hotkey.Manager

	closed atomic.Bool

	win *gioapp.Window
	ops op.Ops

	userConf misc.ValueWithMutex[*UserConfig]

	data misc.ValueWithMutex[struct {
		started, init          bool
		uConfInit, modulesInit bool
		modules                *modulesMap
	}]

	// Gio UI states
	listState  layout.List
	lightTheme bool
	activeTab  int // 0: modules, 1: config, 2: packs, 3: clicker, 4: settings

	useClonedMinecraft bool

	// Tab navigation clicks
	tabClickModules  widget.Clickable
	tabClickConfig   widget.Clickable
	tabClickSettings widget.Clickable
	playClick        widget.Clickable

	// UI States for modules
	moduleToggleStates   map[string]*widget.Bool
	moduleSliderStates   map[string]*widget.Float
	moduleSliderInputs   map[string]*widget.Editor
	moduleSettingsClicks map[string]*widget.Clickable
	moduleCardClicks     map[string]*widget.Clickable

	// Main Settings Click States
	aboutClick          widget.Clickable
	toggleThemeClick    widget.Clickable
	importConfigClick   widget.Clickable
	exportConfigClick   widget.Clickable
	resetConfigClick    widget.Clickable
	showErrorsCheck     widget.Bool
	showErrorsClick     widget.Clickable

	// Overlay states (modal dialogs in Gio)
	showModuleOverlay   bool
	activeOverlayModule module.Module
	activeOverlayConfig module.Config
	closeOverlayClick   widget.Clickable
	bindButtonClick     widget.Clickable

	showBindOverlay      bool
	activeBindModule     module.Module
	activeBindModuleConf module.Config
	closeBindClick       widget.Clickable
	resetBindClick       widget.Clickable
	charBindClicks       map[string]*widget.Clickable
	bindListState        layout.List

	// Alerts
	alertMessage string
	alertTitle   string
	alertShow    bool
	alertClick   widget.Clickable

	// Cached UI static data
	cachedModules []moduleEntry
	iconSettings  *widget.Icon
	iconInfo      *widget.Icon
	iconPlay      *widget.Icon

	blockerClick        widget.Clickable

	tabAnimProgress      float32

	appIconOp     paint.ImageOp
	appIconLoaded bool

	// Color presets clicks
	presetClicks [6]widget.Clickable
}

func (app *App) initUnsafe(proc *win.Process) (err error) {
	if app.data.V.init {
		return e.ErrAlreadyInitialized
	}
	app.data.V.init = true

	// Load app icon
	if f, err := os.Open("app/icon.png"); err == nil {
		if img, _, err := image.Decode(f); err == nil {
			app.appIconOp = paint.NewImageOp(img)
			app.appIconLoaded = true
		}
		f.Close()
	}

	app.initStates()

	app.userConf.Lock()
	app.userConf.V, err = app.loadUserConfigUnsafe()
	if err != nil {
		app.userConf.Unlock()
		return fmt.Errorf("load user config: %w", err)
	}
	app.data.V.uConfInit = true
	app.lightTheme = app.userConf.V.LightTheme
	app.useClonedMinecraft = app.userConf.V.UseClonedMinecraft

	showErrors := app.userConf.V.ShowErrors
	app.userConf.Unlock()

	app.conf.Logger.Debug("creating module configs...")
	configs := app.setupModules(proc)
	if len(configs) == 0 {
		return errors.New("no modules created")
	}
	app.conf.Logger.Debug("created module configs.")

	app.conf.Logger.Debug("creating modules from configs...")
	modules, ok, err := app.createModulesFromConfigs(configs)
	if !ok && err != nil {
		return fmt.Errorf("create modules from configs: %w", err)
	}
	if err != nil {
		app.conf.Logger.Error("error creating modules from configs", "err", err.Error())
		if showErrors {
			app.showError("create module(s) from config(s)", err)
		}
	} else {
		app.conf.Logger.Debug("created modules from configs.")
	}
	app.data.V.modules = modules
	app.data.V.modulesInit = true

	app.initCachedModules(modules)

	app.userConf.Lock()
	app.hm = hotkey.ManagerConfig{Handlers: app.loadBinds(app.userConf.V)}.New()
	app.userConf.Unlock()

	return nil
}

func (app *App) initStates() {
	app.listState.Axis = layout.Vertical
	app.bindListState.Axis = layout.Vertical
	app.moduleToggleStates = make(map[string]*widget.Bool)
	app.moduleSliderStates = make(map[string]*widget.Float)
	app.moduleSliderInputs = make(map[string]*widget.Editor)
	app.moduleSettingsClicks = make(map[string]*widget.Clickable)
	app.moduleCardClicks = make(map[string]*widget.Clickable)
	app.charBindClicks = make(map[string]*widget.Clickable)
}

const windowWidth, windowHeight = 450, 650

func (app *App) deployGio() {
	app.win = &gioapp.Window{}
	app.win.Option(
		gioapp.Title(Name),
		gioapp.MinSize(unit.Dp(windowWidth), unit.Dp(windowHeight)),
		gioapp.MaxSize(unit.Dp(windowWidth), unit.Dp(windowHeight)),
	)
}

// Run ...
func (app *App) Run() error {
	if app.closed.Load() {
		return e.ErrClosed
	}
	done, err := app.run()
	if err != nil {
		return err
	}

	app.conf.Logger.Info("running window...")

	app.applyWindowsDarkMode()

	th := material.NewTheme()

	for {
		winEv := app.win.Event()
		switch ev := winEv.(type) {
		case gioapp.DestroyEvent:
			app.conf.Logger.Info("window closed via UI")
			close(done)
			if err := app.Close(); err != nil && !errors.Is(err, e.ErrAlreadyClosed) {
				app.conf.Logger.Error("close app via ui", "err", err.Error())
			}
			return ev.Err
		case gioapp.FrameEvent:
			gtx := gioapp.NewContext(&app.ops, ev)
			app.layout(gtx, th)
			ev.Frame(gtx.Ops)
		}
	}
}

func (app *App) run() (chan struct{}, error) {
	app.data.Lock()
	defer app.data.Unlock()

	if app.data.V.started {
		return nil, e.ErrAlreadyRunning
	}
	app.conf.Logger.Info("initializing...")
	if err := app.initUnsafe(app.tr.Process()); err != nil {
		return nil, fmt.Errorf("init: %w", err)
	}
	app.conf.Logger.Info("initialized.")

	done := make(chan struct{})
	go func() {
		select {
		case <-app.ctx.Done():
			if err := app.close(closeCauseContextClosed); err != nil && !errors.Is(err, e.ErrAlreadyClosed) {
				app.conf.Logger.Error("close app", "err", err.Error())
			}
		case <-done:
		}
	}()
	app.runBackgroundTasks()
	app.data.V.started = true
	return done, nil
}

func (app *App) runBackgroundTasks() {
	go func() {
		if err := app.tr.Run(app.ctx); err != nil && !errors.Is(err, context.Canceled) {
			app.conf.Logger.Error("process tracker error", "err", err)
		}
	}()
	app.wg.Go(func() {
		if err := app.hm.Run(app.ctx); err != nil && !errors.Is(err, context.Canceled) {
			app.conf.Logger.Error("hotkey manager error", "err", err)
		}
	})
}

// Close implements io.Closer.
func (app *App) Close() error {
	return app.close(nil, true)
}

var (
	closeCauseTrackerClosed = e.NewCloseCauseString("tracker closed")
	closeCauseContextClosed = e.NewCloseCause(context.Canceled)
)

func (app *App) close(cause e.CloseCause, main ...bool) (multi error) {
	if err := app.closeLogic(cause); err != nil && errors.Is(err, e.ErrAlreadyClosed) {
		return err
	}
	app.conf.Logger.Debug("closing window...", "main", misc.HasTrueOption(main))
	app.win.Perform(system.ActionClose)
	return nil
}

func (app *App) closeLogic(cause e.CloseCause) error {
	if !app.closed.CompareAndSwap(false, true) {
		return e.ErrAlreadyClosed
	}
	start := time.Now()
	if cause == nil {
		cause = e.CloseCauseExternal
	}
	app.conf.Logger.Info("closing app logic...", "cause", cause.Error())
	defer func() {
		app.conf.Logger.Info("closed app logic.", "elapsed", time.Since(start).String())
	}()
	app.closeIfStarted(cause)
	app.conf.Logger.Debug("waiting for waitgroup end...")
	app.wg.Wait()
	app.conf.Logger.Debug("waitgroup ended.")
	return nil
}

func (app *App) closeIfStarted(cause e.CloseCause) {
	app.data.Lock()
	defer app.data.Unlock()



	defer func() {
		app.cancel()
		app.doClose("process tracker", app.tr.CloseWithProcess)
	}()
	if app.data.V.uConfInit {
		app.saveUserConfig()
	}
	if app.data.V.modulesInit && !e.CloseCauseIs(cause, closeCauseTrackerClosed) {
		app.disableModulesUnsafe()
	}
}

func (app *App) disableModulesUnsafe() {
	app.conf.Logger.Debug("disabling modules...")
	defer app.conf.Logger.Debug("disabled modules.")

	for _, m := range app.data.V.modules.AllFromFront() {
		m.Disable(actionCauseModuleDisabled)
		app.conf.Logger.Debug("disabled module.", "module", m.Name())
	}
}

func (app *App) doClose(src string, fn func() error) {
	if err := fn(); err != nil {
		app.conf.Logger.Error("an error occurred when closing", "src", src, "err", err.Error())
	}
}

func (app *App) moduleByIDUnsafe(id string) (module.Module, bool) {
	for conf, m := range app.data.V.modules.AllFromFront() {
		if conf.Identifier() == id {
			return m, true
		}
	}
	return nil, false
}

func (app *App) doModuleUpdates(toEdit map[module.Module]module.Property, cause e.ActionCause) {
	app.conf.Logger.Debug("updating modules state...", "cause", cause.String())
	defer app.conf.Logger.Debug("updated modules state.")

	for m, p := range toEdit {
		m.Edit(p, cause)
	}
}

func (app *App) showError(src string, err error) {
	app.alertTitle = "Error: " + src
	app.alertMessage = err.Error()
	app.alertShow = true
	if app.win != nil {
		app.win.Invalidate()
	}
}

func (app *App) showInfo(title, msg string) {
	app.alertTitle = title
	app.alertMessage = msg
	app.alertShow = true
	if app.win != nil {
		app.win.Invalidate()
	}
}

var (
	modDwmapi                  = w.NewLazySystemDLL("dwmapi.dll")
	procDwmSetWindowAttribute = modDwmapi.NewProc("DwmSetWindowAttribute")
	modUser32                  = w.NewLazySystemDLL("user32.dll")
	procFindWindowW            = modUser32.NewProc("FindWindowW")
)

const DWMWA_USE_IMMERSIVE_DARK_MODE = 20

func (app *App) applyWindowsDarkMode() {
	go func() {
		titlePtr, _ := w.UTF16PtrFromString(Name)
		// Try multiple times to find the window as it takes a brief moment to initialize
		for i := 0; i < 50; i++ {
			time.Sleep(50 * time.Millisecond)
			hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(titlePtr)))
			if hwnd != 0 {
				var val int32 = 1 // default dark mode enabled
				if app.lightTheme {
					val = 0
				}
				procDwmSetWindowAttribute.Call(
					hwnd,
					DWMWA_USE_IMMERSIVE_DARK_MODE,
					uintptr(unsafe.Pointer(&val)),
					unsafe.Sizeof(val),
				)
				break
			}
		}
	}()
}

func (app *App) setCustomSkyPreset(r, g, b float64) {
	app.data.Lock()
	defer app.data.Unlock()

	// Need to import modulesutil in app.go if not imported, let's check
	// Wait, we can just use type assertion:
	if mr, ok := app.moduleByIDUnsafe("custom_sky_r"); ok {
		if valMod, ok2 := mr.(interface {
			SetValue(v float64, cause e.ActionCause, opts ...any) error
		}); ok2 {
			_ = valMod.SetValue(r, ActionCauseUserInput)
		}
		if slider, exists := app.moduleSliderStates["custom_sky_r"]; exists {
			slider.Value = float32(r)
		}
	}
	if mg, ok := app.moduleByIDUnsafe("custom_sky_g"); ok {
		if valMod, ok2 := mg.(interface {
			SetValue(v float64, cause e.ActionCause, opts ...any) error
		}); ok2 {
			_ = valMod.SetValue(g, ActionCauseUserInput)
		}
		if slider, exists := app.moduleSliderStates["custom_sky_g"]; exists {
			slider.Value = float32(g)
		}
	}
	if mb, ok := app.moduleByIDUnsafe("custom_sky_b"); ok {
		if valMod, ok2 := mb.(interface {
			SetValue(v float64, cause e.ActionCause, opts ...any) error
		}); ok2 {
			_ = valMod.SetValue(b, ActionCauseUserInput)
		}
		if slider, exists := app.moduleSliderStates["custom_sky_b"]; exists {
			slider.Value = float32(b)
		}
	}
}

