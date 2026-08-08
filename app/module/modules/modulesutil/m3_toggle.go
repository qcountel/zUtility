package modulesutil

import (
	"image/color"
	"math"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

var (
	m3ToggleBg   = color.NRGBA{R: 0x1A, G: 0x1A, B: 0x1A, A: 0xFF}
	m3ToggleBgOn = color.NRGBA{R: 0xAA, G: 0x18, B: 0x18, A: 0xFF}
)

const (
	toggleW     = float32(46)
	toggleH     = float32(26)
	toggleThumb = float32(18)
	togglePad   = float32(4)
)

type M3Toggle struct {
	widget.BaseWidget

	checked       bool
	OnChange      func(bool)
	OnStateChange func(bool) // called on every state change, even from SetChecked

	mu        sync.Mutex
	animating bool

	bg    *canvas.Rectangle
	thumb *canvas.Rectangle
}

func NewM3Toggle(checked bool) *M3Toggle {
	t := &M3Toggle{checked: checked}

	t.bg = canvas.NewRectangle(m3ToggleBg)
	t.bg.CornerRadius = 3
	if checked {
		t.bg.FillColor = m3ToggleBgOn
	}
	t.bg.Resize(fyne.NewSize(toggleW, toggleH))

	t.thumb = canvas.NewRectangle(color.NRGBA{R: 0xE0, G: 0xE0, B: 0xE0, A: 0xFF})
	t.thumb.CornerRadius = 3
	t.thumb.Resize(fyne.NewSize(toggleThumb, toggleThumb))

	if checked {
		t.thumb.Move(fyne.NewPos(toggleW-toggleThumb-togglePad, togglePad))
	} else {
		t.thumb.Move(fyne.NewPos(togglePad, togglePad))
	}

	t.ExtendBaseWidget(t)
	return t
}

func (t *M3Toggle) CreateRenderer() fyne.WidgetRenderer {
	return &m3ToggleRenderer{
		toggle:  t,
		objects: []fyne.CanvasObject{t.bg, t.thumb},
	}
}

type m3ToggleRenderer struct {
	toggle  *M3Toggle
	objects []fyne.CanvasObject
}

func (r *m3ToggleRenderer) MinSize() fyne.Size {
	return fyne.NewSize(toggleW, toggleH)
}

func (r *m3ToggleRenderer) Layout(size fyne.Size) {
	offY := (size.Height - toggleH) / 2
	if offY < 0 {
		offY = 0
	}
	r.toggle.bg.Move(fyne.NewPos(0, offY))
	r.toggle.bg.Resize(fyne.NewSize(toggleW, toggleH))
	cur := r.toggle.thumb.Position()
	r.toggle.thumb.Move(fyne.NewPos(cur.X, offY+togglePad))
}

func (r *m3ToggleRenderer) Refresh() {
	r.toggle.bg.Refresh()
	r.toggle.thumb.Refresh()
}

func (r *m3ToggleRenderer) Objects() []fyne.CanvasObject { return r.objects }
func (r *m3ToggleRenderer) BackgroundColor() color.Color { return color.Transparent }
func (r *m3ToggleRenderer) Destroy()                     {}

func (t *M3Toggle) SetChecked(v bool) {
	if t.checked == v {
		return
	}
	t.checked = v
	t.runAnimation(v)
	if t.OnStateChange != nil {
		t.OnStateChange(v)
	}
}

func (t *M3Toggle) runAnimation(on bool) {
	var endX float32
	if on {
		endX = toggleW - toggleThumb - togglePad
	} else {
		endX = togglePad
	}
	startX := t.thumb.Position().X
	startBg := t.bg.FillColor.(color.NRGBA)
	var endBg color.NRGBA
	if on {
		endBg = m3ToggleBgOn
	} else {
		endBg = m3ToggleBg
	}

	t.mu.Lock()
	t.animating = true
	t.mu.Unlock()

	offY := t.thumb.Position().Y

	go func() {
		const dur = 180 * time.Millisecond
		start := time.Now()
		for {
			elapsed := time.Since(start)
			p := math.Min(1, float64(elapsed)/float64(dur))
			e := 1 - math.Pow(1-p, 3)

			curX := startX + (endX-startX)*float32(e)
			rr := lerpU8(startBg.R, endBg.R, e)
			g := lerpU8(startBg.G, endBg.G, e)
			b := lerpU8(startBg.B, endBg.B, e)

			fyne.Do(func() {
				t.thumb.Move(fyne.NewPos(curX, offY))
				t.thumb.Refresh()
				t.bg.FillColor = color.NRGBA{R: rr, G: g, B: b, A: 0xFF}
				t.bg.Refresh()
			})

			if p >= 1 {
				break
			}
			time.Sleep(14 * time.Millisecond)
		}
		t.mu.Lock()
		t.animating = false
		t.mu.Unlock()
	}()
}

func (t *M3Toggle) Tapped(_ *fyne.PointEvent) {
	newVal := !t.checked
	t.checked = newVal
	t.runAnimation(newVal)
	if t.OnChange != nil {
		t.OnChange(newVal)
	}
	if t.OnStateChange != nil {
		t.OnStateChange(newVal)
	}
}

func (t *M3Toggle) TappedSecondary(_ *fyne.PointEvent) {}

func lerpU8(a, b uint8, t float64) uint8 {
	return uint8(float64(a)*(1-t) + float64(b)*t)
}

func NewM3ToggleWithLabel(text string, checked bool, onChange func(bool)) fyne.CanvasObject {
	toggle := NewM3Toggle(checked)
	toggle.OnChange = onChange
	label := canvas.NewText(text, color.NRGBA{R: 0xF0, G: 0xF0, B: 0xF0, A: 0xFF})
	label.TextSize = 13
	return container.NewBorder(nil, nil, nil, toggle, label)
}
