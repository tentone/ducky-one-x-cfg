// Package i18n contains the application's user-facing strings.
//
// Keeping strings behind stable identifiers makes adding another language a
// data-only change instead of a GUI rewrite.
package i18n

import "sync"

// Language identifies an installed translation.
type Language string

const English Language = "en"

// Catalog is a concurrency-safe translation catalog.
type Catalog struct {
	mu       sync.RWMutex
	language Language
	strings  map[Language]map[string]string
}

// New returns the built-in catalog. English is the initial language.
func New() *Catalog {
	return &Catalog{
		language: English,
		strings: map[Language]map[string]string{
			English: english,
		},
	}
}

// SetLanguage changes the active language when it is installed.
func (c *Catalog) SetLanguage(language Language) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.strings[language]; !ok {
		return false
	}
	c.language = language
	return true
}

// T translates an identifier, falling back to the identifier itself.
func (c *Catalog) T(id string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if value, ok := c.strings[c.language][id]; ok {
		return value
	}
	if value, ok := c.strings[English][id]; ok {
		return value
	}
	return id
}

var english = map[string]string{
	"app.title":               "Ducky One X Configurator",
	"app.subtitle":            "Offline keyboard configuration",
	"action.apply":            "Apply",
	"action.connect":          "Connect",
	"action.disconnect":       "Disconnect",
	"action.load":             "Load from keyboard",
	"action.reset":            "Reset",
	"action.add":              "Add action",
	"action.remove":           "Remove selected",
	"action.refresh":          "Refresh devices",
	"action.select":           "Select",
	"connection.disconnected": "No compatible keyboard connected",
	"connection.ready":        "Keyboard ready",
	"connection.working":      "Communicating with keyboard…",
	"connection.wired":        "Connect the keyboard by USB cable, then choose Connect.",
	"connection.opening":      "Opening %s…",
	"connection.found":        "%d compatible interface(s) found",
	"device.summary":          "%s · firmware %s",
	"device":                  "Device",
	"firmware":                "Firmware",
	"profile":                 "Memory profile",
	"profile.1":               "Profile 1",
	"profile.2":               "Profile 2",
	"theme":                   "Theme",
	"theme.system":            "System",
	"theme.light":             "Light",
	"theme.dark":              "Dark",
	"language":                "Language",
	"tab.keys":                "Key settings",
	"tab.lighting":            "Lighting",
	"tab.actuation":           "Actuation",
	"tab.mpt":                 "Multi-point trigger",
	"tab.macros":              "Macros",
	"layer":                   "Layer",
	"layer.base":              "Base layer",
	"layer.fn":                "Fn layer",
	"key":                     "Key",
	"assignment":              "Assignment",
	"assignment.default":      "Default",
	"assignment.keyboard":     "Keyboard key",
	"assignment.macro":        "Macro",
	"assignment.mouse":        "Mouse button",
	"assignment.mpt":          "MPT preset",
	"assignment.disabled":     "Disabled",
	"keys.description":        "Choose a matrix position and assign a keyboard key, macro, mouse button, or MPT preset. Changes are written to the selected onboard layer.",
	"current":                 "Current",
	"effect":                  "Effect",
	"color":                   "Color",
	"brightness":              "Brightness",
	"speed":                   "Speed",
	"direction":               "Direction / variant",
	"random.color":            "Random color",
	"lighting.description":    "Select an onboard RGB effect and its parameters. Changes are stored locally on the keyboard.",
	"effect.static":           "Static",
	"effect.breathing":        "Breathing",
	"effect.cycle":            "Color cycle",
	"effect.reactive":         "Reactive",
	"effect.ripple":           "Ripple",
	"effect.rainbow":          "Rainbow",
	"effect.analog":           "Analog reactive",
	"effect.off":              "Lights off",
	"actuation.point":         "Actuation point",
	"rapid.trigger":           "Rapid trigger",
	"release.distance":        "Release distance",
	"selection.all":           "Apply to all matrix keys",
	"actuation.description":   "Set the press actuation point and rapid-trigger release distance for an individual matrix position or the entire keyboard. MPT-controlled keys are identified when settings are loaded.",
	"actuation.fixed":         "Fixed actuation",
	"actuation.rapid":         "Rapid trigger",
	"actuation.mpt":           "Controlled by MPT",
	"mpt.preset":              "MPT preset",
	"mpt.stage":               "Stage",
	"press.distance":          "Press distance",
	"release.point":           "Release distance",
	"output.key":              "Output key",
	"mpt.description":         "Each MPT preset can emit up to four outputs at separate press and release distances. Assign a preset to a physical key from Key settings.",
	"output.disabled":         "Disabled",
	"macro.slot":              "Macro slot",
	"macro.kind":              "Action type",
	"macro.value":             "Key, text, or milliseconds",
	"macro.click":             "Click",
	"macro.press":             "Press",
	"macro.release":           "Release",
	"macro.delay":             "Delay",
	"macro.text":              "Text",
	"macro.description":       "Build macros from press, release, click, delay, and text actions. Text actions support the keyboard firmware's 20-character ASCII subset; pair press and release actions for held keys.",
	"macro.placeholder":       "Used for delay (ms) or text",
	"status.saved":            "Settings saved to the keyboard.",
	"status.loaded":           "Settings loaded from the keyboard.",
	"status.profile":          "Memory profile switched.",
	"status.reset":            "Settings reset.",
	"error.no_device":         "Connect a Ducky One X first.",
	"error.communication":     "Keyboard communication failed",
	"error.invalid":           "Check the highlighted values and try again.",
	"reset.confirm":           "Reset %s on the keyboard? This cannot be undone.",
	"about.protocol":          "Uses the Ducky One X HID protocol locally. No account or network connection is required.",
}
