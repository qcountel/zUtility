package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/qcountel/zUtility/app/module"
	"github.com/qcountel/zUtility/internal/pkg/fyneutil"
	win "github.com/qcountel/zUtility/pkg/winlegacy"
	"github.com/qcountel/zUtility/pkg/embeddable"
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

	modules     []module.Module
	modulesMu   sync.Mutex
	autoClicker *autoClicker

	minimizeToTray     bool
	animationsEnabled  bool
	useClonedMinecraft bool
	showConsole        bool
	showSettings       bool
	startedAt          time.Time
	updatePromptShown  atomic.Bool

	updateMu    sync.RWMutex
	updateState UpdateStatus

	// activeTab — индекс активной вкладки в боковом меню
	// 0="Модули", 1="Конфигурация", 2="Ресурспаки", 3=Настройки, 4="Кликер"
	activeTab int

	installedView       fyne.CanvasObject
	refreshInstalled    func()
	installedPacksDirty bool
	loadingConfig       atomic.Bool
}

func (app *App) init(proc *win.Process) ([]module.Module, error) {
	app.winMu.Lock()
	defer app.winMu.Unlock()
	a := fyneapp.New()
	fyneutil.AccentColor = accentRed
	a.Settings().SetTheme(&fyneutil.CrimsonDarkTheme{})
	app.app = a

	app.win = a.NewWindow(Name)
	app.win.SetMaster()
	app.win.CenterOnScreen()
	app.win.Resize(fyne.NewSize(520, 720))
	app.win.SetFixedSize(true)

	var needRestore int32 // atomic flag: 1 = был свёрнут/потерял фокус

	a.Lifecycle().SetOnExitedForeground(func() {
		atomic.StoreInt32(&needRestore, 1)
	})

	a.Lifecycle().SetOnEnteredForeground(func() {
		if !atomic.CompareAndSwapInt32(&needRestore, 1, 0) {
			return // флаг не стоял — первый запуск или повторный вызов
		}
		fyne.Do(func() {
			if app.win == nil {
				return
			}
			sz := app.win.Canvas().Size()
			app.win.Resize(fyne.NewSize(sz.Width+1, sz.Height)) // ломаем кэш canvas.size
			app.win.Resize(sz)                                  // возвращаем точный размер
			app.win.Canvas().Refresh(app.win.Content())
		})
	})

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
		fyne.NewMenuItem("Показать", func() {
			app.win.Show()
			app.win.Content().Refresh()
		}),
		fyne.NewMenuItem("Скрыть", func() {
			app.win.Hide()
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Выход", func() {
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
	if app.startedAt.IsZero() {
		app.startedAt = time.Now()
	}


	if err := app.PreloadTheme(); err != nil {
		app.conf.Logger.Error("failed to preload theme", "err", err)
	}

	// init создаёт все модули и первоначальный UI.
	modules, err := app.init(app.tr.Process())
	if err != nil {
		return fmt.Errorf("init: %w", err)
	}
	app.modulesMu.Lock()
	app.modules = modules
	app.modulesMu.Unlock()

	// Defaults before LoadConfig — animations are on by default (bool zero-value is false).
	app.animationsEnabled = true
	AnimationsEnabled = true
	if err := app.LoadConfig(); err != nil {
		app.conf.Logger.Error("failed to load config", "err", err)
	}
	app.StartUpdateCheck(func(status UpdateStatus) {
		if !status.HasUpdate || status.Checking || status.Downloading || status.Error != "" {
			return
		}
		if !app.updatePromptShown.CompareAndSwap(false, true) {
			return
		}
		fyne.Do(func() {
			app.showStartupUpdatePrompt(status)
		})
	})
	app.updateTrayMenu()

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
	if app.autoClicker != nil {
		app.autoClicker.Close()
	}
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

func (app *App) showStartupUpdatePrompt(status UpdateStatus) {
	if app.win == nil {
		return
	}
	message := widget.NewLabel(fmt.Sprintf("Доступна новая версия %s.\nСкачать и заменить текущую утилиту сейчас?", status.LatestVersion))
	message.Wrapping = fyne.TextWrapWord
	dialog.ShowCustomConfirm(
		"Доступно обновление",
		"Скачать",
		"Пропустить",
		message,
		func(confirm bool) {
			if !confirm {
				return
			}
			app.DownloadLatestReleaseAsset(nil)
		},
		app.win,
	)
}

