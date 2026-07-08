package app

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"os/exec"
	"strings"
	"syscall"

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

	if !app.lightTheme {
		bounds := image.Rect(0, 0, gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
		paint.LinearGradientOp{
			Stop1:  layout.FPt(bounds.Min),
			Stop2:  layout.FPt(bounds.Max),
			Color1: color.NRGBA{R: 0x0A, G: 0x09, B: 0x1A, A: 0xFF}, // Darker cosmic
			Color2: color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xFF},
		}.Add(gtx.Ops)
		cl := clip.Rect(bounds).Push(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		cl.Pop()
	} else {
		paint.Fill(gtx.Ops, th.Palette.Bg)
	}

	return layout.Stack{}.Layout(gtx,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return app.layoutHeader(gtx, th)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return app.layoutToolbar(gtx, th)
				}),
				layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
					var dims layout.Dimensions
					switch app.activeTab {
					case 1:
						dims = app.layoutConfigsTab(gtx, th)
					case 4:
						dims = app.layoutSettingsTab(gtx, th)
					default:
						dims = app.layoutModulesList(gtx, th)
					}
					return dims
				}),
			)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			if app.alertShow || app.showModuleOverlay || app.showBindOverlay {
				return app.blockerClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{Size: gtx.Constraints.Max}
				})
			}
			return layout.Dimensions{}
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
			Bg:         color.NRGBA{R: 0xF8, G: 0xFA, B: 0xFC, A: 0xFF},
			Fg:         color.NRGBA{R: 0x0F, G: 0x17, B: 0x2A, A: 0xFF},
			ContrastBg: color.NRGBA{R: 0x4F, G: 0x46, B: 0xE5, A: 0xFF},
			ContrastFg: color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF},
		}
	} else {
		th.Palette = material.Palette{
			Bg:         color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xFF},
			Fg:         color.NRGBA{R: 0xE2, G: 0xE8, B: 0xF0, A: 0xFF}, // Softer text
			ContrastBg: color.NRGBA{R: 0x4F, G: 0x46, B: 0xE5, A: 0xFF}, // Darker Indigo accent
			ContrastFg: color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF},
		}
	}
}

func (app *App) layoutHeader(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								zLbl := material.H4(th, "z")
								zLbl.Color = th.Palette.ContrastBg
								zLbl.Font.Typeface = "monospace"
								zLbl.Font.Weight = font.Bold
								return zLbl.Layout(gtx)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								utilLbl := material.H4(th, "util-gio")
								utilLbl.Font.Typeface = "monospace"
								utilLbl.Font.Weight = font.Bold
								return utilLbl.Layout(gtx)
							}),
						)
					}),
					// Play button on the far right of the header
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if app.playClick.Clicked(gtx) {
							go func() {
								cmd := exec.Command("cmd", "/c", "start", "minecraft:")
								cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
								_ = cmd.Run()
							}()
						}
						return layoutCustomIconButton(gtx, th, &app.playClick, app.widgetIconPlay(), "Play Minecraft", unit.Dp(24), th.Palette.ContrastBg)
					}),
				)
			})
		}),
		// Bottom border line (accent color)
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			d := image.Point{X: gtx.Constraints.Max.X, Y: gtx.Dp(2)}
			defer clip.Rect{Max: d}.Push(gtx.Ops).Pop()
			paint.Fill(gtx.Ops, th.Palette.ContrastBg)
			return layout.Dimensions{Size: d}
		}),
	)
}

func (app *App) layoutModulesList(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if len(app.cachedModules) == 0 {
		return material.Body1(th, "Initializing modules...").Layout(gtx)
	}

	list := app.cachedModules

	return layout.Inset{
		Top:   unit.Dp(12),
		Left:  unit.Dp(12),
		Right: unit.Dp(12),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return app.listState.Layout(gtx, len(list), func(gtx layout.Context, index int) layout.Dimensions {
			conf := list[index].Key
			m := list[index].Value
			nameLower := list[index].NameLower

			cardClick := app.moduleCardClicks[conf.Identifier()]
			if t, isToggle := m.(modulesutil.ToggleableModule); isToggle {
				if cardClick.Clicked(gtx) {
					newState := !t.State()
					_ = t.UpdateState(newState, ActionCauseUserInput)
				}
			}

			dims := layout.Inset{
				Bottom: unit.Dp(8),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				cardBg := app.themeCardBg()

				isEnabled := false
				if t, ok := m.(modulesutil.ToggleableModule); ok {
					isEnabled = t.State()
				} else if _, ok := m.(modulesutil.ModuleWithValue[float64]); ok {
					isEnabled = true
				}

				return layout.Stack{}.Layout(gtx,
					layout.Expanded(func(gtx layout.Context) layout.Dimensions {
						return cardClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							d := image.Rectangle{Max: gtx.Constraints.Min}
							r := 0
							
							var finalBg color.NRGBA
							if isEnabled {
								if app.lightTheme {
									finalBg = color.NRGBA{R: 0xEC, G: 0xEC, B: 0xEC, A: 0xFF}
								} else {
									finalBg = color.NRGBA{R: 0x1A, G: 0x18, B: 0x30, A: 0xCC} // Darker transparent cosmic for active
								}
							} else {
								if app.lightTheme {
									finalBg = cardBg
								} else {
									finalBg = color.NRGBA{R: 0x08, G: 0x08, B: 0x0E, A: 0x99} // Very dark inactive card
								}
							}

							// Background card
							paint.FillShape(gtx.Ops, finalBg, clip.RRect{Rect: d, NE: r, NW: r, SE: r, SW: r}.Op(gtx.Ops))

							// Card border stroke (2dp)
							var borderCol color.NRGBA
							if isEnabled {
								borderCol = th.Palette.ContrastBg
							} else if app.lightTheme {
								borderCol = color.NRGBA{R: 0xDD, G: 0xDD, B: 0xDD, A: 0xFF}
							} else {
								borderCol = color.NRGBA{R: 0x1E, G: 0x1E, B: 0x2A, A: 0xFF} // Subtle dark border
							}

							strokeWidth := gtx.Dp(2)
							cl := clip.Stroke{
								Path:  clip.RRect{Rect: d, NE: r, NW: r, SE: r, SW: r}.Path(gtx.Ops),
								Width: float32(strokeWidth),
							}.Op().Push(gtx.Ops)
							paint.ColorOp{Color: borderCol}.Add(gtx.Ops)
							paint.PaintOp{}.Add(gtx.Ops)
							cl.Pop()

							return layout.Dimensions{Size: gtx.Constraints.Min}
						})
					}),
					layout.Stacked(func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									var flexChildren []layout.FlexChild

									// Title label (uppercase monospace)
									flexChildren = append(flexChildren, layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
										nameLbl := material.Body1(th, strings.ToUpper(nameLower))
										nameLbl.Font.Typeface = "monospace"
										nameLbl.Font.Weight = font.Bold
										return nameLbl.Layout(gtx)
									}))

									// Status indicator tag (ON/OFF pill)
									if _, isToggle := m.(modulesutil.ToggleableModule); isToggle {
										flexChildren = append(flexChildren, layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout))
										flexChildren = append(flexChildren, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return layoutStatusPill(gtx, th, isEnabled)
										}))
									}

									// Settings/Info button
									flexChildren = append(flexChildren, layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout))
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

										return layoutCustomIconButton(gtx, th, settingsClick, icon, "Module Settings", unit.Dp(20), th.Palette.Fg)
									}))

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
												format := "value: %.0f"
												if maxVal < 10 {
													format = "value: %.2f"
												}
												lbl := material.Caption(th, fmt.Sprintf(format, val))
												return lbl.Layout(gtx)
											}),
										)

										scaledVal := scaleFloat(valState.Value, minVal, maxVal)
										epsilon := float32(0.5)
										if maxVal < 10 {
											epsilon = 0.005
										}
										if math.Abs(float64(scaledVal-float32(val))) > float64(epsilon) {
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
									titleLbl := material.Body1(th, strings.ToUpper(title))
									titleLbl.Font.Typeface = "monospace"
									titleLbl.Font.Weight = font.Bold
									return titleLbl.Layout(gtx)
								})
							}),
							layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									contentLbl := material.Body2(th, content)
									contentLbl.Font.Typeface = "monospace"
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

func (app *App) widgetIconPlay() *widget.Icon {
	return app.iconPlay
}

func (app *App) initCachedModules(modules *modulesMap) {
	app.iconSettings, _ = widget.NewIcon(icons.ActionSettings)
	app.iconInfo, _ = widget.NewIcon(icons.ActionInfo)
	app.iconPlay, _ = widget.NewIcon(icons.AVPlayArrow)

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
		app.moduleCardClicks[id] = &widget.Clickable{}
		if t, isToggle := m.(modulesutil.ToggleableModule); isToggle {
			app.moduleToggleStates[id] = &widget.Bool{Value: t.State()}
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
	btn := material.Button(th, click, strings.ToUpper(text))
	btn.CornerRadius = unit.Dp(0)
	btn.Font.Typeface = "monospace"
	btn.Font.Weight = font.Bold
	return btn.Layout(gtx)
}


func layoutCustomIconButton(gtx layout.Context, th *material.Theme, click *widget.Clickable, icon *widget.Icon, description string, size unit.Dp, iconColor color.NRGBA) layout.Dimensions {
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.Button.Add(gtx.Ops)
		if description != "" {
			semantic.DescriptionOp(description).Add(gtx.Ops)
		}

		iconSize := gtx.Dp(size)

		var bgCol color.NRGBA
		if click.Pressed() {
			bgCol = mulAlpha(th.Palette.ContrastBg, 80)
		} else if click.Hovered() {
			bgCol = color.NRGBA{R: 0x22, G: 0x22, B: 0x22, A: 0xFF}
		}

		return layout.Stack{Alignment: layout.Center}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				if bgCol.A > 0 {
					d := image.Rectangle{Max: gtx.Constraints.Min}
					r := 0
					paint.FillShape(gtx.Ops, bgCol, clip.RRect{Rect: d, NE: r, NW: r, SE: r, SW: r}.Op(gtx.Ops))
				}
				return layout.Dimensions{Size: gtx.Constraints.Min}
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(6)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					if icon != nil {
						gtx.Constraints.Min = image.Point{X: iconSize, Y: iconSize}
						gtx.Constraints.Max = gtx.Constraints.Min
						return icon.Layout(gtx, iconColor)
					}
					return layout.Dimensions{
						Size: image.Point{X: iconSize, Y: iconSize},
					}
				})
			}),
		)
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
	trackActive := rect(
		tr, sizeCross/2-trackWidth/2,
		thumbPos, sizeCross/2+trackWidth/2,
	)
	rTrack := 0
	paint.FillShape(gtx.Ops, color, clip.RRect{Rect: trackActive, NE: rTrack, NW: rTrack, SE: rTrack, SW: rTrack}.Op(gtx.Ops))

	// 2. Draw track after thumb (inactive track with rounded corners)
	trackInactive := rect(
		thumbPos, sizeCross/2-trackWidth/2,
		sizeMain-tr, sizeCross/2+trackWidth/2,
	)
	paint.FillShape(gtx.Ops, mulAlpha(color, 64), clip.RRect{Rect: trackInactive, NE: rTrack, NW: rTrack, SE: rTrack, SW: rTrack}.Op(gtx.Ops))

	// 3. Draw thumb (rounded circle handle)
	pt := image.Pt(thumbPos, sizeCross/2)
	thumb := rect(
		pt.X-tr, pt.Y-tr,
		pt.X+tr, pt.Y+tr,
	)
	paint.FillShape(gtx.Ops, color, clip.RRect{Rect: thumb}.Op(gtx.Ops))

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

func (app *App) tabTextColor(i float32, th *material.Theme) color.NRGBA {
	dist := float32(math.Abs(float64(app.tabAnimProgress - i)))
	if dist < 0.5 {
		return th.Palette.ContrastFg
	}
	if app.lightTheme {
		return color.NRGBA{R: 0x66, G: 0x66, B: 0x66, A: 0xFF}
	}
	return color.NRGBA{R: 0x90, G: 0x90, B: 0x90, A: 0xFF}
}

func (app *App) layoutToolbar(gtx layout.Context, th *material.Theme) layout.Dimensions {
	tabChanged := false
	if app.tabClickModules.Clicked(gtx) {
		if app.activeTab != 0 {
			app.activeTab = 0
			tabChanged = true
		}
	}
	if app.tabClickConfig.Clicked(gtx) {
		if app.activeTab != 1 {
			app.activeTab = 1
			tabChanged = true
		}
	}
	if app.tabClickSettings.Clicked(gtx) {
		if app.activeTab != 4 {
			app.activeTab = 4
			tabChanged = true
		}
	}

	if tabChanged {
		gtx.Execute(op.InvalidateCmd{})
	}

	// Calculate target index
	var targetIdx float32
	switch app.activeTab {
	case 1:
		targetIdx = 1.0
	case 4:
		targetIdx = 2.0
	default:
		targetIdx = 0.0
	}

	if app.tabAnimProgress != targetIdx {
		speed := float32(0.2) // fast, responsive slide
		diff := targetIdx - app.tabAnimProgress
		if math.Abs(float64(diff)) < float64(speed) {
			app.tabAnimProgress = targetIdx
		} else {
			if diff > 0 {
				app.tabAnimProgress += speed
			} else {
				app.tabAnimProgress -= speed
			}
		}
		gtx.Execute(op.InvalidateCmd{})
	}

	var children []layout.FlexChild

	// 1. MODULES Tab
	children = append(children, layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
		return app.tabClickModules.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			textColor := app.tabTextColor(0, th)
			return layoutTabBlockWithColor(gtx, th, "MODULES", textColor)
		})
	}))

	// 2. CONFIG Tab
	children = append(children, layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
		return app.tabClickConfig.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			textColor := app.tabTextColor(1, th)
			return layoutTabBlockWithColor(gtx, th, "CONFIG", textColor)
		})
	}))

	// 3. SETTINGS Tab
	children = append(children, layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
		return app.tabClickSettings.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			textColor := app.tabTextColor(2, th)
			return layoutTabBlockWithColor(gtx, th, "SETTINGS", textColor)
		})
	}))

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Stack{}.Layout(gtx,
				// Background layer: Animated sliding capsule
				layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					width := gtx.Constraints.Min.X
					if width == 0 {
						width = gtx.Constraints.Max.X
					}
					totalWidth := float32(width)
					cellW := totalWidth / 3.0
					left := app.tabAnimProgress * cellW

					padding := gtx.Dp(4)
					r := 0

					capsuleRect := image.Rectangle{
						Min: image.Pt(int(left)+padding, padding),
						Max: image.Pt(int(left+cellW)-padding, gtx.Constraints.Min.Y-padding),
					}

					if capsuleRect.Max.X > capsuleRect.Min.X && capsuleRect.Max.Y > capsuleRect.Min.Y {
						paint.FillShape(gtx.Ops, th.Palette.ContrastBg, clip.RRect{
							Rect: capsuleRect,
							NE: r, NW: r, SE: r, SW: r,
						}.Op(gtx.Ops))
					}
					return layout.Dimensions{Size: gtx.Constraints.Min}
				}),
				// Foreground layer: Tab labels
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			d := image.Point{X: gtx.Constraints.Max.X, Y: gtx.Dp(2)}
			defer clip.Rect{Max: d}.Push(gtx.Ops).Pop()
			paint.Fill(gtx.Ops, th.Palette.ContrastBg)
			return layout.Dimensions{Size: d}
		}),
	)
}

func layoutTabBlockWithColor(gtx layout.Context, th *material.Theme, label string, textColor color.NRGBA) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body2(th, label)
			lbl.Font.Typeface = "monospace"
			lbl.Font.Weight = font.Bold
			lbl.Color = textColor
			return lbl.Layout(gtx)
		})
	})
}

func layoutStatusPill(gtx layout.Context, th *material.Theme, isEnabled bool) layout.Dimensions {
	text := "OFF"
	bgCol := color.NRGBA{R: 0x1E, G: 0x29, B: 0x3B, A: 0xFF}
	txtCol := color.NRGBA{R: 0x94, G: 0xA3, B: 0xB8, A: 0xFF}
	
	if isEnabled {
		text = "ON"
		bgCol = th.Palette.ContrastBg
		txtCol = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xFF}
	} else if th.Palette.Bg.R > 0xEE {
		bgCol = color.NRGBA{R: 0xE2, G: 0xE8, B: 0xF0, A: 0xFF}
		txtCol = color.NRGBA{R: 0x64, G: 0x74, B: 0x8B, A: 0xFF}
	}

	return layout.Stack{Alignment: layout.Center}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			d := image.Rectangle{Max: gtx.Constraints.Min}
			r := 0
			paint.FillShape(gtx.Ops, bgCol, clip.RRect{Rect: d, NE: r, NW: r, SE: r, SW: r}.Op(gtx.Ops))
			return layout.Dimensions{Size: gtx.Constraints.Min}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{
				Top:    unit.Dp(2),
				Bottom: unit.Dp(2),
				Left:   unit.Dp(8),
				Right:  unit.Dp(8),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(th, text)
				lbl.Font.Typeface = "monospace"
				lbl.Font.Weight = font.Bold
				lbl.Color = txtCol
				return lbl.Layout(gtx)
			})
		}),
	)
}



func (app *App) layoutConfigsTab(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body1(th, "FILES CONFIGURATION")
				lbl.Font.Typeface = "monospace"
				lbl.Font.Weight = font.Bold
				lbl.Color = th.Palette.Fg
				return lbl.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if app.importConfigClick.Clicked(gtx) {
					app.importConfig()
				}
				return layoutFullWidthButton(gtx, th, &app.importConfigClick, "Import config")
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if app.exportConfigClick.Clicked(gtx) {
					app.exportConfig()
				}
				return layoutFullWidthButton(gtx, th, &app.exportConfigClick, "Export config")
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if app.resetConfigClick.Clicked(gtx) {
					app.resetConfig()
				}
				return layoutFullWidthButton(gtx, th, &app.resetConfigClick, "Reset config")
			}),
		)
	})
}

func (app *App) layoutSettingsTab(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body1(th, "SETTINGS")
				lbl.Font.Typeface = "monospace"
				lbl.Font.Weight = font.Bold
				lbl.Color = th.Palette.Fg
				return lbl.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				app.userConf.Lock()
				currentVal := app.userConf.V.ShowErrors
				app.userConf.Unlock()

				if app.showErrorsClick.Clicked(gtx) {
					currentVal = !currentVal
					app.showErrors(currentVal, ActionCauseUserInput)
					app.saveUserConfig()
				}

				labelText := "SHOW ERRORS: OFF"
				var bgCol color.NRGBA
				var txtCol color.NRGBA
				if currentVal {
					labelText = "SHOW ERRORS: ON"
					bgCol = th.Palette.ContrastBg
					txtCol = color.NRGBA{R: 0, G: 0, B: 0, A: 255}
				} else {
					if app.lightTheme {
						bgCol = color.NRGBA{R: 0xE2, G: 0xE8, B: 0xF0, A: 0xFF}
						txtCol = color.NRGBA{R: 0x64, G: 0x74, B: 0x8B, A: 0xFF}
					} else {
						bgCol = color.NRGBA{R: 0x1E, G: 0x29, B: 0x3B, A: 0xFF}
						txtCol = color.NRGBA{R: 0x94, G: 0xA3, B: 0xB8, A: 0xFF}
					}
				}

				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				btn := material.Button(th, &app.showErrorsClick, labelText)
				btn.CornerRadius = unit.Dp(0)
				btn.Background = bgCol
				btn.Color = txtCol
				btn.Font.Typeface = "monospace"
				btn.Font.Weight = font.Bold
				return btn.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if app.toggleThemeClick.Clicked(gtx) {
					app.lightTheme = !app.lightTheme
					app.userConf.Lock()
					app.userConf.V.LightTheme = app.lightTheme
					app.userConf.Unlock()
					app.saveUserConfig()
					app.applyWindowsDarkMode()
				}
				label := "Switch to Dark Theme"
				if !app.lightTheme {
					label = "Switch to Light Theme"
				}
				return layoutFullWidthButton(gtx, th, &app.toggleThemeClick, label)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if app.aboutClick.Clicked(gtx) {
					app.showInfo("ABOUT", aboutMessage)
				}
				return layoutFullWidthButton(gtx, th, &app.aboutClick, "About app")
			}),
		)
	})
}


