package modulesutil

import (
	"image/color"
	"math"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

var (
	sliderTrackColor  = color.NRGBA{R: 0x28, G: 0x14, B: 0x14, A: 0xFF}
	sliderActiveColor = color.NRGBA{R: 0xBB, G: 0x1E, B: 0x1E, A: 0xFF}
	sliderThumbColor  = color.NRGBA{R: 0xFF, G: 0xDD, B: 0xDD, A: 0xFF}
)

const (
	trackH    = float32(5)
	thumbSize = float32(14)
)

type M3BounceSlider struct {
	widget.BaseWidget

	Min, Max, Value float64
	Step            float64

	OnChanged     func(float64)
	OnChangeEnded func(float64)

	mu           sync.Mutex
	displayValue float64
	animating    bool
	dragging     bool

	track  *canvas.Rectangle
	active *canvas.Rectangle
	thumb  *canvas.Rectangle
}

func NewM3BounceSlider(min, max, value float64) *M3BounceSlider {
	s := &M3BounceSlider{
		Min:          min,
		Max:          max,
		Value:        value,
		displayValue: value,
		Step:         1,
	}
	s.track = canvas.NewRectangle(sliderTrackColor)
	s.track.CornerRadius = 3
	s.active = canvas.NewRectangle(sliderActiveColor)
	s.active.CornerRadius = 3
	s.thumb = canvas.NewRectangle(sliderThumbColor)
	s.thumb.CornerRadius = 7
	s.ExtendBaseWidget(s)
	return s
}

func (s *M3BounceSlider) CreateRenderer() fyne.WidgetRenderer {
	return &m3SliderRenderer{
		slider:  s,
		objects: []fyne.CanvasObject{s.track, s.active, s.thumb},
	}
}

type m3SliderRenderer struct {
	slider  *M3BounceSlider
	objects []fyne.CanvasObject
	size    fyne.Size
}

func (r *m3SliderRenderer) MinSize() fyne.Size { return fyne.NewSize(80, 26) }

func (r *m3SliderRenderer) Layout(size fyne.Size) {
	r.size = size
	r.layout(size)
}

func (r *m3SliderRenderer) layout(size fyne.Size) {
	s := r.slider
	if size.Width == 0 {
		size = r.size
	}
	if size.Width == 0 {
		return
	}

	s.mu.Lock()
	pct := float32(clampF((s.displayValue-s.Min)/(s.Max-s.Min), 0, 1))
	s.mu.Unlock()

	trackY := (size.Height - trackH) / 2
	thumbY := (size.Height - thumbSize) / 2

	s.track.Move(fyne.NewPos(0, trackY))
	s.track.Resize(fyne.NewSize(size.Width, trackH))

	activeW := pct * size.Width
	s.active.Move(fyne.NewPos(0, trackY))
	s.active.Resize(fyne.NewSize(activeW, trackH))

	thumbX := pct*size.Width - thumbSize/2
	s.thumb.Move(fyne.NewPos(thumbX, thumbY))
	s.thumb.Resize(fyne.NewSize(thumbSize, thumbSize))
}

func (r *m3SliderRenderer) Refresh() {
	r.layout(r.size)
	r.slider.track.Refresh()
	r.slider.active.Refresh()
	r.slider.thumb.Refresh()
}

func (r *m3SliderRenderer) Objects() []fyne.CanvasObject { return r.objects }
func (r *m3SliderRenderer) BackgroundColor() color.Color { return color.Transparent }
func (r *m3SliderRenderer) Destroy()                     {}

func (s *M3BounceSlider) Tapped(e *fyne.PointEvent) {
	size := s.Size()
	if size.Width == 0 {
		return
	}
	pct := clampF(float64(e.Position.X)/float64(size.Width), 0, 1)
	target := s.Min + pct*(s.Max-s.Min)
	if s.Step > 0 {
		target = math.Round(target/s.Step) * s.Step
	}
	s.mu.Lock()
	s.Value = target
	s.mu.Unlock()

	if s.OnChanged != nil {
		s.OnChanged(target)
	}
	s.SetValueAnimated(target)
	if s.OnChangeEnded != nil {
		s.OnChangeEnded(target)
	}
}

func (s *M3BounceSlider) Dragged(e *fyne.DragEvent) {
	size := s.Size()
	if size.Width == 0 {
		return
	}
	s.dragging = true
	pct := clampF(float64(e.Position.X)/float64(size.Width), 0, 1)
	v := s.Min + pct*(s.Max-s.Min)
	if s.Step > 0 {
		v = math.Round(v/s.Step) * s.Step
	}
	s.mu.Lock()
	s.displayValue = v
	s.Value = v
	s.mu.Unlock()
	s.Refresh()
	if s.OnChanged != nil {
		s.OnChanged(v)
	}
}

func (s *M3BounceSlider) DragEnd() {
	s.dragging = false
	if s.OnChangeEnded != nil {
		s.OnChangeEnded(s.Value)
	}
}

func (s *M3BounceSlider) SetValueAnimated(target float64) {
	s.mu.Lock()
	start := s.displayValue
	s.Value = target
	s.animating = true
	s.mu.Unlock()

	go func() {
		const dur = 200 * time.Millisecond
		startT := time.Now()
		for {
			elapsed := time.Since(startT)
			p := math.Min(1, float64(elapsed)/float64(dur))
			e := easeOutCubic(p)
			s.mu.Lock()
			s.displayValue = start + (target-start)*e
			s.mu.Unlock()
			fyne.Do(func() { s.Refresh() })
			if p >= 1 {
				break
			}
			time.Sleep(14 * time.Millisecond)
		}
		s.mu.Lock()
		s.animating = false
		s.mu.Unlock()
	}()
}

func (s *M3BounceSlider) SetValueImmediate(target float64) {
	s.mu.Lock()
	s.Value = target
	s.displayValue = target
	s.mu.Unlock()
	s.Refresh()
}

func (s *M3BounceSlider) MinSize() fyne.Size { return fyne.NewSize(80, 26) }

func easeOutCubic(t float64) float64 {
	return 1 - math.Pow(1-t, 3)
}

func clampF(v, mn, mx float64) float64 {
	if v < mn {
		return mn
	}
	if v > mx {
		return mx
	}
	return v
}
