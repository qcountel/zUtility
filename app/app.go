package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"github.com/something-that-is-cool/zutil/app/module"
	"github.com/something-that-is-cool/zutil/internal/pkg/fyneutil"
	"github.com/something-that-is-cool/zutil/internal/pkg/win"
	"github.com/something-that-is-cool/zutil/pkg/embeddable"
)

const Name = "zUtility"

type App struct {
	ctx    context.Context
	cancel context.CancelFunc

	conf Config

	app fyne.App

	win   fyne.Window
	winMu sync.Mutex

	wg sync.WaitGroup

	tr *win.ProcessTracker

	closed, started atomic.Bool

	modules   []module.Module
	modulesMu sync.Mutex

	minimizeToTray bool
	showSettings   bool
	showHotkey     uint32
	language       string

	// activeTab — индекс активной вкладки в боковом меню (0=Модули, 1=Конфигурация, 2=Настройки, 3=Ресурспаки)
	activeTab int
}

func (app *App) init(proc *win.Process) ([]module.Module, error) {
	app.winMu.Lock()
	defer app.winMu.Unlock()
	app.language = normalizeLanguage(app.language)

	a := fyneapp.New()
	a.Settings().SetTheme(&fyneutil.CrimsonDarkTheme{})
	app.app = a

	app.win = a.NewWindow(Name)
	app.win.SetMaster()
	app.win.CenterOnScreen()
	app.win.Resize(fyne.NewSize(520, 720))
	app.win.SetFixedSize(true)

	if icon, err := embeddable.LoadIcon(); err == nil {
		app.app.SetIcon(icon)
		app.win.SetIcon(icon)
	}

	app.win.SetCloseIntercept(func() {

		if app.minimizeToTray {
			app.win.Hide()
		} else {

			if err := app.SaveConfig(); err != nil {
				app.conf.Logger.Error("failed to save config on close", "err", err)
			}
			_ = app.Close(true)
		}
	})

	app.updateTrayMenu()

	c, modules, err := app.createContent(proc)
	if err != nil {
		return nil, fmt.Errorf("create content: %w", err)
	}
	app.win.SetContent(c)
	return modules, nil
}

func (app *App) updateTrayMenu() {
	desk, ok := app.app.(interface{ SetSystemTrayMenu(*fyne.Menu) })
	if !ok {
		return
	}
	menu := fyne.NewMenu("zUtility",
		fyne.NewMenuItem(app.t("Показать", "Show"), func() {
			app.win.Show()
			app.win.Resize(fyne.NewSize(520, 720))
		}),
		fyne.NewMenuItem(app.t("Скрыть", "Hide"), func() {
			app.win.Hide()
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem(app.t("Выход", "Exit"), func() {
			if err := app.SaveConfig(); err != nil {
				app.conf.Logger.Error("failed to save config on exit", "err", err)
			}
			_ = app.Close(true)
		}),
	)
	desk.SetSystemTrayMenu(menu)
}

var ErrAppClosed = errors.New("app closed")
var ErrAlreadyRunning = errors.New("app is already running")

func (app *App) Run() error {
	if app.closed.Load() {
		return ErrAppClosed
	}
	if !app.started.CompareAndSwap(false, true) {
		return ErrAlreadyRunning
	}
	modules, err := app.init(app.tr.Process())
	if err != nil {
		return fmt.Errorf("init: %w", err)
	}
	app.modulesMu.Lock()
	app.modules = modules
	app.modulesMu.Unlock()

	if err := app.LoadConfig(); err != nil {
		app.conf.Logger.Error("failed to load config", "err", err)
	}
	app.updateTrayMenu()
	if app.win != nil {
		c, _, _ := app.createContent(app.tr.Process())
		app.win.SetContent(c)
		app.win.Resize(fyne.NewSize(520, 720))
	}

	go func() {
		<-app.ctx.Done()
		if err := app.Close(false); err != nil && !errors.Is(err, ErrAppClosed) {
			app.conf.Logger.Error("close app", "err", err.Error())
		}
	}()
	go func() {
		defer app.tr.Close()
		if err := app.tr.Run(app.ctx); err != nil {
			_ = app.Close(false)
		}
	}()
	go app.runHotkeyListener(app.ctx)
	app.win.ShowAndRun()
	return nil
}

func (app *App) Close(main bool) error {
	if !app.closed.CompareAndSwap(false, true) {
		return ErrAppClosed
	}
	func() {
		app.modulesMu.Lock()
		defer app.modulesMu.Unlock()
		for _, m := range app.modules {
			m.Disable()
		}
	}()
	app.cancel()
	app.tr.Close()

	app.wg.Wait()
	if !main {
		fyne.DoAndWait(app.closeWin)
	} else {
		app.closeWin()
	}
	return nil
}

func (app *App) closeWin() {
	app.winMu.Lock()
	defer app.winMu.Unlock()

	if app.win != nil {
		app.win.Close()
	}
}
