package ui

import (
	"image/color"
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

func TestBlueRedGradient(t *testing.T) {
	tests := []struct {
		phase float64
		want  color.NRGBA
	}{
		{phase: 0, want: color.NRGBA{R: 0, G: 0, B: 255, A: 255}},
		{phase: 0.25, want: color.NRGBA{R: 128, G: 0, B: 128, A: 255}},
		{phase: 0.5, want: color.NRGBA{R: 255, G: 0, B: 0, A: 255}},
		{phase: 0.75, want: color.NRGBA{R: 128, G: 0, B: 128, A: 255}},
		{phase: 1, want: color.NRGBA{R: 0, G: 0, B: 255, A: 255}},
	}
	for _, test := range tests {
		red, green, blue := blueRedGradient(test.phase, 1)
		got := color.NRGBA{R: red, G: green, B: blue, A: 255}
		if got != test.want {
			t.Fatalf("blueRedGradient(%v, 1) = %#v, want %#v", test.phase, got, test.want)
		}
	}
}

func TestRippleStartsAtPressedKey(t *testing.T) {
	if got := rippleIntensity(0, 0, 1); math.Abs(got-1) > 0.001 {
		t.Fatalf("pressed key intensity = %v, want 1", got)
	}
	if got := rippleIntensity(0, 3, 1); got >= 0.1 {
		t.Fatalf("distant key intensity = %v, want less than 0.1", got)
	}
	if got := rippleIntensity(2, 0, 1); got != 0 {
		t.Fatalf("expired ripple intensity = %v, want 0", got)
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

func TestAnalogHighestLitRowMovesFromBottomToTop(t *testing.T) {
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
		if got := analogHighestLitRow(test.depth); got != test.row {
			t.Fatalf("analogHighestLitRow(%v) = %v, want %v", test.depth, got, test.row)
		}
	}
}

func TestAnalogFillKeepsAllLowerRowsLit(t *testing.T) {
	if !analogKeyIsLit(0.01, 5, 1) {
		t.Fatal("bottom-row key should be lit at the start of the sweep")
	}
	if analogKeyIsLit(0.01, 4, 1) {
		t.Fatal("row above the band should remain unlit")
	}
	if !analogKeyIsLit(0.5, 2, 1) {
		t.Fatal("the highest reached row should be lit")
	}
	if !analogKeyIsLit(0.5, 3, 1) || !analogKeyIsLit(0.5, 4, 1) || !analogKeyIsLit(0.5, 5, 1) {
		t.Fatal("every row below the highest reached row should remain lit")
	}
	if analogKeyIsLit(0.5, 1, 1) {
		t.Fatal("a row above the fill should remain unlit")
	}
	if !analogKeyIsLit(0.35, 2, 2) {
		t.Fatal("a two-row key intersecting the fill should light in full")
	}
}
