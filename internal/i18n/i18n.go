// Package i18n contains the application's user-facing strings.
//
// Keeping strings behind stable identifiers makes adding another language a
// data-only change instead of a GUI rewrite.
package i18n

import "sync"

// Language identifies an installed translation.
type Language string

const (
	English            Language = "en"
	French             Language = "fr"
	German             Language = "de"
	PortuguesePortugal Language = "pt-PT"
	ChineseSimplified  Language = "zh-CN"
)

var installedLanguages = []Language{English, French, German, PortuguesePortugal, ChineseSimplified}

var nativeNames = map[Language]string{
	English:            "English",
	French:             "Français",
	German:             "Deutsch",
	PortuguesePortugal: "Português (Portugal)",
	ChineseSimplified:  "中文（简体）",
}

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
			English: english, French: french, German: german,
			PortuguesePortugal: portuguesePortugal, ChineseSimplified: chineseSimplified,
		},
	}
}

// Languages returns the installed languages in display order.
func Languages() []Language { return append([]Language(nil), installedLanguages...) }

// NativeName returns the language name written in that language.
func NativeName(language Language) string {
	if name, ok := nativeNames[language]; ok {
		return name
	}
	return string(language)
}

// Language returns the active language.
func (c *Catalog) Language() Language {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.language
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
