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

type preset struct {
	Name    string
	R, G, B float64
	Color   color.NRGBA
}

var presets = []preset{
	{Name: "Neon Green", R: 0.0, G: 1.0, B: 0.0, Color: color.NRGBA{R: 0x00, G: 0xFF, B: 0x00, A: 0xFF}},
	{Name: "Cosmic Purple", R: 0.7, G: 0.0, B: 1.0, Color: color.NRGBA{R: 0xB2, G: 0x00, B: 0xFF, A: 0xFF}},
	{Name: "Sunset Red", R: 1.0, G: 0.2, B: 0.0, Color: color.NRGBA{R: 0xFF, G: 0x33, B: 0x00, A: 0xFF}},
	{Name: "Deep Blue", R: 0.0, G: 0.2, B: 1.0, Color: color.NRGBA{R: 0x00, G: 0x33, B: 0xFF, A: 0xFF}},
	{Name: "Cyber Pink", R: 1.0, G: 0.0, B: 0.5, Color: color.NRGBA{R: 0xFF, G: 0x00, B: 0x80, A: 0xFF}},
	{Name: "Pure Black", R: 0.0, G: 0.0, B: 0.0, Color: color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xFF}},
}

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
									titleLbl := material.Body1(th, strings.ToUpper(m.Name())+" SETTINGS")
									titleLbl.Font.Typeface = "monospace"
									titleLbl.Font.Weight = font.Bold
									return titleLbl.Layout(gtx)
								})
							}),
							layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								descLbl := material.Body2(th, m.Description())
								descLbl.Font.Typeface = "monospace"
								return descLbl.Layout(gtx)
							}),
							layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
							
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if app.activeOverlayConfig.Identifier() == "custom_sky" {
									return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											lbl := material.Body2(th, "COLOR PRESETS:")
											lbl.Font.Typeface = "monospace"
											lbl.Font.Weight = font.Bold
											return lbl.Layout(gtx)
										}),
										layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
												layout.Rigid(func(gtx layout.Context) layout.Dimensions {
													return layout.Flex{}.Layout(gtx,
														layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
															return app.layoutPresetButton(gtx, th, 0)
														}),
														layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
														layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
															return app.layoutPresetButton(gtx, th, 1)
														}),
														layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
														layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
															return app.layoutPresetButton(gtx, th, 2)
														}),
													)
												}),
												layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
												layout.Rigid(func(gtx layout.Context) layout.Dimensions {
													return layout.Flex{}.Layout(gtx,
														layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
															return app.layoutPresetButton(gtx, th, 3)
														}),
														layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
														layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
															return app.layoutPresetButton(gtx, th, 4)
														}),
														layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
														layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
															return app.layoutPresetButton(gtx, th, 5)
														}),
													)
												}),
											)
										}),
										layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
									)
								}
								return layout.Dimensions{}
							}),

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

func (app *App) layoutPresetButton(gtx layout.Context, th *material.Theme, index int) layout.Dimensions {
	p := presets[index]
	click := &app.presetClicks[index]

	if click.Clicked(gtx) {
		app.setCustomSkyPreset(p.R, p.G, p.B)
	}

	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		d := image.Rectangle{Max: image.Point{X: gtx.Constraints.Max.X, Y: gtx.Dp(36)}}
		r := gtx.Dp(4)
		
		paint.FillShape(gtx.Ops, p.Color, clip.RRect{Rect: d, NE: r, NW: r, SE: r, SW: r}.Op(gtx.Ops))
		
		borderColor := color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x40}
		if click.Hovered() {
			borderColor = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
		}
		strokeWidth := gtx.Dp(1)
		cl := clip.Stroke{
			Path:  clip.RRect{Rect: d, NE: r, NW: r, SE: r, SW: r}.Path(gtx.Ops),
			Width: float32(strokeWidth),
		}.Op().Push(gtx.Ops)
		paint.ColorOp{Color: borderColor}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		cl.Pop()

		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			textColor := color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
			if p.R > 0.6 && p.G > 0.6 && p.B > 0.6 {
				textColor = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xFF}
			} else if p.Color.R == 0 && p.Color.G == 0xFF && p.Color.B == 0 {
				textColor = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xFF}
			}
			lbl := material.Caption(th, p.Name)
			lbl.Color = textColor
			lbl.Font.Typeface = "monospace"
			lbl.Font.Weight = font.Bold
			return lbl.Layout(gtx)
		})
	})
}
