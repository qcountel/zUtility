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
	"gioui.org/widget/material"

	"github.com/something-that-is-cool/zutil/app/module/modules/modulesutil"
)

func (app *App) layoutModuleOverlay(gtx layout.Context, th *material.Theme) layout.Dimensions {
	m := app.activeOverlayModule
	if m == nil {
		return layout.Dimensions{}
	}

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
									titleLbl := material.Body1(th, strings.ToLower(m.Name())+" settings")
									titleLbl.Font.Weight = font.Bold
									return titleLbl.Layout(gtx)
								})
							}),
							layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								descLbl := material.Body2(th, m.Description())
								return descLbl.Layout(gtx)
							}),
							layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if _, isToggleable := m.(modulesutil.ToggleableModule); isToggleable {
									if app.bindButtonClick.Clicked(gtx) {
										app.showBindOverlay = true
										app.activeBindModule = m
										app.activeBindModuleConf = app.activeOverlayConfig
										app.showModuleOverlay = false
									}
									return layoutFullWidthButton(gtx, th, &app.bindButtonClick, "Bind Key")
								}
								return layout.Dimensions{}
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if _, isToggleable := m.(modulesutil.ToggleableModule); isToggleable {
									return layout.Spacer{Height: unit.Dp(12)}.Layout(gtx)
								}
								return layout.Dimensions{}
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if app.closeOverlayClick.Clicked(gtx) {
									app.showModuleOverlay = false
								}
								return layoutFullWidthButton(gtx, th, &app.closeOverlayClick, "Close")
							}),
						)
					})
				}),
			)
		}),
	)
}
