package modules

import (
	"context"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"github.com/qcountel/zUtility/app/module"
	"github.com/qcountel/zUtility/app/module/modules/modulesutil"
	win "github.com/qcountel/zUtility/pkg/winlegacy"
	w "golang.org/x/sys/windows"
)

var _ module.Module = (*quickSlots)(nil)

var (
	user32DLL          = w.NewLazySystemDLL("user32.dll")
	procGetAsyncKeyState = user32DLL.NewProc("GetAsyncKeyState")
	procPostMessageW   = user32DLL.NewProc("PostMessageW")
)

const (
	wmKeydown   = 0x0100
	wmKeyup     = 0x0101
	targetProc  = "Minecraft.Windows.exe"
)

type QuickSlots struct {
	Process     *win.Process
	Error       func(error)
	AfterChange func()
}

func (conf QuickSlots) Create() module.Module {
	return &quickSlots{
		process:     conf.Process,
		errFn:       conf.Error,
		afterChange: conf.AfterChange,
	}
}

type quickSlots struct {
	mu          sync.Mutex
	process     *win.Process
	errFn       func(error)
	afterChange func()

	enabled  bool
	cancel   context.CancelFunc
	uiToggle *modulesutil.M3Toggle
}

func (*quickSlots) Name() string { return "QuickSlots" }
func (*quickSlots) Description() string {
	return "Быстрое переключение слотов хотбара."
}

func (q *quickSlots) CreateObjects() []fyne.CanvasObject {
	q.mu.Lock()
	initEnabled := q.enabled
	q.mu.Unlock()

	toggle := modulesutil.NewM3Toggle(initEnabled)
	toggle.OnChange = func(b bool) {
		if b {
			q.Enable()
		} else {
			q.Disable()
		}
		if q.afterChange != nil {
			q.afterChange()
		}
	}
	q.uiToggle = toggle
	return []fyne.CanvasObject{toggle}
}

func (q *quickSlots) Enable() {
	q.mu.Lock()
	if q.enabled {
		q.mu.Unlock()
		return
	}
	q.enabled = true
	ctx, cancel := context.WithCancel(context.Background())
	q.cancel = cancel
	q.mu.Unlock()

	go q.loop(ctx)

	if q.uiToggle != nil {
		prev := q.uiToggle.OnChange
		q.uiToggle.OnChange = nil
		q.uiToggle.SetChecked(true)
		q.uiToggle.OnChange = prev
	}
}

func (q *quickSlots) Disable() {
	q.mu.Lock()
	q.enabled = false
	cancel := q.cancel
	q.cancel = nil
	q.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	if q.uiToggle != nil {
		prev := q.uiToggle.OnChange
		q.uiToggle.OnChange = nil
		q.uiToggle.SetChecked(false)
		q.uiToggle.OnChange = prev
	}
}

func (q *quickSlots) IsEnabled() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.enabled
}

func (q *quickSlots) loop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	var wasDown [9]bool

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		if !win.ForegroundIs(targetProc) {
			for i := range wasDown {
				wasDown[i] = false
			}
			continue
		}

		hwnd, _, _ := user32DLL.NewProc("GetForegroundWindow").Call()
		if hwnd == 0 {
			continue
		}

		// Отслеживаем клавиши '1' - '9' (VK_1 = 0x31 ... VK_9 = 0x39)
		for i := 0; i < 9; i++ {
			vk := uintptr(0x31 + i)
			r, _, _ := procGetAsyncKeyState.Call(vk)
			isDown := (r & 0x8000) != 0

			if isDown && !wasDown[i] {
				wasDown[i] = true
				// Мгновенно отправляем WM_KEYDOWN без задержки обработки
				_, _, _ = procPostMessageW.Call(hwnd, wmKeydown, vk, 0)
			} else if !isDown {
				wasDown[i] = false
			}
		}
	}
}
