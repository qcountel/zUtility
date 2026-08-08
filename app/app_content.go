package app

import (
	"image/color"
	"math"
	"os/exec"
	"sort"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/qcountel/zUtility/app/module"
	"github.com/qcountel/zUtility/app/module/modules"
	"github.com/qcountel/zUtility/app/module/modules/modulesutil"
	win "github.com/qcountel/zUtility/pkg/winlegacy"
)

func ctxt(s string, c color.Color, size float32) *canvas.Text {
	t := canvas.NewText(s, c)
	t.TextSize = size
	return t
}

// ─── createContent ────────────────────────────────────────────────────

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
			app.createCustomFogModule(proc),
			app.createItemUseDelayModule(proc),
			app.createNoCamResetModule(proc),
			noDynFovMod,
			app.createNoHurtCamModule(proc),
			app.createNoParticleModule(proc),
			app.createNoVsyncModule(proc),
			app.createQuickSlotsModule(proc),
			app.createSensitivityModule(proc),
			app.createSkyboxModule(proc),
			app.createTimeModule(proc),
			app.createZoomV2Module(proc, noDynFovMod),
		}

		sort.Slice(mods, func(i, j int) bool {
			return strings.ToLower(mods[i].Name()) < strings.ToLower(mods[j].Name())
		})
	}

	bg := canvas.NewRectangle(bgPrimary)

	var contentStack *fyne.Container

	tabFade := canvas.NewRectangle(color.NRGBA{R: 0x08, G: 0x08, B: 0x08, A: 0x00})
	tabFade.Hide()

	var tabAnimating atomic.Bool

	header := app.buildHeader()

	toolbar := app.buildToolbar(mods, func(tab int) {
		if contentStack == nil {
			return
		}
		if !tabAnimating.CompareAndSwap(false, true) {
			return
		}

		tabFade.FillColor = color.NRGBA{R: 0x08, G: 0x08, B: 0x08, A: 0x00}
		tabFade.Show()
		tabFade.Refresh()

		var animOut *fyne.Animation
		var animIn *fyne.Animation

		animOut = &fyne.Animation{
			Duration: 70 * time.Millisecond,
			Curve:    fyne.AnimationLinear,
			Tick: func(p float32) {
				a := uint8(210 * p)
				tabFade.FillColor = color.NRGBA{R: 0x08, G: 0x08, B: 0x08, A: a}
				tabFade.Refresh()

				if p >= 1.0 {
					app.activeTab = tab
					newContent := app.buildTabContent(tab, mods)
					if len(contentStack.Objects) > 0 {
						contentStack.Objects[0] = newContent
					} else {
						contentStack.Add(newContent)
					}
					contentStack.Refresh()

					animIn.Start()
				}
			},
		}

		animIn = &fyne.Animation{
			Duration: 140 * time.Millisecond,
			Curve:    fyne.AnimationLinear,
			Tick: func(p float32) {
				ease := 1.0 - math.Pow(float64(1.0-p), 3.0)
				a := uint8(210 * (1.0 - ease))
				tabFade.FillColor = color.NRGBA{R: 0x08, G: 0x08, B: 0x08, A: a}
				tabFade.Refresh()

				if p >= 1.0 {
					tabFade.Hide()
					tabAnimating.Store(false)
				}
			},
		}

		animOut.Start()
	})

	initialContent := app.buildTabContent(app.activeTab, mods)
	contentStack = container.NewStack(initialContent)
	contentWithFade := container.NewStack(contentStack, tabFade)

	footer := app.buildFooter()

	topSection := container.NewVBox(header, toolbar)
	mainLayout := container.NewBorder(topSection, footer, nil, nil, contentWithFade)

	fadeOverlay := canvas.NewRectangle(color.NRGBA{R: 0x08, G: 0x08, B: 0x08, A: 0xFF})
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
				fadeOverlay.FillColor = color.NRGBA{R: 0x08, G: 0x08, B: 0x08, A: alpha}
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

// ─── Brutalist Header ─────────────────────────────────────────────────

func (app *App) buildHeader() fyne.CanvasObject {
	headerBg := canvas.NewRectangle(accentRed)

	title := ctxt("ZUTILITY", color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xFF}, 16)
	title.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}

	headerContent := container.NewHBox(
		container.New(layout.NewCustomPaddedLayout(0, 0, 0, 0), title),
	)

	headerPadded := container.New(layout.NewCustomPaddedLayout(10, 10, 16, 16), headerContent)
	return container.NewStack(headerBg, headerPadded)
}

// ─── Brutalist Toolbar ────────────────────────────────────────────────

func (app *App) buildToolbar(mods []module.Module, onTabSwitch func(int)) fyne.CanvasObject {
	borderBottom := canvas.NewRectangle(borderSubtle)
	borderBottom.SetMinSize(fyne.NewSize(0, 1))

	type tabDef struct {
		label string
		tab   int
	}
	tabs := []tabDef{
		{"MODULES", 0},
		{"CONFIG", 1},
		{"PACKS", 2},
		{"CLICKER", 4},
		{"SETTINGS", 3},
	}

	type tabBtn struct {
		underline *canvas.Rectangle
		text      *canvas.Text
	}
	btns := make([]tabBtn, len(tabs))

	updateActive := func(activeTab int) {
		for i, tb := range btns {
			if tabs[i].tab == activeTab {
				tb.underline.FillColor = accentRed
				tb.text.Color = accentRed
			} else {
				tb.underline.FillColor = color.Transparent
				tb.text.Color = chex(0x44, 0x44, 0x44)
			}
			tb.underline.Refresh()
			tb.text.Refresh()
		}
	}

	var tabItems []fyne.CanvasObject
	for i, tab := range tabs {
		idx := i
		tabIdx := tab.tab

		lbl := ctxt(tab.label, chex(0x44, 0x44, 0x44), 13)
		lbl.TextStyle = fyne.TextStyle{Monospace: true}

		underline := canvas.NewRectangle(color.Transparent)
		underline.SetMinSize(fyne.NewSize(0, 2))

		if tabIdx == app.activeTab {
			lbl.Color = accentRed
			underline.FillColor = accentRed
		}

		btns[idx] = tabBtn{underline: underline, text: lbl}

		lblPadded := container.New(layout.NewCustomPaddedLayout(10, 7, 16, 16), lbl)
		btnCol := container.NewVBox(lblPadded, underline)

		tap := newNavTapArea(func() {
			app.activeTab = tabIdx
			updateActive(tabIdx)
			onTabSwitch(tabIdx)
		})

		tabItems = append(tabItems, container.NewStack(btnCol, tap))
	}

	tabsRow := container.NewHBox(tabItems...)

	// Launch MC button at the right
	playIcon := ctxt("\u25B6", calpha(accentRed, 0xDD), 12)
	playIcon.TextStyle = fyne.TextStyle{Monospace: true}
	playTap := newNavTapArea(func() {
		cmd := exec.Command("cmd", "/c", "start", "minecraft://")
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		_ = cmd.Start()
	})
	playTap.noHover = true
	playBg := canvas.NewRectangle(calpha(accentRed, 0x0D))
	playBg.CornerRadius = 6
	playBg.SetMinSize(fyne.NewSize(36, 0))
	playIconPadded := container.New(layout.NewCustomPaddedLayout(0, 0, 2, 0), playIcon)
	playBtn := container.NewStack(playBg, container.NewCenter(playIconPadded), playTap)
	playBtnPadded := container.New(layout.NewCustomPaddedLayout(6, 6, 6, 8), playBtn)

	toolbarContent := container.NewBorder(nil, nil, tabsRow, playBtnPadded)

	return container.NewVBox(toolbarContent, borderBottom)
}

// ─── Brutalist Footer ─────────────────────────────────────────────────

func (app *App) buildFooter() fyne.CanvasObject {
	borderTop := canvas.NewRectangle(accentRed)
	borderTop.SetMinSize(fyne.NewSize(0, 2))

	footerBg := canvas.NewRectangle(bgFooter)

	verLabel := ctxt("v"+CurrentVersion, accentRed, 12)
	verLabel.TextStyle = fyne.TextStyle{Monospace: true}

	footerContent := container.NewBorder(nil, nil, nil, verLabel)
	footerPadded := container.New(layout.NewCustomPaddedLayout(8, 8, 16, 16), footerContent)

	return container.NewVBox(borderTop, container.NewStack(footerBg, footerPadded))
}

// ─── animateToSettings ────────────────────────────────────────────────

func (app *App) animateToSettings() {
	if app.win == nil {
		return
	}
	app.activeTab = 3
	app.showSettings = true
	go func() {
		fyne.Do(func() {
			nc, _, _ := app.createContent(app.tr.Process())
			if !app.animationsEnabled {
				app.win.SetContent(nc)
				return
			}
			overlay := canvas.NewRectangle(color.NRGBA{R: 0x08, G: 0x08, B: 0x08, A: 0xFF})
			app.win.SetContent(container.NewStack(nc, overlay))
			go func() {
			const dur = 220 * time.Millisecond
			start := time.Now()
			ticker := time.NewTicker(14 * time.Millisecond)
			defer ticker.Stop()
			for range ticker.C {
				p := math.Min(1.0, float64(time.Since(start))/float64(dur))
				ease := 1 - math.Pow(1-p, 3)
				a := uint8(255 * (1 - ease))
				fyne.Do(func() {
					overlay.FillColor = color.NRGBA{R: 0x08, G: 0x08, B: 0x08, A: a}
					overlay.Refresh()
					if p >= 1 {
						overlay.Hide()
					}
				})
				if p >= 1 {
					break
				}
			}
		}()
	})
	}()
}

// ─── Tab content routing ──────────────────────────────────────────────

func (app *App) buildTabContent(tab int, mods []module.Module) fyne.CanvasObject {
	switch tab {
	case 1:
		return app.buildConfigsContent()
	case 2:
		return app.buildResourcePacksContent()
	case 3:
		return app.createSettingsContent()
	case 4:
		return app.buildClickerContent()
	default:
		return app.buildModulesContent(mods)
	}
}


// ─── Brutalist Modules List ───────────────────────────────────────────

func getModuleCategory(name string) string {
	switch canonicalModuleName(name) {
	case "autosprint", "itemusedelay", "timechanger", "offvsync":
		return "GAMEPLAY"
	case "nohurtcam", "nocamreset", "nodynamicfov", "noparticle", "zoom":
		return "VISUAL"
	case "controllersensitivity", "instanthotbar":
		return "CONTROLLER"
	default:
		return "GAMEPLAY"
	}
}

func (app *App) buildModulesContent(mods []module.Module) fyne.CanvasObject {
	listContainer := container.NewVBox()

	var rows []fyne.CanvasObject
	for _, m := range mods {
		rows = append(rows, app.buildModuleRow(m))
	}
	listContainer.Objects = rows
	listContainer.Refresh()

	scrollableList := container.NewVScroll(listContainer)
	paddedList := container.New(layout.NewCustomPaddedLayout(4, 0, 0, 0), scrollableList)
	return paddedList
}

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

func (app *App) buildModuleRow(m module.Module) fyne.CanvasObject {
	// 8×8 square state indicator
	stateBox := canvas.NewRectangle(color.Transparent)
	stateBox.CornerRadius = 3
	stateBox.SetMinSize(fyne.NewSize(10, 10))
	stateBox.StrokeWidth = 1.2
	stateBox.StrokeColor = textGhost

	nameText := ctxt(m.Name(), chex(0x44, 0x44, 0x44), 14)
	nameText.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}

	descText := ctxt(m.Description(), textGhost, 11)

	controls := m.CreateObjects()
	var rightContent fyne.CanvasObject
	var bottomContent []fyne.CanvasObject
	if len(controls) > 0 {
		rightContent = controls[0]
		if len(controls) > 1 {
			bottomContent = controls[1:]
		}
	}

	// Row background (tinted when enabled)
	rowBg := canvas.NewRectangle(color.Transparent)
	rowBg.CornerRadius = 3

	updateState := func(enabled bool) {
		if enabled {
			stateBox.FillColor = accentRed
			stateBox.StrokeColor = accentRed
			nameText.Color = chex(0xE0, 0xE0, 0xE0)
			descText.Color = textSecondary
			rowBg.FillColor = accentAlpha(0x0A)
		} else {
			stateBox.FillColor = color.Transparent
			stateBox.StrokeColor = textGhost
			nameText.Color = chex(0x44, 0x44, 0x44)
			descText.Color = textGhost
			rowBg.FillColor = color.Transparent
		}
		stateBox.Refresh()
		nameText.Refresh()
		descText.Refresh()
		rowBg.Refresh()
	}

	updateState(m.IsEnabled())

	if toggle := findToggleRecursive(controls); toggle != nil {
		toggle.OnStateChange = func(enabled bool) {
			fyne.Do(func() {
				updateState(enabled)
			})
		}
	}

	textCol := container.NewVBox(nameText, descText)

	leftPart := container.NewHBox(
		container.NewCenter(stateBox),
		container.New(layout.NewCustomPaddedLayout(0, 0, 4, 0), textCol),
	)

	rowContent := container.NewBorder(nil, nil, leftPart, rightContent)
	paddedRow := container.New(layout.NewCustomPaddedLayout(8, 8, 16, 16), rowContent)

	cardVBox := container.NewVBox(paddedRow)
	for _, b := range bottomContent {
		cardVBox.Add(b)
	}

	rowBorder := canvas.NewRectangle(borderRow)
	rowBorder.SetMinSize(fyne.NewSize(0, 1))

	return container.NewVBox(
		container.NewStack(rowBg, cardVBox),
		rowBorder,
	)
}

// ─── Configs content ──────────────────────────────────────────────────

func (app *App) buildConfigsContent() fyne.CanvasObject {
	title := ctxt("CONFIGURATION", textPrimary, 18)
	title.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
	titleAccent := canvas.NewRectangle(accentRed)
	titleAccent.SetMinSize(fyne.NewSize(36, 2))

	exportBtn := createSettingsBtn("Экспорт конфигурации", accentRed, func() {
		w := app.app.NewWindow("Экспорт")
		w.Resize(fyne.NewSize(400, 300))
		app.ExportConfig(w)
		w.Show()
	})
	importBtn := createSettingsBtn("Импорт конфигурации", accentRedBright, func() {
		w := app.app.NewWindow("Импорт")
		w.Resize(fyne.NewSize(400, 300))
		app.ImportConfig(w)
		w.Show()
	})
	resetBtn := createSettingsBtn("Сбросить настройки", accentOrange, func() {
		w := app.app.NewWindow("Сброс")
		w.Resize(fyne.NewSize(300, 200))
		app.ResetConfig(w)
		w.Show()
	})

	section := createSettingsSection(
		"Файлы конфигурации",
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

// ─── Helpers ──────────────────────────────────────────────────────────

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

type navTapArea struct {
	widget.BaseWidget
	onTap     func()
	hoverRect *canvas.Rectangle
	noHover   bool
}

func newNavTapArea(onTap func()) *navTapArea {
	a := &navTapArea{onTap: onTap}
	a.hoverRect = canvas.NewRectangle(color.Transparent)
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
	if a.noHover {
		return
	}
	a.hoverRect.FillColor = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x08}
	a.hoverRect.Refresh()
}

func (a *navTapArea) MouseOut() {
	a.hoverRect.FillColor = color.Transparent
	a.hoverRect.Refresh()
}

func (a *navTapArea) MouseMoved(_ *desktop.MouseEvent) {}

// ─── Module factories ─────────────────────────────────────────────────

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
	}.Create()
}
func (app *App) createTimeModule(proc *win.Process) module.Module {
	return modules.Time{Process: proc, Error: app.onError("time_changer"), AfterChange: app.autoSave}.Create()
}
func (app *App) createItemUseDelayModule(proc *win.Process) module.Module {
	return modules.ItemUseDelay{Process: proc, Error: app.onError("item_use_delay"), AfterChange: app.autoSave}.Create()
}
func (app *App) createSensitivityModule(proc *win.Process) module.Module {
	return modules.Sensitivity{Process: proc, Error: app.onError("sensitivity"), AfterChange: app.autoSave}.Create()
}

func (app *App) createCustomFogModule(proc *win.Process) module.Module {
	return modules.CustomFog{Process: proc, Error: app.onError("custom_fog"), AfterChange: app.autoSave}.Create()
}

func (app *App) createQuickSlotsModule(proc *win.Process) module.Module {
	return modules.QuickSlots{Process: proc, Error: app.onError("quick_slots"), AfterChange: app.autoSave}.Create()
}

func (app *App) createSkyboxModule(proc *win.Process) module.Module {
	return modules.Skybox{Process: proc, Error: app.onError("skybox"), AfterChange: app.autoSave}.Create()
}

func (app *App) onError(mod string) func(error) {
	return func(err error) {
		app.conf.Logger.Error("an error occurred", "module", mod, "err", err.Error())
	}
}

func (app *App) autoSave() {
	if app.loadingConfig.Load() {
		return
	}
	if err := app.SaveConfig(); err != nil {
		app.conf.Logger.Error("auto-save failed", "err", err)
	}
}
