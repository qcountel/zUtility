package app

import (
	"image/color"

	"github.com/something-that-is-cool/zutil/pkg/e"
)

func (app *App) ifUserConf(x func(*UserConfig) bool, fn func()) {
	res := false
	func() {
		app.userConf.Lock()
		defer app.userConf.Unlock()
		res = x(app.userConf.V)
	}()
	if res {
		fn()
	}
}

var _ e.ErrorHandler = (*App)(nil)

func (app *App) HandleError(src string, err error) {
	app.conf.Logger.Error("error handled", "src", src, "err", err.Error())
}

func (app *App) themeCardBg() color.NRGBA {
	if app.lightTheme {
		return color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	}
	return color.NRGBA{R: 0x0D, G: 0x0D, B: 0x0D, A: 255}
}

