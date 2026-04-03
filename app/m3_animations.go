package app

import (
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

var AnimationSpeed float64 = 1.0

type M3DurationConstant int

const (
	M3DurationShort2  M3DurationConstant = 100
	M3DurationMedium1 M3DurationConstant = 250
	M3DurationMedium2 M3DurationConstant = 300
)

func M3Duration(d M3DurationConstant) time.Duration {
	return time.Duration(float64(d)/AnimationSpeed) * time.Millisecond
}

func M3Animate(duration M3DurationConstant, easing func(float64) float64, callback func(float64)) {
	go func() {
		start := time.Now()
		d := M3Duration(duration)
		ticker := time.NewTicker(16 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			elapsed := time.Since(start)
			if elapsed >= d {
				callback(1.0)
				break
			}
			callback(easing(float64(elapsed) / float64(d)))
		}
	}()
}

func EmphasizedEasing(t float64) float64 {
	return cubicBezier(t, 0.2, 0.0, 0.0, 1.0)
}

func EmphasizedDecelerateEasing(t float64) float64 {
	return cubicBezier(t, 0.05, 0.7, 0.1, 1.0)
}

func cubicBezier(t, p1x, p1y, p2x, p2y float64) float64 {
	x := t
	for i := 0; i < 8; i++ {
		fx := (3*(1-x)*(1-x)*x*p1x + 3*(1-x)*x*x*p2x + x*x*x) - t
		if fx < 0.001 && fx > -0.001 {
			break
		}
		fpx := 3*(1-x)*(1-x)*p1x + 6*(1-x)*x*(p2x-p1x) + 3*x*x*(1-p2x)
		if fpx == 0 {
			break
		}
		x -= fx / fpx
	}
	return 3*(1-x)*(1-x)*x*p1y + 3*(1-x)*x*x*p2y + x*x*x
}

type M3AnimatedButton struct {
	widget.Button
	mu       sync.Mutex
	pressing bool
}

func NewM3AnimatedButton(text string, onTapped func()) *M3AnimatedButton {
	btn := &M3AnimatedButton{
		Button: widget.Button{Text: text},
	}
	btn.ExtendBaseWidget(btn)
	// Вызываем onTapped сразу — кнопка работает корректно без анимации
	btn.OnTapped = func() {
		if onTapped != nil {
			onTapped()
		}
	}
	return btn
}

func (b *M3AnimatedButton) MinSize() fyne.Size {
	return b.Button.MinSize()
}
