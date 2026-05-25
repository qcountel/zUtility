package app

import (
	"fmt"
	"image/color"
	"math"
	"net/url"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/something-that-is-cool/zutil/app/module/modules/modulesutil"
	"github.com/something-that-is-cool/zutil/internal/pkg/fyneutil"
)

func (app *App) ShowSettings() {
	app.animateToSettings()
}

func (app *App) HideSettings() {
	if app.win == nil {
		return
	}

	// No animations: instant content swap.
	if !app.animationsEnabled {
		app.showSettings = false
		fyne.Do(func() {
			nc, _, _ := app.createContent(app.tr.Process())
			app.win.SetContent(nc)
		})
		return
	}

	current := app.win.Content()
	overlay := canvas.NewRectangle(color.NRGBA{R: 0x08, G: 0x08, B: 0x08, A: 0x00})
	app.win.SetContent(container.NewStack(current, overlay))

	go func() {
		const dur1 = 180 * time.Millisecond
		start := time.Now()
		ticker := time.NewTicker(14 * time.Millisecond)
		for range ticker.C {
			p := math.Min(1.0, float64(time.Since(start))/float64(dur1))
			a := uint8(255 * (p * p))
			fyne.Do(func() {
				overlay.FillColor = color.NRGBA{R: 0x08, G: 0x08, B: 0x08, A: a}
				overlay.Refresh()
			})
			if p >= 1 {
				break
			}
		}
		ticker.Stop()

		app.showSettings = false
		fyne.Do(func() {
			nc, _, _ := app.createContent(app.tr.Process())
			newOverlay := canvas.NewRectangle(color.NRGBA{R: 0x08, G: 0x08, B: 0x08, A: 0xFF})
			app.win.SetContent(container.NewStack(nc, newOverlay))

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
						newOverlay.FillColor = color.NRGBA{R: 0x08, G: 0x08, B: 0x08, A: a}
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

func (app *App) rebuildAfterPaletteChange() {
	fyneutil.AccentColor = accentRed
	app.app.Settings().SetTheme(&fyneutil.CrimsonDarkTheme{})
	fyne.Do(func() {
		nc, _, _ := app.createContent(app.tr.Process())
		app.win.SetContent(nc)
	})
}

func (app *App) createSettingsContent() fyne.CanvasObject {
	app.showSettings = true

	backBtn := widget.NewButton("← Назад", func() {
		app.HideSettings()
	})
	backBtn.Importance = widget.LowImportance

	titleText := ctxt("SETTINGS", textPrimary, 26)
	titleText.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}

	titleAccent := canvas.NewRectangle(accentRed)
	titleAccent.SetMinSize(fyne.NewSize(40, 2))

	headerInner := container.NewVBox(
		container.NewHBox(backBtn),
		container.New(layout.NewCustomPaddedLayout(4, 2, 0, 0), titleText),
		container.New(layout.NewCustomPaddedLayout(0, 6, 0, 0), titleAccent),
	)

	headerBg := canvas.NewRectangle(bgCard)
	sepLine := canvas.NewRectangle(borderRed)
	sepLine.SetMinSize(fyne.NewSize(0, 1))

	headerPadded := container.New(layout.NewCustomPaddedLayout(10, 10, 14, 14), headerInner)
	headerContent := container.NewBorder(nil, sepLine, nil, nil, headerPadded)
	header := container.NewStack(headerBg, headerContent)

	// ─── Color Picker Section ─────────────────────────────────────
	var colorDots []fyne.CanvasObject
	for _, p := range Palettes {
		pal := p // capture
		dot := canvas.NewRectangle(pal.Accent)
		dot.SetMinSize(fyne.NewSize(18, 18))

		if pal.Name == CurrentPaletteName() {
			dot.StrokeColor = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
			dot.StrokeWidth = 2
		}

		tap := newNavTapArea(func() {
			SetPalette(pal.Name)
			_ = app.SaveConfig()
			app.rebuildAfterPaletteChange()
		})

		dotContainer := container.NewStack(dot, tap)
		colorDots = append(colorDots, container.New(layout.NewCustomPaddedLayout(2, 2, 2, 2), dotContainer))
	}
	colorRow := container.NewHBox(colorDots...)
	colorSection := createSettingsSection(
		"Цвет акцента",
		colorRow,
	)

	// ─── System Section ───────────────────────────────────────────
	trayToggle := modulesutil.NewM3Toggle(app.minimizeToTray)
	trayToggle.OnChange = func(checked bool) {
		app.minimizeToTray = checked
		_ = app.SaveConfig()
	}
	trayRow := container.NewBorder(nil, nil, nil, trayToggle,
		ctxt("Сворачивать в трей при закрытии", textPrimary, 14),
	)

	animToggle := modulesutil.NewM3Toggle(app.animationsEnabled)
	animToggle.OnChange = func(checked bool) {
		app.animationsEnabled = checked
		AnimationsEnabled = checked
		_ = app.SaveConfig()
	}
	animRow := container.NewBorder(nil, nil, nil, animToggle,
		ctxt("Анимации интерфейса", textPrimary, 14),
	)

	systemSection := createSettingsSection(
		"Система",
		container.NewVBox(
			container.New(layout.NewCustomPaddedLayout(4, 4, 0, 0), trayRow),
			container.New(layout.NewCustomPaddedLayout(4, 4, 0, 0), animRow),
		),
	)

	// ─── Session Section ──────────────────────────────────────────
	uptimeLabel := widget.NewLabel(app.SessionUptimeText())
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-app.ctx.Done():
				return
			case <-ticker.C:
				if !app.showSettings {
					return
				}
				fyne.Do(func() {
					uptimeLabel.SetText(app.SessionUptimeText())
				})
			}
		}
	}()

	sessionSection := createSettingsSection(
		"Статистика сессии",
		container.NewVBox(
			ctxt("Время работы утилиты", textSecondary, 13),
			uptimeLabel,
		),
	)

	// ─── Updates Section ──────────────────────────────────────────
	updateStatus := widget.NewLabel("")
	updateStatus.Wrapping = fyne.TextWrapWord
	updateBtn := widget.NewButton("Проверить", nil)
	openReleaseBtn := widget.NewButton("Открыть релиз", nil)
	openReleaseBtn.Importance = widget.LowImportance
	var applyUpdateState func(UpdateStatus)
	applyUpdateState = func(status UpdateStatus) {
		recheck := func() {
			app.StartUpdateCheck(func(next UpdateStatus) {
				fyne.Do(func() {
					applyUpdateState(next)
				})
			})
		}

		switch {
		case status.Checking:
			updateStatus.SetText("Проверяем GitHub Releases...")
			updateBtn.SetText("Проверка...")
			updateBtn.Disable()
			openReleaseBtn.Disable()
			openReleaseBtn.OnTapped = nil
		case status.Downloading:
			updateStatus.SetText("Скачиваем обновление и заменяем текущий .exe...")
			updateBtn.SetText("Скачивание...")
			updateBtn.Disable()
			openReleaseBtn.Disable()
		case status.Error != "":
			updateStatus.SetText("Не удалось проверить обновления: " + status.Error)
			updateBtn.SetText("Повторить")
			updateBtn.OnTapped = recheck
			updateBtn.Enable()
			if status.ReleaseURL != "" {
				openReleaseBtn.OnTapped = func() {
					if err := app.OpenReleaseURL(status.ReleaseURL); err != nil {
						app.conf.Logger.Error("open release url", "err", err)
					}
				}
				openReleaseBtn.Enable()
			} else {
				openReleaseBtn.Disable()
			}
		case status.HasUpdate:
			updateStatus.SetText(fmt.Sprintf("Доступна новая версия: %s (у вас %s)", status.LatestVersion, CurrentVersion))
			updateBtn.SetText("Скачать .exe")
			updateBtn.OnTapped = func() {
				app.DownloadLatestReleaseAsset(func(next UpdateStatus) {
					fyne.Do(func() {
						applyUpdateState(next)
					})
				})
			}
			updateBtn.Enable()
			openReleaseBtn.OnTapped = func() {
				if err := app.OpenReleaseURL(status.ReleaseURL); err != nil {
					app.conf.Logger.Error("open release url", "err", err)
				}
			}
			openReleaseBtn.Enable()
		case status.CheckedAt.IsZero():
			updateStatus.SetText("Автопроверка ещё не завершилась.")
			updateBtn.SetText("Проверить")
			updateBtn.OnTapped = recheck
			updateBtn.Enable()
			openReleaseBtn.Disable()
		default:
			updateStatus.SetText(fmt.Sprintf("Установлена актуальная версия: %s", CurrentVersion))
			updateBtn.SetText("Проверить снова")
			updateBtn.OnTapped = recheck
			updateBtn.Enable()
			openReleaseBtn.OnTapped = func() {
				if err := app.OpenReleaseURL(status.ReleaseURL); err != nil {
					app.conf.Logger.Error("open release url", "err", err)
				}
			}
			if status.ReleaseURL != "" {
				openReleaseBtn.Enable()
			} else {
				openReleaseBtn.Disable()
			}
		}
	}
	applyUpdateState(app.UpdateStatus())

	updatesSection := createSettingsSection(
		"Обновления",
		container.NewVBox(
			ctxt("Текущая версия: "+CurrentVersion, textSecondary, 13),
			updateStatus,
			container.New(layout.NewCustomPaddedLayout(0, 4, 0, 0), container.NewHBox(updateBtn, openReleaseBtn)),
		),
	)

	// ─── Resource packs Section ───────────────────────────────────
	clonedToggle := modulesutil.NewM3Toggle(app.useClonedMinecraft)
	clonedToggle.OnChange = func(checked bool) {
		app.useClonedMinecraft = checked
		_ = app.SaveConfig()
	}
	clonedRow := container.NewBorder(nil, nil, nil, clonedToggle,
		ctxt("Использую клонированный Майнкрафт", textPrimary, 14),
	)
	clonedDesc := ctxt("Ресурспаки сохраняются только в папку MC:PE 1.1.5", textDim, 13)

	packsSection := createSettingsSection(
		"Ресурспаки",
		container.NewVBox(
			container.New(layout.NewCustomPaddedLayout(4, 2, 0, 0), clonedRow),
			container.New(layout.NewCustomPaddedLayout(0, 4, 0, 0), clonedDesc),
		),
	)

	// ─── About Section ────────────────────────────────────────────
	aboutLines := []fyne.CanvasObject{
		ctxt("zUtility • версия "+CurrentVersion, textSecondary, 14),
		ctxt("Утилита для Minecraft Pocket Edition", textDim, 13),
		settingsDivider(),
		func() fyne.CanvasObject {
			urlK4ties, _ := url.Parse("https://t.me/nigger1790")
			urlAnx1ous, _ := url.Parse("https://t.me/anx1ous")
			return widget.NewRichText(
				&widget.TextSegment{
					Text:  "Авторы: ",
					Style: widget.RichTextStyle{ColorName: theme.ColorNameDisabled, Inline: true},
				},
				&widget.HyperlinkSegment{Text: "k4ties", URL: urlK4ties},
				&widget.TextSegment{
					Text:  " & ",
					Style: widget.RichTextStyle{ColorName: theme.ColorNameDisabled, Inline: true},
				},
				&widget.HyperlinkSegment{Text: "anx1ous", URL: urlAnx1ous},
			)
		}(),
		settingsDivider(),
		ctxt("© 2026 ZUTIL · All rights reserved", textDim, 12),
	}
	aboutSection := createSettingsSection("О приложении", container.NewVBox(aboutLines...))

	mainContent := container.NewVBox(header, colorSection, systemSection, sessionSection, updatesSection, packsSection, aboutSection)
	bg := canvas.NewRectangle(bgPrimary)
	scroll := container.NewVScroll(mainContent)
	return container.NewStack(bg, scroll)
}

func createSettingsSection(title string, content fyne.CanvasObject) fyne.CanvasObject {
	dot := canvas.NewRectangle(accentRed)
	dot.SetMinSize(fyne.NewSize(3, 14))

	titleLabel := ctxt(title, textPrimary, 15)
	titleLabel.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}

	titleRow := container.NewHBox(dot, container.New(layout.NewCustomPaddedLayout(0, 0, 5, 0), titleLabel))

	inner := container.NewVBox(
		titleRow,
		container.New(layout.NewCustomPaddedLayout(0, 6, 0, 0), settingsDivider()),
		content,
	)

	cardBg := canvas.NewRectangle(bgCard)
	cardBg.CornerRadius = 3
	cardBorder := canvas.NewRectangle(borderRed)
	cardBorder.CornerRadius = 3

	paddedInner := container.New(layout.NewCustomPaddedLayout(12, 12, 14, 14), inner)
	card := container.NewStack(cardBorder, cardBg, paddedInner)
	return container.New(layout.NewCustomPaddedLayout(0, 8, 12, 12), card)
}

func createSettingsBtn(text string, labelColor color.NRGBA, onTap func()) fyne.CanvasObject {
	btn := NewM3AnimatedButton(text, onTap)
	btn.Importance = widget.LowImportance

	bg := canvas.NewRectangle(calpha(labelColor, 0x18))

	lbl := ctxt(text, labelColor, 14)
	lbl.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}

	return container.New(layout.NewCustomPaddedLayout(2, 2, 4, 4),
		container.NewStack(bg, btn),
	)
}

func settingsDivider() fyne.CanvasObject {
	d := canvas.NewRectangle(borderRed)
	d.SetMinSize(fyne.NewSize(0, 1))
	return d
}
