package spec

import "testing"

func TestShortcutNormalizationAndInvalidCombinations(t *testing.T) {
	for input, want := range map[string]string{
		"": "", "alt + control + 1": "Ctrl+Alt+1", "shift+cmd+f11": "Shift+Super+F11", "Win+space": "Super+Space",
	} {
		s, err := Parse(input)
		if err != nil || s.String() != want {
			t.Fatalf("Parse(%q) = %q, %v; want %q", input, s.String(), err, want)
		}
	}
	for _, input := range []string{"A", "Shift+A", "Ctrl", "Ctrl+A+B", "Ctrl++A", "Ctrl+Ctrl+A", "Ctrl+F13"} {
		if _, err := Parse(input); err == nil {
			t.Fatalf("accepted invalid shortcut %q", input)
		}
	}
}
