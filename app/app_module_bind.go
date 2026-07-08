package app

import (
	"image"
	"image/color"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/something-that-is-cool/zutil/app/module"
	"github.com/something-that-is-cool/zutil/app/module/modules/modulesutil"
	"github.com/something-that-is-cool/zutil/pkg/win/cursor"
	"github.com/something-that-is-cool/zutil/pkg/win/hotkey"
)

func (app *App) layoutBindOverlay(gtx layout.Context, th *material.Theme) layout.Dimensions {
	m := app.activeBindModule
	c := app.activeBindModuleConf
	if m == nil || c == nil {
		return layout.Dimensions{}
	}

	id := c.Identifier()

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

			gtx.Constraints.Min = image.Point{X: gtx.Dp(360), Y: gtx.Dp(480)}
			gtx.Constraints.Max = gtx.Constraints.Min

			return layout.Stack{}.Layout(gtx,
				layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					d := image.Rectangle{Max: gtx.Constraints.Min}
					paint.FillShape(gtx.Ops, cardBg, clip.Rect(d).Op())

					// Thick border (2dp)
					strokeWidth := gtx.Dp(2)
					cl := clip.Stroke{
						Path:  clip.RRect{Rect: d}.Path(gtx.Ops),
						Width: float32(strokeWidth),
					}.Op().Push(gtx.Ops)
					paint.ColorOp{Color: th.Palette.ContrastBg}.Add(gtx.Ops)
					paint.PaintOp{}.Add(gtx.Ops)
					cl.Pop()

					return layout.Dimensions{Size: gtx.Constraints.Min}
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(20)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									titleLbl := material.Body1(th, "BIND "+strings.ToUpper(m.Name()))
									titleLbl.Font.Typeface = "monospace"
									titleLbl.Font.Weight = font.Bold
									return titleLbl.Layout(gtx)
								})
							}),
							layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
									layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
										curLbl := material.Body2(th, "CURRENT BIND: "+app.currentBind(id))
										curLbl.Font.Typeface = "monospace"
										return curLbl.Layout(gtx)
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										if app.resetBindClick.Clicked(gtx) {
											app.resetBind(id)
											app.showBindOverlay = false
										}
										btn := material.Button(th, &app.resetBindClick, "RESET")
										btn.CornerRadius = unit.Dp(0)
										btn.Font.Typeface = "monospace"
										return btn.Layout(gtx)
									}),
								)
							}),
							layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
							layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
								// Render characters list to choose from
								chars := hotkey.AllChars
								return app.bindListState.Layout(gtx, len(chars), func(gtx layout.Context, index int) layout.Dimensions {
									char := chars[index]
									click, exists := app.charBindClicks[char]
									if !exists {
										click = &widget.Clickable{}
										app.charBindClicks[char] = click
									}
									if click.Clicked(gtx) {
										app.bindModuleTo(m, c, char)
										app.showBindOverlay = false
									}
									return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										return layoutFullWidthButton(gtx, th, click, char)
									})
								})
							}),
							layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if app.closeBindClick.Clicked(gtx) {
									app.showBindOverlay = false
								}
								return layoutFullWidthButton(gtx, th, &app.closeBindClick, "Close")
							}),
						)
					})
				}),
			)
		}),
	)
}

func (app *App) bindModuleTo(m module.Module, c module.Config, char string) {
	app.userConf.Lock()
	defer app.userConf.Unlock()

	if mod, ok := app.userConf.V.CharBinds[char]; ok {
		delete(app.userConf.V.Binds, mod)
	}
	app.userConf.V.Binds[c.Identifier()] = char
	app.userConf.V.CharBinds[char] = c.Identifier()

	app.hm.Handle(char, app.bindToggleModule(c.Identifier(), m))
}

func (app *App) currentBind(id string) string {
	app.userConf.Lock()
	defer app.userConf.Unlock()

	b, ok := app.userConf.V.Binds[id]
	if !ok {
		return "not set"
	}
	return b
}

func (app *App) resetBind(id string) {
	app.userConf.Lock()
	defer app.userConf.Unlock()

	h, ok := app.userConf.V.Binds[id]
	if !ok {
		return
	}
	delete(app.userConf.V.Binds, id)
	app.hm.DeleteHandler(h)
}

func (app *App) bindToggleModule(id string, m module.Module) func() {
	return func() {
		if id == "zoom" {
			return
		}
		if !cursor.Focused() {
			return
		}
		app.data.Lock()
		defer app.data.Unlock()

		t, ok := m.(modulesutil.ToggleableModule)
		if !ok {
			return
		}
		v := t.State()

		// Update state directly without blocking, and invalidate window to repaint
		if err := t.UpdateState(!v, actionCauseModuleToggledByBind); err != nil {
			app.conf.Logger.Error("cannot toggle module state", "id", id)
		}
		if app.win != nil {
			app.win.Invalidate()
		}
	}
}

func (app *App) loadBinds(conf *UserConfig) map[string]func() {
	x := make(map[string]func())
	for id, key := range conf.Binds {
		m, ok := app.moduleByIDUnsafe(id)
		if !ok {
			continue
		}
		if _, ok = m.(modulesutil.ToggleableModule); !ok {
			continue
		}
		x[key] = app.bindToggleModule(id, m)
		conf.CharBinds[key] = id
	}
	return x
}
