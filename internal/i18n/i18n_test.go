package i18n

import (
	"regexp"
	"testing"
)

var formatVerb = regexp.MustCompile(`%[sd]`)

func TestInstalledLanguagesAreComplete(t *testing.T) {
	catalog := New()
	seenNames := make(map[string]bool)
	if got := len(Languages()); got != 5 {
		t.Fatalf("installed language count = %d, want 5", got)
	}
	for _, language := range Languages() {
		name := NativeName(language)
		if name == "" || seenNames[name] {
			t.Fatalf("invalid or duplicate native language name %q", name)
		}
		seenNames[name] = true
		if !catalog.SetLanguage(language) {
			t.Fatalf("cannot select installed language %q", language)
		}
		translations := catalog.strings[language]
		for id, englishValue := range english {
			translated, ok := translations[id]
			if !ok || translated == "" {
				t.Errorf("language %q is missing %q", language, id)
				continue
			}
			englishVerbs := formatVerb.FindAllString(englishValue, -1)
			translatedVerbs := formatVerb.FindAllString(translated, -1)
			if len(englishVerbs) != len(translatedVerbs) {
				t.Errorf("language %q has incompatible formatting for %q: %q", language, id, translated)
			}
		}
	}
}

func TestCatalogFallsBackToEnglish(t *testing.T) {
	catalog := New()
	catalog.strings[French] = map[string]string{}
	if !catalog.SetLanguage(French) {
		t.Fatal("could not select French")
	}
	if got := catalog.T("action.apply"); got != english["action.apply"] {
		t.Fatalf("fallback = %q, want %q", got, english["action.apply"])
	}
}
