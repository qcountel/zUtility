package app

import "strings"

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
	"instanthotbar":         "instanthotbar",
}
