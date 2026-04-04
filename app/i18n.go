package app

import (
	"strings"

	"github.com/something-that-is-cool/zutil/app/module"
)

const (
	langRU = "ru"
	langEN = "en"
)

func normalizeLanguage(lang string) string {
	if strings.EqualFold(strings.TrimSpace(lang), langEN) {
		return langEN
	}
	return langRU
}

func (app *App) t(ru, en string) string {
	if normalizeLanguage(app.language) == langEN {
		return en
	}
	return ru
}

func canonicalModuleName(name string) string {
	key := normalizeModuleKey(name)
	if canonical, ok := moduleNameAliases[key]; ok {
		return canonical
	}
	return key
}

func normalizeModuleKey(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (app *App) moduleDisplayDescription(m module.Module) string {
	key := canonicalModuleName(m.Name())
	lang := normalizeLanguage(app.language)
	if lang == langEN {
		if v, ok := moduleDescriptionsEN[key]; ok {
			return v
		}
		return m.Description()
	}
	if v, ok := moduleDescriptionsRU[key]; ok {
		return v
	}
	return m.Description()
}

var moduleNameAliases = map[string]string{
	"autosprint":            "autosprint",
	"nohurtcam":             "nohurtcam",
	"nocamreset":            "nocamreset",
	"nodynamicfov":          "nodynamicfov",
	"noparticle":            "noparticle",
	"novsync":               "offvsync",
	"offvsync":              "offvsync",
	"controllersensitivity": "controllersensitivity",
	"zoom":                  "zoom",
	"timechanger":           "timechanger",
	"itemusedelay":          "itemusedelay",
	"itemdelayfix":          "itemusedelay",
}

var moduleDescriptionsEN = map[string]string{
	"autosprint":            "Keeps sprint always enabled without holding the sprint key.",
	"nohurtcam":             "Disables camera shake when taking damage.",
	"nocamreset":            "Disables camera reset interpolation during teleports.",
	"nodynamicfov":          "Prevents FOV changes while sprinting or taking damage.",
	"noparticle":            "Disables particle rendering for a cleaner image and better FPS.",
	"offvsync":              "Forces VSync off.",
	"controllersensitivity": "Increases controller sensitivity beyond the default in-game limit.",
	"zoom":                  "Zooms in while the selected key is held.",
	"timechanger":           "Changes and freezes in-game time at the selected value.",
	"itemusedelay":          "Removes the 200 ms item-use delay after an attack.",
}

var moduleDescriptionsRU = map[string]string{
	"autosprint":            "Постоянный спринт без удержания клавиши.",
	"nohurtcam":             "Отключает тряску камеры при получении урона.",
	"nocamreset":            "Отключает сброс камеры при телепорте.",
	"nodynamicfov":          "Отключает изменение FOV при спринте и получении урона.",
	"noparticle":            "Отключает частицы для более чистой картинки и высокого FPS.",
	"offvsync":              "Принудительно отключает VSync.",
	"controllersensitivity": "Повышает чувствительность контроллера выше лимита игры.",
	"zoom":                  "Приближает изображение при удержании выбранной клавиши.",
	"timechanger":           "Изменяет и фиксирует внутриигровое время на выбранном значении.",
	"itemusedelay":          "Убирает задержку 200 мс после атаки при использовании предметов.",
}
