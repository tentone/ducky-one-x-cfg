//go:build !windows && !darwin && !linux

package shortcuts

import "fmt"

func registerNative(Shortcut) (registration, error) {
	return nil, fmt.Errorf("global profile shortcuts are unavailable on this platform")
}
