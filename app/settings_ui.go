package app

import (
	"context"
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/something-that-is-cool/zutil/app/module/modules/modulesutil"
)

func (app *App) createSettingsContent() fyne.CanvasObject {
	titleText := ctxt(app.t("Настройки", "Settings"), textPrimary, 18)
	titleText.TextStyle = fyne.TextStyle{Bold: true}

	titleAccent := canvas.NewRectangle(accentRed)
	titleAccent.SetMinSize(fyne.NewSize(36, 2))
	titleAccent.CornerRadius = 1

	trayToggle := modulesutil.NewM3Toggle(app.minimizeToTray)
	trayToggle.OnChange = func(checked bool) {
		app.minimizeToTray = checked
		_ = app.SaveConfig()
	}
	trayRow := container.NewBorder(nil, nil, nil, trayToggle,
		ctxt(app.t("Сворачивать в трей при закрытии", "Minimize to tray on close"), textPrimary, 13),
	)

	bindLabel := ctxt(app.t("Горячая клавиша", "Hotkey"), textPrimary, 13)
	bindDesc := ctxt(app.t("Показывает/скрывает окно утилиты.", "Shows/hides the utility window."), textDim, 11)

	keyName := app.t("Не задано", "Not set")
	if app.showHotkey != 0 {
		keyName = VKName(app.showHotkey)
	}

	keyBadgeBg := canvas.NewRectangle(calpha(accentRed, 0x22))
	keyBadgeBg.CornerRadius = 6
	keyBadgeText := ctxt(keyName, accentRedBright, 12)
	keyBadgeText.TextStyle = fyne.TextStyle{Bold: true}
	keyBadge := container.NewStack(
		keyBadgeBg,
		container.New(layout.NewCustomPaddedLayout(4, 4, 8, 8), keyBadgeText),
	)

	clearBtn := widget.NewButtonWithIcon("", theme.ContentClearIcon(), nil)
	clearBtn.Importance = widget.LowImportance

	bindBtn := widget.NewButton(app.t("Изменить", "Change"), nil)
	bindBtn.Importance = widget.LowImportance

	var captureCancel context.CancelFunc

	clearBtn.OnTapped = func() {
		if captureCancel != nil {
			captureCancel()
			captureCancel = nil
		}
		app.showHotkey = 0
		keyBadgeText.Text = app.t("Не задано", "Not set")
		keyBadgeText.Color = textSecondary
		keyBadgeText.Refresh()
		keyBadgeBg.FillColor = calpha(accentRedDim, 0x15)
		keyBadgeBg.Refresh()
		bindBtn.SetText(app.t("Изменить", "Change"))
		_ = app.SaveConfig()
	}

	bindBtn.OnTapped = func() {
		if captureCancel != nil {
			captureCancel()
			captureCancel = nil
			bindBtn.SetText(app.t("Изменить", "Change"))
			if app.showHotkey != 0 {
				keyBadgeText.Text = VKName(app.showHotkey)
				keyBadgeText.Color = accentRedBright
			} else {
				keyBadgeText.Text = app.t("Не задано", "Not set")
				keyBadgeText.Color = textSecondary
			}
			keyBadgeText.Refresh()
			return
		}

		hotkeyCapturing.Store(true)
		bindBtn.SetText(app.t("Отмена", "Cancel"))
		keyBadgeText.Text = app.t("Нажмите клавишу...", "Press a key...")
		keyBadgeText.Color = accentOrange
		keyBadgeText.Refresh()
		keyBadgeBg.FillColor = calpha(accentOrange, 0x20)
		keyBadgeBg.Refresh()

		captureCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		captureCancel = cancel

		go func() {
			defer func() {
				hotkeyCapturing.Store(false)
				cancel()
				captureCancel = nil
			}()

			vk, ok := CaptureNextKey(captureCtx)

			fyne.Do(func() {
				if ok && vk != 0 {
					app.showHotkey = vk
					keyBadgeText.Text = VKName(vk)
					keyBadgeText.Color = accentRedBright
					keyBadgeBg.FillColor = calpha(accentRed, 0x22)
					_ = app.SaveConfig()
				} else {
					if app.showHotkey != 0 {
						keyBadgeText.Text = VKName(app.showHotkey)
						keyBadgeText.Color = accentRedBright
					} else {
						keyBadgeText.Text = app.t("Не задано", "Not set")
						keyBadgeText.Color = textSecondary
					}
					keyBadgeBg.FillColor = calpha(accentRed, 0x22)
				}
				keyBadgeText.Refresh()
				keyBadgeBg.Refresh()
				bindBtn.SetText(app.t("Изменить", "Change"))
			})
		}()
	}

	bindRow := container.NewBorder(nil, nil, bindLabel, container.NewHBox(keyBadge, clearBtn, bindBtn))
	bindBlock := container.NewVBox(
		container.New(layout.NewCustomPaddedLayout(4, 2, 0, 0), bindRow),
		container.New(layout.NewCustomPaddedLayout(0, 4, 0, 0), bindDesc),
	)

	langLabel := ctxt(app.t("Язык интерфейса", "Interface language"), textPrimary, 13)
	ruBtn := widget.NewButton("RU", nil)
	enBtn := widget.NewButton("EN", nil)
	ruBtn.Importance = widget.LowImportance
	enBtn.Importance = widget.LowImportance

	applyLang := func(lang string) {
		lang = normalizeLanguage(lang)
		if app.language == lang {
			return
		}
		app.language = lang
		app.updateTrayMenu()
		_ = app.SaveConfig()
		if app.win != nil {
			nc, _, _ := app.createContent(app.tr.Process())
			app.win.SetContent(nc)
			app.win.SetFixedSize(true) // переустанавливаем после SetContent — без изменения размера
		}
	}

	ruBtn.OnTapped = func() { applyLang(langRU) }
	enBtn.OnTapped = func() { applyLang(langEN) }

	if normalizeLanguage(app.language) == langEN {
		enBtn.Importance = widget.HighImportance
	} else {
		ruBtn.Importance = widget.HighImportance
	}

	langRow := container.NewBorder(nil, nil, langLabel, container.NewHBox(ruBtn, enBtn))

	systemSection := createSettingsSection(
		app.t("Система", "System"),
		container.NewVBox(
			container.New(layout.NewCustomPaddedLayout(4, 4, 0, 0), trayRow),
			settingsDivider(),
			bindBlock,
			settingsDivider(),
			container.New(layout.NewCustomPaddedLayout(4, 2, 0, 0), langRow),
		),
	)

	aboutLines := []fyne.CanvasObject{
		ctxt(app.t("zUtility • версия Custom", "zUtility • Custom version"), textSecondary, 13),
		ctxt(app.t("Утилита для Minecraft Pocket Edition", "Utility for Minecraft Pocket Edition"), textDim, 12),
		settingsDivider(),
		ctxt(app.t("Авторы: k4ties & anx1ous", "Authors: k4ties & anx1ous"), textDim, 12),
		settingsDivider(),
		ctxt("© 2026 ZUTIL · All rights reserved", textDim, 11),
	}
	aboutSection := createSettingsSection(app.t("О приложении", "About"), container.NewVBox(aboutLines...))

	header := container.NewVBox(
		container.New(layout.NewCustomPaddedLayout(16, 4, 14, 14), titleText),
		container.New(layout.NewCustomPaddedLayout(0, 10, 14, 14),
			container.New(layout.NewCustomPaddedLayout(0, 0, 0, 0), titleAccent)),
	)

	mainContent := container.NewVBox(header, systemSection, aboutSection)
	bg := canvas.NewRectangle(bgPrimary)
	scroll := container.NewVScroll(mainContent)
	return container.NewStack(bg, scroll)
}

func createSettingsSection(title string, content fyne.CanvasObject) fyne.CanvasObject {
	return createSettingsSectionObj(title, content)
}

func createSettingsSectionObj(title string, content fyne.CanvasObject) fyne.CanvasObject {
	dot := canvas.NewRectangle(accentRed)
	dot.SetMinSize(fyne.NewSize(3, 14))
	dot.CornerRadius = 2

	titleLabel := ctxt(title, textPrimary, 13)
	titleLabel.TextStyle = fyne.TextStyle{Bold: true}

	titleRow := container.NewHBox(dot, container.New(layout.NewCustomPaddedLayout(0, 0, 5, 0), titleLabel))

	inner := container.NewVBox(
		titleRow,
		container.New(layout.NewCustomPaddedLayout(0, 6, 0, 0), settingsDivider()),
		content,
	)

	cardBg := canvas.NewRectangle(bgCard)
	cardBg.CornerRadius = 10

	cardBorder := canvas.NewRectangle(borderRed)
	cardBorder.CornerRadius = 11

	paddedInner := container.New(layout.NewCustomPaddedLayout(12, 12, 14, 14), inner)
	card := container.NewStack(cardBorder, cardBg, paddedInner)
	return container.New(layout.NewCustomPaddedLayout(0, 8, 12, 12), card)
}

func createSettingsBtn(text string, labelColor color.NRGBA, onTap func()) fyne.CanvasObject {
	btn := NewM3AnimatedButton(text, onTap)
	btn.Importance = widget.LowImportance

	bg := canvas.NewRectangle(calpha(labelColor, 0x18))
	bg.CornerRadius = 8

	return container.New(layout.NewCustomPaddedLayout(2, 2, 4, 4),
		container.NewStack(bg, btn),
	)
}

func settingsDivider() fyne.CanvasObject {
	d := canvas.NewRectangle(borderRed)
	d.SetMinSize(fyne.NewSize(0, 1))
	return d
}
