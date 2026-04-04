package app

import (
	"image/color"
	"math"
	"os/exec"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/something-that-is-cool/zutil/app/module"
	"github.com/something-that-is-cool/zutil/app/module/modules"
	"github.com/something-that-is-cool/zutil/app/module/modules/modulesutil"
	"github.com/something-that-is-cool/zutil/internal/pkg/win"
)

var (
	// Цветовая схема как в HTML-прототипе
	bgPrimary  = chex(0x0F, 0x0F, 0x12) // основной фон #0f0f12
	bgSidebar  = chex(0x11, 0x11, 0x15) // сайдбар #111115
	bgCard     = color.NRGBA{R: 0x18, G: 0x18, B: 0x1E, A: 0xFF} // карточки тёмно-синеватые
	bgElevated = chex(0x22, 0x22, 0x22) // приподнятые элементы

	// Красные акценты
	accentRed       = chex(0xCC, 0x33, 0x33)
	accentRedBright = chex(0xFF, 0x55, 0x55)
	accentRedDim    = chex(0x7A, 0x1A, 0x1A)
	accentOrange    = chex(0xDD, 0x77, 0x22)
	accentGreen     = chex(0x33, 0xBB, 0x55)

	// Текст
	textPrimary   = chex(0xE8, 0xE8, 0xE8)
	textSecondary = chex(0x88, 0x88, 0x88)
	textDim       = chex(0x50, 0x50, 0x50)

	// Границы
	borderSubtle = chex(0x2A, 0x2A, 0x2A)
	borderRed    = color.NRGBA{R: 0xCC, G: 0x33, B: 0x33, A: 0x20}
)

func chex(r, g, b uint8) color.NRGBA {
	return color.NRGBA{R: r, G: g, B: b, A: 0xff}
}

func calpha(c color.NRGBA, a uint8) color.NRGBA {
	return color.NRGBA{R: c.R, G: c.G, B: c.B, A: a}
}


func (app *App) createContent(proc *win.Process) (fyne.CanvasObject, []module.Module, error) {
	app.modulesMu.Lock()
	existingMods := app.modules
	app.modulesMu.Unlock()

	var mods []module.Module
	if len(existingMods) > 0 {
		mods = existingMods
	} else {
		noDynFovMod := app.createNoDynamicFovModule(proc)
		mods = []module.Module{
			app.createAutoSprintModule(proc),
			app.createNoHurtCamModule(proc),
			app.createNoCamResetModule(proc),
			noDynFovMod,
			app.createNoParticleModule(proc),
			app.createNoVsyncModule(proc),
			app.createControllerSensitivityModule(proc),
			app.createZoomV2Module(proc, noDynFovMod),
			app.createTimeModule(proc),
			app.createItemUseDelayModule(proc),
			app.createRainModule(proc),
		}
	}

	bg := canvas.NewRectangle(bgPrimary)

	var contentStack *fyne.Container

	// Оверлей для анимации смены вкладок (только над контентом, не над сайдбаром)
	tabFade := canvas.NewRectangle(color.NRGBA{R: 0x0F, G: 0x0F, B: 0x12, A: 0x00})
	tabFade.Hide()

	// Защита от параллельных анимаций при быстром переключении вкладок
	var tabAnimating atomic.Bool

	sidebar, contentArea := app.buildSidebarWithContent(mods, proc, func(tab int) {
		if contentStack == nil {
			return
		}
		// Если анимация уже идёт — пропускаем, чтобы не было гонки goroutine
		if !tabAnimating.CompareAndSwap(false, true) {
			return
		}
		app.activeTab = tab
		go func(switchTab int) {
			defer tabAnimating.Store(false)
			// --- Fade OUT ---
			fyne.Do(func() {
				tabFade.FillColor = color.NRGBA{R: 0x0F, G: 0x0F, B: 0x12, A: 0x00}
				tabFade.Show()
				tabFade.Refresh()
			})
			const durOut = 70 * time.Millisecond
			t1 := time.NewTicker(14 * time.Millisecond)
			start1 := time.Now()
			for range t1.C {
				p := math.Min(1.0, float64(time.Since(start1))/float64(durOut))
				a := uint8(210 * p)
				fyne.Do(func() {
					tabFade.FillColor = color.NRGBA{R: 0x0F, G: 0x0F, B: 0x12, A: a}
					tabFade.Refresh()
				})
				if p >= 1 {
					break
				}
			}
			t1.Stop()

			// --- Switch content ---
			newContent := app.buildTabContent(switchTab, mods)
			fyne.Do(func() {
				if len(contentStack.Objects) > 0 {
					contentStack.Objects[0] = newContent
				} else {
					contentStack.Add(newContent)
				}
				contentStack.Refresh()
			})
			time.Sleep(16 * time.Millisecond)

			// --- Fade IN ---
			const durIn = 140 * time.Millisecond
			t2 := time.NewTicker(14 * time.Millisecond)
			start2 := time.Now()
			for range t2.C {
				p := math.Min(1.0, float64(time.Since(start2))/float64(durIn))
				ease := 1 - math.Pow(1-p, 3)
				a := uint8(210 * (1 - ease))
				fyne.Do(func() {
					tabFade.FillColor = color.NRGBA{R: 0x0F, G: 0x0F, B: 0x12, A: a}
					tabFade.Refresh()
					if p >= 1 {
						tabFade.Hide()
					}
				})
				if p >= 1 {
					break
				}
			}
			t2.Stop()
		}(tab)
	})

	contentStack = container.NewStack(contentArea)
	contentWithFade := container.NewStack(contentStack, tabFade)

	mainLayout := container.NewBorder(nil, nil, sidebar, nil, contentWithFade)

	fadeOverlay := canvas.NewRectangle(color.NRGBA{R: 0x12, G: 0x12, B: 0x12, A: 0xFF})
	wrapper := container.NewStack(bg, mainLayout, fadeOverlay)

	go func() {
		const dur = 350 * time.Millisecond
		start := time.Now()
		ticker := time.NewTicker(14 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			p := math.Min(1.0, float64(time.Since(start))/float64(dur))
			e := 1 - math.Pow(1-p, 3)
			alpha := uint8(255 * (1 - e))
			fyne.Do(func() {
				fadeOverlay.FillColor = color.NRGBA{R: 0x12, G: 0x12, B: 0x12, A: alpha}
				fadeOverlay.Refresh()
				if p >= 1 {
					fadeOverlay.Hide()
				}
			})
			if p >= 1 {
				break
			}
		}
	}()

	return wrapper, mods, nil
}

func (app *App) buildSidebarWithContent(mods []module.Module, proc *win.Process, onTabSwitch func(int)) (fyne.CanvasObject, fyne.CanvasObject) {
	initialContent := app.buildTabContent(app.activeTab, mods)
	sidebar := app.buildSidebar(onTabSwitch)
	return sidebar, initialContent
}

func (app *App) buildTabContent(tab int, mods []module.Module) fyne.CanvasObject {
	switch tab {
	case 1:
		return app.buildConfigsContent()
	case 2:
		return app.createSettingsContent()
	default:
		return app.buildModulesContent(mods)
	}
}

// buildModulesContent — вертикальный список модулей вместо сетки
func (app *App) buildModulesContent(mods []module.Module) fyne.CanvasObject {
	var rows []fyne.CanvasObject
	for _, m := range mods {
		rows = append(rows, app.buildModuleRow(m))
	}
	list := container.NewVBox(rows...)
	padded := container.New(layout.NewCustomPaddedLayout(8, 8, 10, 10), list)
	return container.NewVScroll(padded)
}


// findToggleRecursive рекурсивно ищет M3Toggle в дереве объектов.
func findToggleRecursive(objects []fyne.CanvasObject) *modulesutil.M3Toggle {
	for _, obj := range objects {
		if t, ok := obj.(*modulesutil.M3Toggle); ok {
			return t
		}
		if c, ok := obj.(*fyne.Container); ok {
			if t := findToggleRecursive(c.Objects); t != nil {
				return t
			}
		}
	}
	return nil
}

// buildModuleRow — строка модуля с красной подсветкой при активации
func (app *App) buildModuleRow(m module.Module) fyne.CanvasObject {
	// Левая красная полоска (видна только когда включён)
	accentBar := canvas.NewRectangle(color.Transparent)
	accentBar.CornerRadius = 2
	accentBar.SetMinSize(fyne.NewSize(3, 0))

	// Название модуля (жирный)
	nameText := ctxt(m.Name(), textPrimary, 13)
	nameText.TextStyle = fyne.TextStyle{Bold: true}

	// Описание
	descText := ctxt(app.moduleDisplayDescription(m), textSecondary, 11)

	// Левая часть: название + описание
	leftContent := container.NewVBox(
		nameText,
		container.New(layout.NewCustomPaddedLayout(2, 0, 0, 0), descText),
	)

	// Правая часть: контролы из модуля
	controls := m.CreateObjects()
	var rightContent fyne.CanvasObject
	if len(controls) == 1 {
		rightContent = controls[0]
	} else {
		rightContent = container.NewVBox(controls...)
	}

	// Строка: левый текст + правые контролы
	rowContent := container.NewBorder(nil, nil, nil, rightContent, leftContent)

	// Фон карточки
	cardBg := canvas.NewRectangle(bgCard)
	cardBg.CornerRadius = 6

	// Функция обновления вида при смене состояния
	updateState := func(enabled bool) {
		if enabled {
			accentBar.FillColor = accentRed
			nameText.Color = accentRedBright
			cardBg.FillColor = calpha(accentRed, 0x16)
		} else {
			accentBar.FillColor = color.Transparent
			nameText.Color = textPrimary
			cardBg.FillColor = bgCard
		}
		accentBar.Refresh()
		nameText.Refresh()
		cardBg.Refresh()
	}

	// Начальное состояние
	updateState(m.IsEnabled())

	if toggle := findToggleRecursive(controls); toggle != nil {
		prev := toggle.OnChange
		toggle.OnChange = func(enabled bool) {
			if prev != nil {
				prev(enabled)
			}
			fyne.Do(func() {
				updateState(enabled)
			})
		}
	}

	// Паддинг внутри карточки
	paddedRow := container.New(layout.NewCustomPaddedLayout(10, 10, 10, 10), rowContent)
	// Левая полоска + контент
	rowWithBar := container.NewBorder(nil, nil, accentBar, nil, paddedRow)

	// Итоговая карточка
	card := container.NewStack(cardBg, rowWithBar)

	// Отступ между карточками
	return container.New(layout.NewCustomPaddedLayout(0, 4, 0, 0), card)
}

func (app *App) buildConfigsContent() fyne.CanvasObject {
	title := ctxt(app.t("Конфигурация", "Configuration"), textPrimary, 18)
	title.TextStyle = fyne.TextStyle{Bold: true}
	titleAccent := canvas.NewRectangle(accentRed)
	titleAccent.SetMinSize(fyne.NewSize(36, 2))
	titleAccent.CornerRadius = 1

	exportBtn := createSettingsBtn(app.t("Экспорт конфигурации", "Export configuration"), accentRed, func() {
		w := app.app.NewWindow(app.t("Экспорт", "Export"))
		w.Resize(fyne.NewSize(400, 300))
		app.ExportConfig(w)
		w.Show()
	})
	importBtn := createSettingsBtn(app.t("Импорт конфигурации", "Import configuration"), accentRedBright, func() {
		w := app.app.NewWindow(app.t("Импорт", "Import"))
		w.Resize(fyne.NewSize(400, 300))
		app.ImportConfig(w)
		w.Show()
	})
	resetBtn := createSettingsBtn(app.t("Сбросить настройки", "Reset settings"), accentOrange, func() {
		w := app.app.NewWindow(app.t("Сброс", "Reset"))
		w.Resize(fyne.NewSize(300, 200))
		app.ResetConfig(w)
		w.Show()
	})

	section := createSettingsSection(
		app.t("Файлы конфигурации", "Configuration files"),
		container.NewVBox(exportBtn, importBtn, resetBtn),
	)

	content := container.NewVBox(
		container.New(layout.NewCustomPaddedLayout(16, 4, 14, 14), title),
		container.New(layout.NewCustomPaddedLayout(0, 10, 14, 14),
			container.New(layout.NewCustomPaddedLayout(0, 0, 0, 0), titleAccent)),
		section,
	)
	return container.NewVScroll(content)
}

func (app *App) buildSidebar(onTabSwitch func(int)) fyne.CanvasObject {
	// Логотип
	logoZ := ctxt("z", accentRed, 20)
	logoZ.TextStyle = fyne.TextStyle{Bold: true}
	logoUtil := ctxt("Utility", textPrimary, 20)
	logoUtil.TextStyle = fyne.TextStyle{Bold: true}
	logoRow := container.NewHBox(
		logoZ,
		container.New(layout.NewCustomPaddedLayout(0, 0, -4, 0), logoUtil),
	)
	logoPadded := container.New(layout.NewCustomPaddedLayout(14, 12, 16, 16), logoRow)

	sep1 := canvas.NewRectangle(borderSubtle)
	sep1.SetMinSize(fyne.NewSize(0, 1))

	type navItem struct {
		label string
		tab   int
	}
	tabs := []navItem{
		{app.t("Модули", "Modules"), 0},
		{app.t("Конфигурация", "Configuration"), 1},
		{app.t("Настройки", "Settings"), 2},
	}

	navItems := make([]fyne.CanvasObject, 0, len(tabs))
	type navBtn struct {
		bg   *canvas.Rectangle
		bar  *canvas.Rectangle
		text *canvas.Text
	}
	btns := make([]navBtn, len(tabs))

	updateActive := func(activeTab int) {
		for i, nb := range btns {
			if i == activeTab {
				nb.bg.FillColor = calpha(accentRed, 0x18)
				nb.bar.FillColor = accentRed
				nb.text.Color = accentRedBright
			} else {
				nb.bg.FillColor = color.Transparent
				nb.bar.FillColor = color.Transparent
				nb.text.Color = textSecondary
			}
			nb.bg.Refresh()
			nb.bar.Refresh()
			nb.text.Refresh()
		}
	}

	for i, tab := range tabs {
		idx := i
		tabIdx := tab.tab

		bg := canvas.NewRectangle(color.Transparent)
		bg.CornerRadius = 4

		bar := canvas.NewRectangle(color.Transparent)
		bar.SetMinSize(fyne.NewSize(3, 0))
		bar.CornerRadius = 2

		lbl := ctxt(tab.label, textSecondary, 13)

		if idx == app.activeTab {
			bg.FillColor = calpha(accentRed, 0x18)
			bar.FillColor = accentRed
			lbl.Color = accentRedBright
		}

		btns[idx] = navBtn{bg: bg, bar: bar, text: lbl}

		labelPadded := container.New(layout.NewCustomPaddedLayout(8, 8, 8, 6), lbl)
		row := container.NewBorder(nil, nil, bar, nil, labelPadded)
		item := container.NewStack(bg, row)

		tap := newNavTapArea(func() {
			app.activeTab = tabIdx
			updateActive(tabIdx)
			onTabSwitch(tabIdx)
		})

		navItems = append(navItems, container.NewStack(item, tap))
	}

	navSection := container.New(layout.NewCustomPaddedLayout(8, 0, 6, 6),
		container.NewVBox(navItems...),
	)

	// Minecraft-карточка внизу сайдбара
	dot := canvas.NewRectangle(accentGreen)
	dot.CornerRadius = 3.5
	dot.SetMinSize(fyne.NewSize(7, 7))
	dotBox := container.NewStack(newSizedBox(7, 7), dot)
	dotPadded := container.New(layout.NewCustomPaddedLayout(4, 0, 0, 8), dotBox)

	mcTitle := ctxt("Minecraft", textPrimary, 11)
	mcTitle.TextStyle = fyne.TextStyle{Bold: true}
	mcSub := ctxt("PE 1.1.5 • UWP", calpha(accentGreen, 0xAA), 9)
	mcTextCol := container.NewVBox(
		mcTitle,
		container.New(layout.NewCustomPaddedLayout(2, 0, 0, 0), mcSub),
	)
	mcLeft := container.NewHBox(dotPadded, mcTextCol)

	playIcon := ctxt("▶", calpha(accentGreen, 0xDD), 13)
	playBg := canvas.NewRectangle(calpha(accentGreen, 0x1A))
	playBg.CornerRadius = 6
	playTap := newNavTapArea(func() {
		_ = exec.Command("cmd", "/c", "start", "minecraft://").Start()
	})
	playBox := container.NewStack(
		playBg,
		newSizedBox(28, 28),
		container.NewCenter(playIcon),
		playTap,
	)

	mcRow := container.NewBorder(nil, nil, mcLeft, playBox)

	mcCardBg := canvas.NewRectangle(calpha(accentGreen, 0x0D))
	mcCardBg.CornerRadius = 8
	mcCard := container.NewStack(
		mcCardBg,
		container.New(layout.NewCustomPaddedLayout(7, 7, 8, 8), mcRow),
	)
	statusBlock := container.New(layout.NewCustomPaddedLayout(4, 6, 6, 6), mcCard)

	sidebarInner := container.NewBorder(
		container.NewVBox(logoPadded, sep1, navSection),
		statusBlock,
		nil, nil,
		layout.NewSpacer(),
	)

	sidebarBg := canvas.NewRectangle(bgSidebar)
	sepRight := canvas.NewRectangle(borderSubtle)
	sepRight.SetMinSize(fyne.NewSize(1, 0))

	minSizer := canvas.NewRectangle(color.Transparent)
	minSizer.SetMinSize(fyne.NewSize(140, 0))

	sidebarContent := container.NewBorder(nil, nil, nil, sepRight, sidebarInner)

	return container.NewStack(sidebarBg, minSizer, sidebarContent)
}


// sizedBox — прозрачный виджет с фиксированным MinSize.
type sizedBox struct {
	widget.BaseWidget
	w, h float32
}

func newSizedBox(w, h float32) *sizedBox {
	b := &sizedBox{w: w, h: h}
	b.ExtendBaseWidget(b)
	return b
}

func (b *sizedBox) CreateRenderer() fyne.WidgetRenderer {
	r := canvas.NewRectangle(color.Transparent)
	return widget.NewSimpleRenderer(r)
}

func (b *sizedBox) MinSize() fyne.Size { return fyne.NewSize(b.w, b.h) }

func ctxt(s string, c color.Color, size float32) *canvas.Text {
	t := canvas.NewText(s, c)
	t.TextSize = size
	return t
}

// navTapArea — кликабельная область с мягким hover-эффектом.
type navTapArea struct {
	widget.BaseWidget
	onTap     func()
	hoverRect *canvas.Rectangle
}

func newNavTapArea(onTap func()) *navTapArea {
	a := &navTapArea{onTap: onTap}
	a.hoverRect = canvas.NewRectangle(color.Transparent)
	a.hoverRect.CornerRadius = 4
	a.ExtendBaseWidget(a)
	return a
}

func (a *navTapArea) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(a.hoverRect)
}

func (a *navTapArea) Tapped(_ *fyne.PointEvent) {
	if a.onTap != nil {
		a.onTap()
	}
}

func (a *navTapArea) MouseIn(_ *desktop.MouseEvent) {
	a.hoverRect.FillColor = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x0A}
	a.hoverRect.Refresh()
}

func (a *navTapArea) MouseOut() {
	a.hoverRect.FillColor = color.Transparent
	a.hoverRect.Refresh()
}

func (a *navTapArea) MouseMoved(_ *desktop.MouseEvent) {}

func (app *App) createControllerSensitivityModule(proc *win.Process) module.Module {
	return modules.ControllerSensitivity{Process: proc, Error: app.onError("controller_sensitivity")}.Create()
}
func (app *App) createNoDynamicFovModule(proc *win.Process) module.Module {
	return modules.NoDynamicFov{Process: proc, Error: app.onError("no_dynamic_fov"), AfterChange: app.autoSave}.Create()
}
func (app *App) createNoHurtCamModule(proc *win.Process) module.Module {
	return modules.NoHurtCam{Process: proc, Error: app.onError("no_hurt_cam"), AfterChange: app.autoSave}.Create()
}
func (app *App) createNoCamResetModule(proc *win.Process) module.Module {
	return modules.NoCamReset{Process: proc, Error: app.onError("no_cam_reset"), AfterChange: app.autoSave}.Create()
}
func (app *App) createAutoSprintModule(proc *win.Process) module.Module {
	return modules.AutoSprint{Process: proc, Error: app.onError("auto_sprint"), AfterChange: app.autoSave}.Create()
}
func (app *App) createNoParticleModule(proc *win.Process) module.Module {
	return modules.NoParticle{Process: proc, Error: app.onError("no_particle"), AfterChange: app.autoSave}.Create()
}
func (app *App) createNoVsyncModule(proc *win.Process) module.Module {
	return modules.NoVsync{Process: proc, Error: app.onError("no_vsync"), AfterChange: app.autoSave}.Create()
}
func (app *App) createZoomV2Module(proc *win.Process, noDynFov module.Module) module.Module {
	return modules.ZoomV2{
		Process:      proc,
		Error:        app.onError("zoom"),
		AfterChange:  app.autoSave,
		NoDynamicFov: noDynFov,
		Localize:     app.t,
	}.Create()
}
func (app *App) createTimeModule(proc *win.Process) module.Module {
	return modules.Time{Process: proc, Error: app.onError("time_changer"), AfterChange: app.autoSave}.Create()
}

func (app *App) createItemUseDelayModule(proc *win.Process) module.Module {
	return modules.ItemUseDelay{Process: proc, Error: app.onError("item_use_delay"), AfterChange: app.autoSave}.Create()
}

func (app *App) createRainModule(proc *win.Process) module.Module {
	return modules.Rain{Process: proc, Error: app.onError("rain"), AfterChange: app.autoSave}.Create()
}

func (app *App) onError(mod string) func(error) {
	return func(err error) {
		app.conf.Logger.Error("an error occurred", "module", mod, "err", err.Error())
	}
}

func (app *App) autoSave() {
	if err := app.SaveConfig(); err != nil {
		app.conf.Logger.Error("auto-save failed", "err", err)
	}
}

// suppress unused variable warning for bgElevated
var _ = bgElevated
