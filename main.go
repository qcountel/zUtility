package main

import (
	"context"
	"fmt"
	"image/color"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/something-that-is-cool/zutil/app"
	"github.com/something-that-is-cool/zutil/internal/pkg/win"
	"github.com/something-that-is-cool/zutil/pkg/embeddable"
)

var config = app.Config{
	Logger:  slog.Default(),
	Process: "Minecraft.Windows.exe",
}

func main() {

	alreadyRunning, cleanup := win.EnsureSingleInstance(app.Name)
	if alreadyRunning {
		os.Exit(0)
	}
	defer cleanup()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	a, err := config.New(ctx)
	if err != nil {
		doPanic(fmt.Errorf("error creating app: %w", err))
	}
	defer a.Close(true)
	if err = a.Run(); err != nil {
		doPanic(fmt.Errorf("error running app: %w", err))
	}
}

var (
	errBackground = color.NRGBA{R: 0x08, G: 0x08, B: 0x08, A: 0xFF}
	errSurface    = color.NRGBA{R: 0x0D, G: 0x0D, B: 0x0D, A: 0xFF}
	errPrimary    = color.NRGBA{R: 0xCC, G: 0x33, B: 0x33, A: 0xFF}
	errOnSurface  = color.NRGBA{R: 0xE0, G: 0xE0, B: 0xE0, A: 0xFF}
	errMuted      = color.NRGBA{R: 0x44, G: 0x44, B: 0x44, A: 0xFF}
	errAccent     = color.NRGBA{R: 0xCC, G: 0x33, B: 0x33, A: 0xFF}
	errDivider    = color.NRGBA{R: 0xCC, G: 0x33, B: 0x33, A: 0x40}
)

func etxt(s string, c color.Color, size float32) *canvas.Text {
	t := canvas.NewText(s, c)
	t.TextSize = size
	return t
}

func doPanic(v any) {
	raw := fmt.Sprint(v)
	headline := "Startup Error"
	hint := raw

	if strings.Contains(raw, "no process by name") || strings.Contains(raw, "open process") {
		headline = "Minecraft is not running"
		hint = "Please launch Minecraft Pocket Edition v1.1.5\nand start zUtility again."
	}

	showStyledError(headline, hint)
	os.Exit(1)
}

func showStyledError(headline, hint string) {
	a := fyneapp.New()

	icon, _ := embeddable.LoadIcon()
	if icon != nil {
		a.SetIcon(icon)
	}

	w := a.NewWindow("zUtility — Error")
	w.SetFixedSize(true)
	w.Resize(fyne.NewSize(440, 230))
	w.CenterOnScreen()

	if icon != nil {
		w.SetIcon(icon)
	}

	bg := canvas.NewRectangle(errBackground)

	cardBg := canvas.NewRectangle(errSurface)
	cardBg.CornerRadius = 3

	dot := canvas.NewRectangle(errAccent)
	dot.CornerRadius = 3
	dot.SetMinSize(fyne.NewSize(10, 10))

	titleTxt := etxt("Error", errPrimary, 22)
	titleTxt.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
	titleRow := container.NewHBox(
		container.New(layout.NewCustomPaddedLayout(6, 0, 0, 8), dot),
		titleTxt,
	)

	div := canvas.NewRectangle(errDivider)
	div.SetMinSize(fyne.NewSize(0, 1))

	headlineTxt := etxt(headline, errOnSurface, 15)
	headlineTxt.TextStyle = fyne.TextStyle{Bold: true}

	hintLabel := widget.NewLabel(hint)
	hintLabel.Wrapping = fyne.TextWrapWord

	cardContent := container.NewVBox(
		titleRow,
		container.New(layout.NewCustomPaddedLayout(6, 10, 0, 0), div),
		headlineTxt,
		container.New(layout.NewCustomPaddedLayout(4, 0, 0, 0), hintLabel),
	)

	paddedCard := container.New(layout.NewCustomPaddedLayout(20, 20, 20, 20), cardContent)
	card := container.NewStack(cardBg, paddedCard)
	outer := container.New(layout.NewCustomPaddedLayout(24, 24, 24, 24), card)

	w.SetContent(container.NewStack(bg, outer))
	w.ShowAndRun()
}
