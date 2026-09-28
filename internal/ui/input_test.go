package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

func TestPreviewMatrixIndex(t *testing.T) {
	tests := []struct {
		name fyne.KeyName
		want int
	}{
		{name: fyne.KeyEscape, want: 0},
		{name: fyne.KeyA, want: 64},
		{name: fyne.KeyReturn, want: 76},
		{name: desktop.KeyShiftLeft, want: 84},
		{name: fyne.KeySpace, want: 110},
		{name: fyne.KeyRight, want: 121},
	}
	for _, test := range tests {
		got, ok := previewMatrixIndex(test.name)
		if !ok || got != test.want {
			t.Fatalf("previewMatrixIndex(%q) = %d, %v; want %d, true", test.name, got, ok, test.want)
		}
	}
	if _, ok := previewMatrixIndex(fyne.KeyUnknown); ok {
		t.Fatal("unknown key should not map to the keyboard preview")
	}
}
