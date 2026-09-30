//go:build !windows && !linux && !darwin

package startup

import "fmt"

func LegacyEnabled() bool   { return false }
func Enabled() bool         { return false }
func SetEnabled(bool) error { return fmt.Errorf("automatic startup is unavailable on this platform") }
