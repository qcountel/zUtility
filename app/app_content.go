package app

import (
	"image/color"
	"math"
	"net/url"
	"os/exec"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/something-that-is-cool/zutil/app/module"
	"github.com/something-that-is-cool/zutil/app/module/modules"
	"github.com/something-that-is-cool/zutil/internal/pkg/win"
)

var (
	bgPrimary  = chex(0x08, 0x05, 0x05)
	bgCard     = chex(0x10, 0x09, 0x09)
	bgElevated = chex(0x18, 0x0E, 0x0E)

	accentRed       = chex(0xC0, 0x1E, 0x1E)
	accentRedBright = chex(0xE5, 0x33, 0x33)
	accentRedDim    = chex(0x7A, 0x14, 0x14)
	accentOrange    = chex(0xDD, 0x77, 0x22)
	accentGreen     = chex(0x33, 0xBB, 0x55)

	textPrimary   = chex(0xF0, 0xF0, 0xF0)
	textSecondary = chex(0x88, 0x77, 0x77)
	textDim       = chex(0x50, 0x40, 0x40)

	borderSubtle = chex(0x1E, 0x12, 0x12)
	borderRed    = color.NRGBA{R: 0xC0, G: 0x1E, B: 0x1E, A: 0x35}
)

func chex(r, g, b uint8) color.NRGBA {
	return color.NRGBA{R: r, G: g, B: b, A: 0xff}
}

func calpha(c color.NRGBA, a uint8) color.NRGBA {
	return color.NRGBA{R: c.R, G: c.G, B: c.B, A: a}
}

var telegramURL, _ = url.Parse("https://t.me/zovutil")

func (app *App) createContent(proc *win.Process) (fyne.CanvasObject, []module.Module, error) {
	if app.showSettings {
		return app.createSettingsContent(), nil, nil
	}
	if app.showPacks {
		return app.buildResourcePacksContent(), nil, nil
	}

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
			app.createNoFireModule(proc),
			app.createNoVsyncModule(proc),
			app.createControllerSensitivityModule(proc),
			app.createZoomV2Module(proc, noDynFovMod),
			app.createTimeModule(proc),
			app.createItemUseDelayModule(proc),
		}
	}

	bg := canvas.NewRectangle(bgPrimary)
	header := app.buildHeader()

	var cards []fyne.CanvasObject
	for _, m := range mods {
		cards = append(cards, buildCard(app, m))
	}
	grid := container.NewGridWithColumns(2, cards...)
	gridPadded := container.New(layout.NewCustomPaddedLayout(4, 2, 8, 8), grid)

	footer := buildFooter()

	body := container.NewBorder(header, footer, nil, nil, container.NewVScroll(gridPadded))

	fadeOverlay := canvas.NewRectangle(color.NRGBA{R: 0x08, G: 0x05, B: 0x05, A: 0xFF})
	wrapper := container.NewStack(bg, body, fadeOverlay)
	wrapper.Resize(fyne.NewSize(520, 720))

	go func() {
		const dur = 350 * time.Millisecond
		start := time.Now()
		ticker := time.NewTicker(14 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			p := math.Min(1.0, float64(time.Since(start))/float64(dur))

			e := 1 - math.Pow(1-p, 3)
			alpha := uint8(255 * (1 - e))
			fadeOverlay.FillColor = color.NRGBA{R: 0x0A, G: 0x0C, B: 0x14, A: alpha}
			fadeOverlay.Refresh()
			if p >= 1 {

				fadeOverlay.Hide()
				break
			}
		}
	}()

	return wrapper, mods, nil
}

func (app *App) buildHeader() fyne.CanvasObject {
	logoZ := ctxt("z", accentRed, 24)
	logoZ.TextStyle = fyne.TextStyle{Bold: true}
	logoUtil := ctxt("Utility", textPrimary, 24)
	logoUtil.TextStyle = fyne.TextStyle{Bold: true}

	logoBox := container.New(layout.NewCustomPaddedLayout(0, 0, 0, 0),
		container.New(layout.NewHBoxLayout(), logoZ,
			container.New(layout.NewCustomPaddedLayout(0, 0, -6, 0), logoUtil),
		),
	)

	settingsBtn := widget.NewButtonWithIcon("", theme.SettingsIcon(), func() {
		app.animateToSettings()
	})
	settingsBtn.Importance = widget.LowImportance

	packsBtn := widget.NewButtonWithIcon("", theme.StorageIcon(), func() {
		app.animateToPacks()
	})
	packsBtn.Importance = widget.LowImportance

	launchBtn := widget.NewButtonWithIcon("", theme.MediaPlayIcon(), func() {
		go func() {
			cmd := exec.Command("cmd", "/C", "start", "minecraft:")
			if err := cmd.Run(); err != nil {
				_ = exec.Command("explorer.exe",
					`shell:AppsFolder\Microsoft.MinecraftUWP_8wekyb3d8bbwe!App`).Run()
			}
		}()
	})
	launchBtn.Importance = widget.LowImportance

	btnRow := container.NewHBox(launchBtn, packsBtn, settingsBtn)
	topRow := container.NewBorder(nil, nil, logoBox, btnRow)

	headerBg := canvas.NewRectangle(bgCard)
	sepLine := canvas.NewRectangle(borderRed)
	sepLine.SetMinSize(fyne.NewSize(0, 1))

	paddedInner := container.New(layout.NewCustomPaddedLayout(10, 8, 12, 12), topRow)
	content := container.NewBorder(nil, sepLine, nil, nil, paddedInner)
	return container.NewStack(headerBg, content)
}

func (app *App) animateToSettings() {
	if app.win == nil {
		return
	}

	current := app.win.Content()
	overlay := canvas.NewRectangle(color.NRGBA{R: 0x08, G: 0x05, B: 0x05, A: 0x00})
	app.win.SetContent(container.NewStack(current, overlay))

	go func() {

		const dur1 = 180 * time.Millisecond
		start := time.Now()
		ticker := time.NewTicker(14 * time.Millisecond)
		for range ticker.C {
			p := math.Min(1.0, float64(time.Since(start))/float64(dur1))
			a := uint8(255 * (p * p))
			fyne.Do(func() {
				overlay.FillColor = color.NRGBA{R: 0x08, G: 0x05, B: 0x05, A: a}
				overlay.Refresh()
			})
			if p >= 1 {
				break
			}
		}
		ticker.Stop()

		app.showSettings = true
		fyne.Do(func() {
			nc, _, _ := app.createContent(app.tr.Process())
			newOverlay := canvas.NewRectangle(color.NRGBA{R: 0x08, G: 0x05, B: 0x05, A: 0xFF})
			app.win.SetContent(container.NewStack(nc, newOverlay))
			app.win.Resize(fyne.NewSize(520, 720))

			go func() {
				const dur2 = 220 * time.Millisecond
				start2 := time.Now()
				t2 := time.NewTicker(14 * time.Millisecond)
				defer t2.Stop()
				for range t2.C {
					p := math.Min(1.0, float64(time.Since(start2))/float64(dur2))
					ease := 1 - math.Pow(1-p, 3)
					a := uint8(255 * (1 - ease))
					fyne.Do(func() {
						newOverlay.FillColor = color.NRGBA{R: 0x08, G: 0x05, B: 0x05, A: a}
						newOverlay.Refresh()
						if p >= 1 {
							newOverlay.Hide()
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

func (app *App) animateToPacks() {
	if app.win == nil {
		return
	}

	current := app.win.Content()
	overlay := canvas.NewRectangle(color.NRGBA{R: 0x08, G: 0x05, B: 0x05, A: 0x00})
	app.win.SetContent(container.NewStack(current, overlay))

	go func() {
		const dur1 = 180 * time.Millisecond
		start := time.Now()
		ticker := time.NewTicker(14 * time.Millisecond)
		for range ticker.C {
			p := math.Min(1.0, float64(time.Since(start))/float64(dur1))
			a := uint8(255 * (p * p))
			fyne.Do(func() {
				overlay.FillColor = color.NRGBA{R: 0x08, G: 0x05, B: 0x05, A: a}
				overlay.Refresh()
			})
			if p >= 1 {
				break
			}
		}
		ticker.Stop()

		app.showPacks = true
		fyne.Do(func() {
			nc, _, _ := app.createContent(app.tr.Process())
			newOverlay := canvas.NewRectangle(color.NRGBA{R: 0x08, G: 0x05, B: 0x05, A: 0xFF})
			app.win.SetContent(container.NewStack(nc, newOverlay))
			app.win.Resize(fyne.NewSize(520, 720))

			go func() {
				const dur2 = 220 * time.Millisecond
				start2 := time.Now()
				t2 := time.NewTicker(14 * time.Millisecond)
				defer t2.Stop()
				for range t2.C {
					p := math.Min(1.0, float64(time.Since(start2))/float64(dur2))
					ease := 1 - math.Pow(1-p, 3)
					a := uint8(255 * (1 - ease))
					fyne.Do(func() {
						newOverlay.FillColor = color.NRGBA{R: 0x08, G: 0x05, B: 0x05, A: a}
						newOverlay.Refresh()
						if p >= 1 {
							newOverlay.Hide()
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

func buildCard(app *App, m module.Module) fyne.CanvasObject {
	nameLabel := ctxt(m.Name(), textPrimary, 13)
	nameLabel.TextStyle = fyne.TextStyle{Bold: true}

	infoBtn := widget.NewButtonWithIcon("", theme.InfoIcon(), func() {
		showModuleInfo(app, m.Name(), app.moduleDisplayDescription(m))
	})
	infoBtn.Importance = widget.LowImportance

	nameRow := container.NewBorder(nil, nil, nameLabel, infoBtn)

	controls := container.NewVBox(m.CreateObjects()...)

	inner := container.NewVBox(
		nameRow,
		container.New(layout.NewCustomPaddedLayout(4, 0, 0, 0), controls),
	)

	cardBg := canvas.NewRectangle(bgCard)
	cardBg.CornerRadius = 8
	cardBorder := canvas.NewRectangle(borderRed)
	cardBorder.CornerRadius = 9

	paddedInner := container.New(layout.NewCustomPaddedLayout(12, 12, 12, 12), inner)
	card := container.NewStack(cardBorder, cardBg, paddedInner)
	return container.New(layout.NewCustomPaddedLayout(2, 5, 5, 5), card)
}

func showModuleInfo(app *App, name, description string) {
	a := fyne.CurrentApp()
	if a == nil {
		return
	}
	var win fyne.Window
	for _, w := range a.Driver().AllWindows() {
		if w.Title() == Name {
			win = w
			break
		}
	}
	if win == nil {
		return
	}

	c := win.Canvas()

	accent := canvas.NewRectangle(accentRed)
	accent.SetMinSize(fyne.NewSize(3, 18))
	accent.CornerRadius = 2

	titleTxt := canvas.NewText(name, accentRedBright)
	titleTxt.TextSize = 15
	titleTxt.TextStyle = fyne.TextStyle{Bold: true}

	titleRow := container.NewHBox(
		container.New(layout.NewCustomPaddedLayout(0, 0, 0, 8), accent),
		titleTxt,
	)

	div := canvas.NewRectangle(borderRed)
	div.SetMinSize(fyne.NewSize(0, 1))

	descLabel := widget.NewLabel(description)
	descLabel.Wrapping = fyne.TextWrapWord

	var popup *widget.PopUp

	closeBtn := widget.NewButton(app.t("Закрыть", "Close"), nil)
	closeBtn.Importance = widget.LowImportance

	inner := container.NewVBox(
		titleRow,
		container.New(layout.NewCustomPaddedLayout(4, 10, 0, 0), div),
		container.New(layout.NewCustomPaddedLayout(0, 16, 0, 0), descLabel),
		container.NewHBox(layout.NewSpacer(), closeBtn),
	)

	cardBg := canvas.NewRectangle(bgCard)
	cardBg.CornerRadius = 12
	cardBorder := canvas.NewRectangle(borderRed)
	cardBorder.CornerRadius = 13

	minSizer := canvas.NewRectangle(color.Transparent)
	minSizer.SetMinSize(fyne.NewSize(300, 10))

	cardContent := container.NewStack(
		cardBorder,
		cardBg,
		minSizer,
		container.New(layout.NewCustomPaddedLayout(16, 16, 16, 16), inner),
	)

	fadeOverlay := canvas.NewRectangle(bgCard)
	fadeOverlay.CornerRadius = 12

	animContainer := container.NewStack(cardContent, fadeOverlay)

	popup = widget.NewModalPopUp(animContainer, c)

	doClose := func() {
		go func() {
			fadeOverlay.Show()
			const dur = 160 * time.Millisecond
			start := time.Now()
			ticker := time.NewTicker(14 * time.Millisecond)
			defer ticker.Stop()
			for range ticker.C {
				p := math.Min(1.0, float64(time.Since(start))/float64(dur))
				ease := p * p
				a := uint8(255 * ease)
				fyne.Do(func() {
					fadeOverlay.FillColor = color.NRGBA{R: bgCard.R, G: bgCard.G, B: bgCard.B, A: a}
					fadeOverlay.Refresh()
				})
				if p >= 1 {
					break
				}
			}
			fyne.Do(popup.Hide)
		}()
	}

	closeBtn.OnTapped = doClose

	popup.Show()

	go func() {
		time.Sleep(16 * time.Millisecond)

		const dur = 200 * time.Millisecond
		start := time.Now()
		ticker := time.NewTicker(14 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			p := math.Min(1.0, float64(time.Since(start))/float64(dur))
			ease := 1 - math.Pow(1-p, 3)
			a := uint8(255 * (1 - ease))
			fyne.Do(func() {
				fadeOverlay.FillColor = color.NRGBA{R: bgCard.R, G: bgCard.G, B: bgCard.B, A: a}
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
}

func buildFooter() fyne.CanvasObject {
	link := widget.NewHyperlink("Telegram", telegramURL)
	ver := ctxt("zutil custom", textDim, 9)

	sep := canvas.NewRectangle(borderRed)
	sep.SetMinSize(fyne.NewSize(0, 1))
	footerBg := canvas.NewRectangle(bgCard)
	row := container.NewBorder(nil, nil, link, ver)
	inner := container.NewVBox(sep, container.New(layout.NewCustomPaddedLayout(6, 6, 10, 10), row))
	return container.NewStack(footerBg, inner)
}

func ctxt(s string, c color.Color, size float32) *canvas.Text {
	t := canvas.NewText(s, c)
	t.TextSize = size
	return t
}

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
func (app *App) createNoFireModule(proc *win.Process) module.Module {
	return modules.NoFire{Process: proc, Error: app.onError("no_fire"), AfterChange: app.autoSave}.Create()
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
