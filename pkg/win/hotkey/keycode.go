package hotkey

import (
	hook "github.com/robotn/gohook"
	"github.com/samber/lo"
	"github.com/something-that-is-cool/zutil/internal/misc"
)

var KeycodeToChar = lo.MapEntries(hook.Keycode, func(k string, v uint16) (uint16, string) {
	return v, k
})

var AllChars = func() []string {
	keys := lo.Values(KeycodeToChar)
	misc.SortByAlphabet(keys)
	return keys
}()
