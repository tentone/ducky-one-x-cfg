package ui

import (
	"testing"

	"github.com/joseferrao/ducky-drv/internal/protocol"
)

func TestFullSizeKeyboardLayoutCoversMatrixKeysOnce(t *testing.T) {
	seen := make(map[int]bool)
	for _, key := range fullSizeKeyboardLayout() {
		if seen[key.index] {
			t.Fatalf("matrix key %d appears more than once", key.index)
		}
		seen[key.index] = true
		if key.width <= 0 || key.height <= 0 {
			t.Fatalf("matrix key %d has invalid size", key.index)
		}
	}
	for _, option := range protocol.MatrixKeyOptions() {
		index, ok := parseMatrixIndex(option)
		if !ok || !seen[index] {
			t.Fatalf("matrix key option %q is missing from the visual layout", option)
		}
	}
}
