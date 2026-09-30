// Package shortcuts parses and registers global profile-loading shortcuts.
package spec

import (
	"fmt"
	"strconv"
	"strings"
)

type Shortcut struct {
	Ctrl, Alt, Shift, Super bool
	Key                     string
}

func Keys() []string {
	keys := []string{}
	for c := 'A'; c <= 'Z'; c++ {
		keys = append(keys, string(c))
	}
	for c := '0'; c <= '9'; c++ {
		keys = append(keys, string(c))
	}
	for n := 1; n <= 12; n++ {
		keys = append(keys, "F"+strconv.Itoa(n))
	}
	return append(keys, "Space")
}

func Parse(value string) (Shortcut, error) {
	var result Shortcut
	if strings.TrimSpace(value) == "" {
		return result, nil
	}
	for _, part := range strings.Split(value, "+") {
		part = strings.ToUpper(strings.TrimSpace(part))
		switch part {
		case "CTRL", "CONTROL":
			if result.Ctrl {
				return result, fmt.Errorf("duplicate Ctrl modifier")
			}
			result.Ctrl = true
		case "ALT", "OPTION":
			if result.Alt {
				return result, fmt.Errorf("duplicate Alt modifier")
			}
			result.Alt = true
		case "SHIFT":
			if result.Shift {
				return result, fmt.Errorf("duplicate Shift modifier")
			}
			result.Shift = true
		case "SUPER", "WIN", "CMD", "COMMAND":
			if result.Super {
				return result, fmt.Errorf("duplicate Super modifier")
			}
			result.Super = true
		default:
			if result.Key != "" {
				return result, fmt.Errorf("choose a single shortcut key")
			}
			for _, key := range Keys() {
				if strings.EqualFold(part, key) {
					result.Key = key
					break
				}
			}
			if result.Key == "" {
				return result, fmt.Errorf("unsupported shortcut key %q", part)
			}
		}
	}
	if result.Key == "" || !(result.Ctrl || result.Alt || result.Super) {
		return result, fmt.Errorf("choose Ctrl, Alt, or Super and one key")
	}
	return result, nil
}

func (s Shortcut) String() string {
	if s.Key == "" {
		return ""
	}
	parts := []string{}
	if s.Ctrl {
		parts = append(parts, "Ctrl")
	}
	if s.Alt {
		parts = append(parts, "Alt")
	}
	if s.Shift {
		parts = append(parts, "Shift")
	}
	if s.Super {
		parts = append(parts, "Super")
	}
	return strings.Join(append(parts, s.Key), "+")
}
