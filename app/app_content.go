package app

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"os/exec"
	"strings"

	"gioui.org/font"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/something-that-is-cool/zutil/app/module"
	"github.com/something-that-is-cool/zutil/app/module/modules/modulesutil"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

type moduleEntry struct {
	Key       module.Config
	Value     module.Module
	NameLower string
}

func scaleFloat(val, min, max float32) float32 {
	return min + val*(max-min)
}

func descaleFloat(val, min, max float32) float32 {
	if max == min {
		return 0
	}
	return (val - min) / (max - min)
}

// Main layout method for the Gio window.
func (app *App) layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	app.syncGioTheme(th)

	paint.Fill(gtx.Ops, th.Palette.Bg)

	return layout.Stack{}.Layout(gtx,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return app.layoutHeader(gtx, th)
				}),
				layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
					return app.layoutModulesList(gtx, th)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return app.layoutFooter(gtx, th)
				}),
			)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			if !app.alertShow {
				return layout.Dimensions{}
			}
			return app.layoutModalOverlay(gtx, th, app.alertTitle, app.alertMessage, func() {
				app.alertShow = false
			}, &app.alertClick)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			if !app.showSettingsOverlay {
				return layout.Dimensions{}
			}
			return app.layoutSettingsOverlay(gtx, th)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			if !app.showModuleOverlay {
				return layout.Dimensions{}
			}
			return app.layoutModuleOverlay(gtx, th)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			if !app.showBindOverlay {
				return layout.Dimensions{}
			}
			return app.layoutBindOverlay(gtx, th)
		}),
	)
}

func (app *App) syncGioTheme(th *material.Theme) {
	if app.lightTheme {
		th.Palette = material.Palette{
			Bg:         color.NRGBA{R: 0xF8, G: 0xFA, B: 0xFC, A: 0xFF}, // Slate-50
			Fg:         color.NRGBA{R: 0x0F, G: 0x17, B: 0x2A, A: 0xFF}, // Slate-900
			ContrastBg: color.NRGBA{R: 79, G: 70, B: 229, A: 0xFF},      // Indigo-600
			ContrastFg: color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF},
		}
	} else {
		th.Palette = material.Palette{
			Bg:         color.NRGBA{R: 15, G: 17, B: 26, A: 0xFF},     // Deep slate `#0f111a`
			Fg:         color.NRGBA{R: 241, G: 245, B: 249, A: 0xFF},  // Slate-100 `#f1f5f9`
			ContrastBg: color.NRGBA{R: 99, G: 102, B: 241, A: 0xFF},   // Indigo accent `#6366f1`
			ContrastFg: color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF},
		}
	}
}

func (app *App) layoutHeader(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						zLbl := material.H4(th, "z")
						zLbl.Color = th.Palette.ContrastBg
						zLbl.Font.Weight = font.Bold
						return zLbl.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						utilLbl := material.H4(th, "util")
						utilLbl.Font.Weight = font.Bold
						return utilLbl.Layout(gtx)
					}),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if app.settingsClick.Clicked(gtx) {
					app.showSettingsOverlay = true
				}
				return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layoutCustomIconButton(gtx, th, &app.settingsClick, app.widgetIconSettings(), "Settings", unit.Dp(24), th.Palette.Fg)
				})
			}),
		)
	})
}

func (app *App) layoutModulesList(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if len(app.cachedModules) == 0 {
		return material.Body1(th, "Initializing modules...").Layout(gtx)
	}

	list := app.cachedModules

	return layout.Inset{
		Left:  unit.Dp(16),
		Right: unit.Dp(16),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return app.listState.Layout(gtx, len(list), func(gtx layout.Context, index int) layout.Dimensions {
			conf := list[index].Key
			m := list[index].Value
			nameLower := list[index].NameLower

			dims := layout.Inset{
				Bottom: unit.Dp(12),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				cardBg := app.themeCardBg()

				return layout.Stack{}.Layout(gtx,
					layout.Expanded(func(gtx layout.Context) layout.Dimensions {
						d := image.Rectangle{Max: gtx.Constraints.Min}
						// Background card
						paint.FillShape(gtx.Ops, cardBg, clip.RRect{
							Rect: d,
							NE:   12, NW: 12, SE: 12, SW: 12,
						}.Op(gtx.Ops))

						// Card border stroke (1dp)
						borderCol := color.NRGBA{R: 0x2A, G: 0x2D, B: 0x3D, A: 0xFF}
						if app.lightTheme {
							borderCol = color.NRGBA{R: 0xE2, G: 0xE8, B: 0xF0, A: 0xFF}
						}
						strokeWidth := gtx.Dp(1)
						rrect := clip.RRect{Rect: d, NE: 12, NW: 12, SE: 12, SW: 12}
						cl := clip.Stroke{
							Path:  rrect.Path(gtx.Ops),
							Width: float32(strokeWidth),
						}.Op().Push(gtx.Ops)
						paint.ColorOp{Color: borderCol}.Add(gtx.Ops)
						paint.PaintOp{}.Add(gtx.Ops)
						cl.Pop()

						return layout.Dimensions{Size: gtx.Constraints.Min}
					}),
					layout.Stacked(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{
							Top:    unit.Dp(16),
							Bottom: unit.Dp(16),
							Left:   unit.Dp(16),
							Right:  unit.Dp(16),
						}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									var flexChildren []layout.FlexChild

									// Title label (lowercase)
									flexChildren = append(flexChildren, layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
										nameLbl := material.Body1(th, nameLower)
										nameLbl.Font.Weight = font.Bold
										return nameLbl.Layout(gtx)
									}))

									// Settings/Info button
									flexChildren = append(flexChildren, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										settingsClick, exists := app.moduleSettingsClicks[conf.Identifier()]
										if !exists {
											settingsClick = &widget.Clickable{}
											app.moduleSettingsClicks[conf.Identifier()] = settingsClick
										}
										if settingsClick.Clicked(gtx) {
											app.showModuleOverlay = true
											app.activeOverlayModule = m
											app.activeOverlayConfig = conf
										}

										icon := app.widgetIconSettings()
										if _, isToggleable := m.(modulesutil.ToggleableModule); !isToggleable {
											icon = app.widgetIconInfo()
										}

										return layout.UniformInset(unit.Dp(2)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											return layoutCustomIconButton(gtx, th, settingsClick, icon, "Module Settings", unit.Dp(20), th.Palette.Fg)
										})
									}))

									// Spacer and Switch (only for toggleable modules)
									if t, isToggle := m.(modulesutil.ToggleableModule); isToggle {
										flexChildren = append(flexChildren, layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout))
										flexChildren = append(flexChildren, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											boolState, exists := app.moduleToggleStates[conf.Identifier()]
											if !exists {
												boolState = &widget.Bool{Value: t.State()}
												app.moduleToggleStates[conf.Identifier()] = boolState
											}
											boolState.Value = t.State()

											dims := app.layoutCustomSwitch(gtx, th, boolState, conf.Identifier())
											if boolState.Value != t.State() {
												_ = t.UpdateState(boolState.Value, ActionCauseUserInput)
											}
											return dims
										}))
									}

									return layout.Flex{Alignment: layout.Middle}.Layout(gtx, flexChildren...)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									if f, isFloat := m.(modulesutil.ModuleWithValue[float64]); isFloat {
										val, _ := f.Value()

										minVal, maxVal := float32(1), float32(300)
										if fm, ok := f.(interface {
											Min() float64
											Max() float64
										}); ok {
											minVal = float32(fm.Min())
											maxVal = float32(fm.Max())
										}

										valState, exists := app.moduleSliderStates[conf.Identifier()]
										if !exists {
											valState = &widget.Float{Value: descaleFloat(float32(val), minVal, maxVal)}
											app.moduleSliderStates[conf.Identifier()] = valState
										}

										dims := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
											layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
											layout.Rigid(func(gtx layout.Context) layout.Dimensions {
												gtx.Constraints.Min.X = gtx.Constraints.Max.X
												return app.layoutCustomSlider(gtx, th, valState)
											}),
											layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
											layout.Rigid(func(gtx layout.Context) layout.Dimensions {
												lbl := material.Caption(th, fmt.Sprintf("value: %.0f", val))
												return lbl.Layout(gtx)
											}),
										)

										scaledVal := scaleFloat(valState.Value, minVal, maxVal)
										if math.Abs(float64(scaledVal-float32(val))) > 0.5 {
											_ = f.SetValue(float64(scaledVal), ActionCauseUserInput)
										}

										return dims
									}
									return layout.Dimensions{}
								}),
							)
						})
					}),
				)
			})
			return dims
		})
	})
}

func (app *App) layoutFooter(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Inset{
		Left:   unit.Dp(16),
		Right:  unit.Dp(16),
		Top:    unit.Dp(12),
		Bottom: unit.Dp(12),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
				linkBtn := material.Button(th, &app.joinControllinClick, "Join to controllin")
				linkBtn.Background = color.NRGBA{A: 0}
				linkBtn.Color = th.Palette.ContrastBg
				if app.joinControllinClick.Clicked(gtx) {
					_ = exec.Command("cmd", "/c", "start", "", "https://t.me/+rweTeGr1vOxjM2Qy").Start()
				}
				linkBtn.Inset = layout.UniformInset(unit.Dp(4))
				return linkBtn.Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				vLabel := material.Caption(th, "v1.0")
				return vLabel.Layout(gtx)
			}),
		)
	})
}

func (app *App) layoutModalOverlay(gtx layout.Context, th *material.Theme, title, content string, closeFunc func(), click *widget.Clickable) layout.Dimensions {
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

			gtx.Constraints.Min.X = gtx.Dp(300)
			gtx.Constraints.Max.X = gtx.Dp(300)

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
									titleLbl := material.Body1(th, title)
									titleLbl.Font.Weight = font.Bold
									return titleLbl.Layout(gtx)
								})
							}),
							layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									contentLbl := material.Body2(th, content)
									return contentLbl.Layout(gtx)
								})
							}),
							layout.Rigid(layout.Spacer{Height: unit.Dp(20)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if click.Clicked(gtx) {
									closeFunc()
								}
								return layoutFullWidthButton(gtx, th, click, "OK")
							}),
						)
					})
				}),
			)
		}),
	)
}

func (app *App) widgetIconSettings() *widget.Icon {
	return app.iconSettings
}

func (app *App) widgetIconInfo() *widget.Icon {
	return app.iconInfo
}

func (app *App) initCachedModules(modules *modulesMap) {
	app.iconSettings, _ = widget.NewIcon(icons.ActionSettings)
	app.iconInfo, _ = widget.NewIcon(icons.ActionInfo)
	app.moduleToggleProgress = make(map[string]float32)

	var list []moduleEntry
	for conf, m := range modules.AllFromFront() {
		entry := moduleEntry{
			Key:       conf,
			Value:     m,
			NameLower: strings.ToLower(m.Name()),
		}
		list = append(list, entry)

		// Pre-initialize states maps to avoid map queries and lazy allocation in layout loop
		id := conf.Identifier()
		app.moduleSettingsClicks[id] = &widget.Clickable{}
		if t, isToggle := m.(modulesutil.ToggleableModule); isToggle {
			app.moduleToggleStates[id] = &widget.Bool{Value: t.State()}
			if t.State() {
				app.moduleToggleProgress[id] = 1.0
			} else {
				app.moduleToggleProgress[id] = 0.0
			}
		}
		if f, isFloat := m.(modulesutil.ModuleWithValue[float64]); isFloat {
			val, _ := f.Value()
			minVal, maxVal := float32(1), float32(300)
			if fm, ok := f.(interface {
				Min() float64
				Max() float64
			}); ok {
				minVal = float32(fm.Min())
				maxVal = float32(fm.Max())
			}
			app.moduleSliderStates[id] = &widget.Float{Value: descaleFloat(float32(val), minVal, maxVal)}
		}
	}
	app.cachedModules = list
}

func layoutFullWidthButton(gtx layout.Context, th *material.Theme, click *widget.Clickable, text string) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	btn := material.Button(th, click, text)
	btn.CornerRadius = unit.Dp(6)
	return btn.Layout(gtx)
}

func (app *App) layoutCustomSwitch(gtx layout.Context, th *material.Theme, state *widget.Bool, moduleID string) layout.Dimensions {
	state.Update(gtx)

	trackWidth := gtx.Dp(36)
	trackHeight := gtx.Dp(20)
	thumbSize := gtx.Dp(16)
	trackOff := (trackHeight - gtx.Dp(12)) / 2

	// Smoothly animate the knob progress
	currentProgress := app.moduleToggleProgress[moduleID]
	target := float32(0.0)
	if state.Value {
		target = 1.0
	}

	if currentProgress != target {
		speed := float32(0.15)
		if currentProgress < target {
			currentProgress += speed
			if currentProgress > target {
				currentProgress = target
			}
		} else {
			currentProgress -= speed
			if currentProgress < target {
				currentProgress = target
			}
		}
		app.moduleToggleProgress[moduleID] = currentProgress
		gtx.Execute(op.InvalidateCmd{})
	}

	var trackColor color.NRGBA
	var thumbColor color.NRGBA

	// OFF colors
	var offTrack, offThumb color.NRGBA
	if app.lightTheme {
		offTrack = color.NRGBA{R: 0xE2, G: 0xE8, B: 0xF0, A: 0xFF}
		offThumb = color.NRGBA{R: 0x94, G: 0xA3, B: 0xB8, A: 0xFF}
	} else {
		offTrack = color.NRGBA{R: 0x1E, G: 0x29, B: 0x3B, A: 0xFF}
		offThumb = color.NRGBA{R: 0x47, G: 0x55, B: 0x69, A: 0xFF}
	}

	// ON colors
	onTrack := th.Palette.ContrastBg
	onThumb := color.NRGBA{R: 255, G: 255, B: 255, A: 255}

	// Linear interpolation helper
	lerp := func(a, b uint8, t float32) uint8 {
		return uint8(float32(a) + t*(float32(b)-float32(a)))
	}

	trackColor = color.NRGBA{
		R: lerp(offTrack.R, onTrack.R, currentProgress),
		G: lerp(offTrack.G, onTrack.G, currentProgress),
		B: lerp(offTrack.B, onTrack.B, currentProgress),
		A: lerp(offTrack.A, onTrack.A, currentProgress),
	}

	thumbColor = color.NRGBA{
		R: lerp(offThumb.R, onThumb.R, currentProgress),
		G: lerp(offThumb.G, onThumb.G, currentProgress),
		B: lerp(offThumb.B, onThumb.B, currentProgress),
		A: lerp(offThumb.A, onThumb.A, currentProgress),
	}

	// 1. Draw Track
	trackRect := image.Rectangle{
		Min: image.Pt(0, trackOff),
		Max: image.Pt(trackWidth, trackOff+gtx.Dp(12)),
	}
	cl := clip.UniformRRect(trackRect, gtx.Dp(6)).Push(gtx.Ops)
	paint.ColorOp{Color: trackColor}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	cl.Pop()

	// 2. Draw Thumb
	minX := float32(gtx.Dp(2))
	maxX := float32(trackWidth - thumbSize - gtx.Dp(2))
	thumbX := int(minX + currentProgress*(maxX-minX))
	thumbY := (trackHeight - thumbSize) / 2

	thumbRect := image.Rectangle{
		Min: image.Pt(thumbX, thumbY),
		Max: image.Pt(thumbX+thumbSize, thumbY+thumbSize),
	}
	cl2 := clip.Ellipse(thumbRect).Push(gtx.Ops)
	paint.ColorOp{Color: thumbColor}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	cl2.Pop()

	// 3. Capture clicks on the switch area without showing hover highlight
	sz := image.Pt(trackWidth, trackHeight)
	defer clip.Rect(image.Rectangle{Max: sz}).Push(gtx.Ops).Pop()
	state.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: sz}
	})

	return layout.Dimensions{Size: sz}
}

func layoutCustomIconButton(gtx layout.Context, th *material.Theme, click *widget.Clickable, icon *widget.Icon, description string, size unit.Dp, color color.NRGBA) layout.Dimensions {
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.Button.Add(gtx.Ops)
		if description != "" {
			semantic.DescriptionOp(description).Add(gtx.Ops)
		}

		iconSize := gtx.Dp(size)
		gtx.Constraints.Min = image.Point{X: iconSize, Y: iconSize}
		gtx.Constraints.Max = gtx.Constraints.Min

		if icon != nil {
			icon.Layout(gtx, color)
		}

		return layout.Dimensions{
			Size: image.Point{X: iconSize, Y: iconSize},
		}
	})
}

func (app *App) layoutCustomSlider(gtx layout.Context, th *material.Theme, float *widget.Float) layout.Dimensions {
	const thumbRadius unit.Dp = 8
	tr := gtx.Dp(thumbRadius)
	trackWidth := gtx.Dp(4)

	axis := layout.Horizontal
	minLength := tr + 3*tr + tr
	touchSizePx := min(gtx.Dp(th.FingerSize), axis.Convert(gtx.Constraints.Max).Y)
	sizeMain := max(axis.Convert(gtx.Constraints.Min).X, minLength)
	sizeCross := max(2*tr, touchSizePx)
	size := axis.Convert(image.Pt(sizeMain, sizeCross))

	o := axis.Convert(image.Pt(tr, 0))
	trans := op.Offset(o).Push(gtx.Ops)
	gtx.Constraints.Min = axis.Convert(image.Pt(sizeMain-2*tr, sizeCross))
	dims := float.Layout(gtx, axis, thumbRadius)
	gtx.Constraints.Min = gtx.Constraints.Min.Add(axis.Convert(image.Pt(0, sizeCross)))
	thumbPos := tr + int(float.Value*float32(axis.Convert(dims.Size).X))
	trans.Pop()

	color := th.Palette.ContrastBg
	if !gtx.Enabled() {
		color = disabledColor(color)
	}

	rect := func(minx, miny, maxx, maxy int) image.Rectangle {
		r := image.Rect(minx, miny, maxx, maxy)
		r.Min = axis.Convert(r.Min)
		r.Max = axis.Convert(r.Max)
		return r
	}

	// 1. Draw track before thumb (active track with rounded corners)
	track := rect(
		tr, sizeCross/2-trackWidth/2,
		thumbPos, sizeCross/2+trackWidth/2,
	)
	cl := clip.UniformRRect(track, trackWidth/2).Push(gtx.Ops)
	paint.ColorOp{Color: color}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	cl.Pop()

	// 2. Draw track after thumb (inactive track with rounded corners)
	track = rect(
		thumbPos, sizeCross/2-trackWidth/2,
		sizeMain-tr, sizeCross/2+trackWidth/2,
	)
	cl2 := clip.UniformRRect(track, trackWidth/2).Push(gtx.Ops)
	paint.ColorOp{Color: mulAlpha(color, 64)}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	cl2.Pop()

	// 3. Draw thumb
	pt := image.Pt(thumbPos, sizeCross/2)
	thumb := rect(
		pt.X-tr, pt.Y-tr,
		pt.X+tr, pt.Y+tr,
	)
	paint.FillShape(gtx.Ops, color, clip.Ellipse(thumb).Op(gtx.Ops))

	return layout.Dimensions{Size: size}
}

func mulAlpha(c color.NRGBA, alpha uint8) color.NRGBA {
	return color.NRGBA{
		R: c.R,
		G: c.G,
		B: c.B,
		A: uint8(uint32(c.A) * uint32(alpha) / 255),
	}
}

func disabledColor(c color.NRGBA) color.NRGBA {
	return color.NRGBA{
		R: c.R,
		G: c.G,
		B: c.B,
		A: uint8(uint32(c.A) * 128 / 255),
	}
}
