package app

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"io"
	"os"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/something-that-is-cool/zutil/app/module"
	"github.com/something-that-is-cool/zutil/internal/misc"
	"github.com/something-that-is-cool/zutil/internal/version"
	"github.com/something-that-is-cool/zutil/pkg/e"

	"github.com/sqweek/dialog"
)

var aboutMessage = misc.JoinNewLine(
	"zutil (MC:PE 1.1.5)",
	"build "+version.Version+" "+"("+version.Commit+")",
	"",
	"made by k4ties, anx1ous",
	"",
	"Copyright (C) 2026 Ivan Z. All rights reserved.",
)

func (app *App) layoutSettingsOverlay(gtx layout.Context, th *material.Theme) layout.Dimensions {
	gtx.Constraints.Min = gtx.Constraints.Max

	return layout.Stack{Alignment: layout.Center}.Layout(gtx,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return app.blockerClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				backdropColor := color.NRGBA{R: 0, G: 0, B: 0, A: 160}
				paint.Fill(gtx.Ops, backdropColor)
				return layout.Dimensions{Size: gtx.Constraints.Min}
			})
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			cardBg := app.themeCardBg()

			gtx.Constraints.Min.X = gtx.Dp(320)
			gtx.Constraints.Max.X = gtx.Dp(320)

			return layout.Stack{}.Layout(gtx,
				layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					d := image.Rectangle{Max: gtx.Constraints.Min}
					paint.FillShape(gtx.Ops, cardBg, clip.RRect{
						Rect: d,
						NE:   16, NW: 16, SE: 16, SW: 16,
					}.Op(gtx.Ops))
					return layout.Dimensions{Size: gtx.Constraints.Min}
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(20)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									titleLbl := material.Body1(th, "Settings")
									titleLbl.Font.Weight = font.Bold
									return titleLbl.Layout(gtx)
								})
							}),
							layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if app.importConfigClick.Clicked(gtx) {
									app.importConfig()
								}
								return layoutFullWidthButton(gtx, th, &app.importConfigClick, "Import config")
							}),
							layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if app.exportConfigClick.Clicked(gtx) {
									app.exportConfig()
								}
								return layoutFullWidthButton(gtx, th, &app.exportConfigClick, "Export config")
							}),
							layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if app.resetConfigClick.Clicked(gtx) {
									app.resetConfig()
								}
								return layoutFullWidthButton(gtx, th, &app.resetConfigClick, "Reset config")
							}),
							layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								app.showErrorsCheck.Value = app.userConf.V.ShowErrors
								cb := material.CheckBox(th, &app.showErrorsCheck, "Show errors")
								dims := cb.Layout(gtx)
								if app.showErrorsCheck.Value != app.userConf.V.ShowErrors {
									app.showErrors(app.showErrorsCheck.Value, ActionCauseUserInput)
								}
								return dims
							}),
							layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if app.toggleThemeClick.Clicked(gtx) {
									app.lightTheme = !app.lightTheme
									app.userConf.Lock()
									app.userConf.V.LightTheme = app.lightTheme
									app.userConf.Unlock()
									app.applyWindowsDarkMode()
								}
								label := "Switch to Light Theme"
								if app.lightTheme {
									label = "Switch to Dark Theme"
								}
								return layoutFullWidthButton(gtx, th, &app.toggleThemeClick, label)
							}),
							layout.Rigid(layout.Spacer{Height: unit.Dp(20)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
									layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
										return layout.Inset{Right: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											if app.aboutClick.Clicked(gtx) {
												app.showInfo("About", aboutMessage)
											}
											return layoutFullWidthButton(gtx, th, &app.aboutClick, "About")
										})
									}),
									layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
										return layout.Inset{Left: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											if app.closeOverlayClick.Clicked(gtx) {
												app.showSettingsOverlay = false
											}
											return layoutFullWidthButton(gtx, th, &app.closeOverlayClick, "Close")
										})
									}),
								)
							}),
						)
					})
				}),
			)
		}),
	)
}

func (app *App) importConfig() {
	filename, err := dialog.File().Filter("JSON Config", "json").Load()
	if err != nil {
		if err.Error() == "Cancelled" {
			return
		}
		app.showError("import config", err)
		return
	}
	reader, err := os.Open(filename)
	if err != nil {
		app.showError("import config", err)
		return
	}
	defer reader.Close()
	if err = app.doImport(reader); err != nil {
		app.showError("import config", err)
		return
	}
	app.showInfo("Import", misc.JoinNewLine(
		"successfully imported config.",
		filename,
	))
}

var actionCauseImportedConfig = e.NewActionCause("imported config")

func (app *App) doImport(reader io.Reader) error {
	d, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("read all (reader): %w", err)
	}
	var conf *UserConfig
	if err = json.Unmarshal(d, &conf); err != nil {
		return fmt.Errorf("unmarshal json: %w", err)
	}
	toEdit := app.applyConfig(conf)
	app.doModuleUpdates(toEdit, actionCauseImportedConfig)
	return nil
}

func (app *App) exportConfig() {
	filename, err := dialog.File().Filter("JSON Config", "json").Save()
	if err != nil {
		if err.Error() == "Cancelled" {
			return
		}
		app.showError("export config", err)
		return
	}
	writer, err := os.Create(filename)
	if err != nil {
		app.showError("export config", err)
		return
	}
	defer writer.Close()
	if err = app.doExport(writer); err != nil {
		app.showError("export config", err)
		return
	}
	app.showInfo("Export config", misc.JoinNewLine(
		"successfully exported config.",
		filename,
	))
}

func (app *App) doExport(writer io.Writer) error {
	app.userConf.Lock()
	defer app.userConf.Unlock()

	d, err := json.MarshalIndent(app.userConf.V, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config to json: %w", err)
	}
	if _, err = writer.Write(d); err != nil {
		return fmt.Errorf("export: write config to file: %w", err)
	}
	return nil
}

var actionCauseResetConfig = e.NewActionCause("reset config")

func (app *App) resetConfig() {
	toEdit := app.applyConfig(DefaultUserConfig())
	app.doModuleUpdates(toEdit, actionCauseResetConfig)
}

func (app *App) showErrors(state bool, _ e.ActionCause) {
	app.userConf.Lock()
	defer app.userConf.Unlock()
	app.userConf.V.ShowErrors = state
}

func (app *App) applyConfig(newConf *UserConfig) map[module.Module]module.Property {
	app.userConf.Lock()
	defer app.userConf.Unlock()
	app.userConf.V = newConf
	app.lightTheme = app.userConf.V.LightTheme

	app.data.Lock()
	defer app.data.Unlock()

	func() {
		app.hm.Events.Lock()
		defer app.hm.Events.Unlock()
		app.hm.ClearHandlersUnsafe()
		app.applyBindsUnsafe(app.userConf.V)
	}()
	toEdit := make(map[module.Module]module.Property)
	for conf, m := range app.data.V.modules.AllFromFront() {
		property := conf.DefaultProperty()
		if p, ok := newConf.Modules[conf.Identifier()]; ok {
			property = p
		}
		app.userConf.V.Modules[conf.Identifier()] = property
		toEdit[m] = property
	}
	return toEdit
}

func (app *App) applyBindsUnsafe(conf *UserConfig) {
	for mod, char := range conf.Binds {
		m, ok := app.moduleByIDUnsafe(mod)
		if !ok {
			continue
		}
		app.hm.HandleUnsafe(char, app.bindToggleModule(mod, m))
	}
}
