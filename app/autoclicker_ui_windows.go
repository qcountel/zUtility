//go:build windows

package app

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/qcountel/zUtility/app/module/modules/modulesutil"
)

func (app *App) buildClickerContent() fyne.CanvasObject {
	ac := app.getAutoClicker()
	title := ctxt("CLICKER", textPrimary, 22)
	title.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}

	enabledToggle := modulesutil.NewM3Toggle(ac.Enabled())
	enabledToggle.OnChange = func(v bool) {
		ac.SetEnabled(v)
		if err := persistAutoClicker(ac); err != nil {
			app.conf.Logger.Warn("failed to save autoclicker config", "err", err)
		}
	}

	minLabel := widget.NewLabel("")
	maxLabel := widget.NewLabel("")
	refreshRangeLabels := func() {
		min, max := ac.Range()
		minLabel.SetText(fmt.Sprintf("Минимум: %.0f CPS", min))
		maxLabel.SetText(fmt.Sprintf("Максимум: %.0f CPS", max))
	}
	refreshRangeLabels()

	minSlider := modulesutil.NewM3BounceSlider(1, 30, 1)
	maxSlider := modulesutil.NewM3BounceSlider(1, 30, 1)
	minSlider.Step = 1
	maxSlider.Step = 1
	min, max := ac.Range()
	minSlider.SetValueImmediate(min)
	maxSlider.SetValueImmediate(max)
	minSlider.OnChanged = func(v float64) {
		if v > maxSlider.Value {
			maxSlider.SetValueImmediate(v)
		}
		ac.SetRange(v, maxSlider.Value)
		refreshRangeLabels()
	}
	minSlider.OnChangeEnded = func(_ float64) {
		if err := persistAutoClicker(ac); err != nil {
			app.conf.Logger.Warn("failed to save autoclicker config", "err", err)
		}
	}
	maxSlider.OnChanged = func(v float64) {
		if v < minSlider.Value {
			minSlider.SetValueImmediate(v)
		}
		ac.SetRange(minSlider.Value, v)
		refreshRangeLabels()
	}
	maxSlider.OnChangeEnded = func(_ float64) {
		if err := persistAutoClicker(ac); err != nil {
			app.conf.Logger.Warn("failed to save autoclicker config", "err", err)
		}
	}

	bindHint := widget.NewLabel("Клавиша переключения")
	bindHint.Wrapping = fyne.TextWrapWord
	bindBtn := widget.NewButton(autoClickerVKName(ac.BindVK()), nil)
	bindBtn.Importance = widget.LowImportance
	bindBg := canvas.NewRectangle(calpha(accentRed, 0x14))
	bindBg.CornerRadius = 3
	bindBtnWrap := container.NewStack(bindBg, bindBtn)
	bindBtn.OnTapped = func() {
		bindHint.SetText("Нажмите любую клавишу...")
		bindBtn.Disable()
		bindBg.FillColor = calpha(accentOrange, 0x20)
		bindBg.Refresh()
		ctx, cancel := withAutoClickerCaptureTimeout()
		go func() {
			defer cancel()
			vk, ok := ac.CaptureBind(ctx)
			fyne.Do(func() {
				defer bindBtn.Enable()
				bindBg.FillColor = calpha(accentRed, 0x14)
				bindBg.Refresh()
				if !ok {
					bindHint.SetText("Захват отменён")
					return
				}
				ac.SetBindVK(vk)
				bindBtn.SetText(autoClickerVKName(vk))
				bindHint.SetText("Клавиша переключения")
				if err := persistAutoClicker(ac); err != nil {
					app.conf.Logger.Warn("failed to save autoclicker config", "err", err)
				}
			})
		}()
	}

	origEnabledChange := enabledToggle.OnChange
	enabledToggle.OnChange = func(v bool) {
		origEnabledChange(v)
	}
	ac.onEnabledChange = func(v bool) {
		fyne.Do(func() {
			enabledToggle.SetChecked(v)
		})
	}

	leftModeBg := canvas.NewRectangle(calpha(accentRed, 0x10))
	leftModeBg.CornerRadius = 3
	rightModeBg := canvas.NewRectangle(calpha(accentRed, 0x10))
	rightModeBg.CornerRadius = 3
	leftModeBtn := NewM3AnimatedButton("Левая кнопка", func() {})
	rightModeBtn := NewM3AnimatedButton("Правая кнопка", func() {})
	leftModeBtn.Importance = widget.LowImportance
	rightModeBtn.Importance = widget.LowImportance
	setModeUI := func(mode clickMode) {
		if mode == clickRight {
			rightModeBg.FillColor = calpha(accentRed, 0x24)
			leftModeBg.FillColor = calpha(accentRed, 0x10)
		} else {
			leftModeBg.FillColor = calpha(accentRed, 0x24)
			rightModeBg.FillColor = calpha(accentRed, 0x10)
		}
		leftModeBg.Refresh()
		rightModeBg.Refresh()
	}
	selectMode := func(mode clickMode) {
		ac.SetMode(mode)
		setModeUI(mode)
		if err := persistAutoClicker(ac); err != nil {
			app.conf.Logger.Warn("failed to save autoclicker config", "err", err)
		}
	}
	leftModeBtn.OnTapped = func() { selectMode(clickLeft) }
	rightModeBtn.OnTapped = func() { selectMode(clickRight) }
	setModeUI(ac.Mode())
	modeButtons := container.NewGridWithColumns(2,
		container.NewStack(leftModeBg, leftModeBtn),
		container.NewStack(rightModeBg, rightModeBtn),
	)

	stateDesc := widget.NewLabel("Работает только при удержании выбранной кнопки мыши. Горячая клавиша включает и выключает кликер.")
	stateDesc.Wrapping = fyne.TextWrapWord

	jitterToggle := modulesutil.NewM3Toggle(ac.JitterEnabled())
	jitterToggle.OnChange = func(v bool) {
		ac.SetJitterEnabled(v)
		if err := persistAutoClicker(ac); err != nil {
			app.conf.Logger.Warn("failed to save autoclicker config", "err", err)
		}
	}
	jitterRow := container.NewBorder(nil, nil, nil, jitterToggle, ctxt("Джиттер (микро-рандомизация)", textPrimary, 14))

	stateRow := container.NewBorder(nil, nil, nil, enabledToggle, ctxt("Автокликер", textPrimary, 14))
	stateSection := createSettingsSection("Состояние", container.NewVBox(
		stateRow,
		container.New(layout.NewCustomPaddedLayout(0, 6, 0, 0), stateDesc),
		container.New(layout.NewCustomPaddedLayout(0, 10, 0, 0), jitterRow),
	))

	minRow := container.NewVBox(
		minLabel,
		container.New(layout.NewCustomPaddedLayout(0, 2, 0, 0), minSlider),
	)
	maxRow := container.NewVBox(
		maxLabel,
		container.New(layout.NewCustomPaddedLayout(0, 2, 0, 0), maxSlider),
	)
	rangeSection := createSettingsSection("Скорость клика", container.NewVBox(
		minRow,
		container.New(layout.NewCustomPaddedLayout(0, 10, 0, 0), maxRow),
	))

	modeDesc := widget.NewLabel("Выберите, какую кнопку мыши кликер будет повторять при удержании.")
	modeDesc.Wrapping = fyne.TextWrapWord
	modeSection := createSettingsSection("Режим клика", container.NewVBox(
		modeButtons,
		container.New(layout.NewCustomPaddedLayout(0, 6, 0, 0), modeDesc),
	))

	bindRow := container.NewBorder(nil, nil, nil, bindBtnWrap, bindHint)
	bindSection := createSettingsSection("Горячая клавиша", container.NewVBox(
		bindRow,
		container.New(layout.NewCustomPaddedLayout(0, 6, 0, 0), ctxt("Нажмите кнопку справа, затем любую клавишу на клавиатуре.", textDim, 13)),
	))

	content := container.NewVBox(
		container.New(layout.NewCustomPaddedLayout(16, 6, 14, 12), title),
		stateSection,
		rangeSection,
		modeSection,
		bindSection,
	)
	return container.NewVScroll(content)
}
