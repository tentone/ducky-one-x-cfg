package ui

import (
	"math"
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

func TestAnalogPressDepth(t *testing.T) {
	tests := []struct {
		age  float64
		want float64
	}{
		{age: -0.1, want: 0},
		{age: 0, want: 0},
		{age: 0.12, want: 0.5},
		{age: 0.24, want: 1},
		{age: 0.57, want: 0.5},
		{age: 0.9, want: 0},
	}
	for _, test := range tests {
		if got := analogPressDepth(test.age); math.Abs(got-test.want) > 0.001 {
			t.Fatalf("analogPressDepth(%v) = %v, want %v", test.age, got, test.want)
		}
	}
}

func TestAnalogBandMovesFromBottomToTop(t *testing.T) {
	tests := []struct {
		depth float64
		row   int
	}{
		{depth: 0, row: -1},
		{depth: 0.01, row: 5},
		{depth: 1.0 / 6, row: 4},
		{depth: 0.5, row: 2},
		{depth: 0.99, row: 0},
		{depth: 1, row: 0},
	}
	for _, test := range tests {
		if got := analogBandRow(test.depth); got != test.row {
			t.Fatalf("analogBandRow(%v) = %v, want %v", test.depth, got, test.row)
		}
	}
}

func TestAnalogBandLightsWholeIntersectingKeys(t *testing.T) {
	if !analogKeyIsLit(0.01, 5, 1) {
		t.Fatal("bottom-row key should be lit at the start of the sweep")
	}
	if analogKeyIsLit(0.01, 4, 1) {
		t.Fatal("row above the band should remain unlit")
	}
	if !analogKeyIsLit(0.35, 2, 2) {
		t.Fatal("a two-row key intersecting the band should light in full")
	}
}
