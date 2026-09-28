package protocol

import (
	"context"
	"fmt"
	"math"
	"testing"
)

func TestDistanceRoundTrip(t *testing.T) {
	for step := 1; step <= 35; step++ {
		want := float64(step) / 10
		got := DistanceMM(DistanceCode(want))
		if math.Abs(got-want) > 0.001 {
			t.Fatalf("distance %.1f round-tripped to %.1f", want, got)
		}
	}
}

func TestMacroRoundTrip(t *testing.T) {
	want := []MacroAction{
		{Kind: MacroPress, Keys: []byte{4, 225}},
		{Kind: MacroDelay, DelayMS: 275, RandomMS: 25},
		{Kind: MacroRelease, Keys: []byte{4, 225}},
		{Kind: MacroText, Text: "hello 123/*"},
	}
	raw, err := encodeMacro(want)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != macroBytes {
		t.Fatalf("encoded macro has %d bytes, want %d", len(raw), macroBytes)
	}
	got, err := decodeMacro(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("decoded %d actions, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Description() != want[i].Description() {
			t.Errorf("action %d = %q, want %q", i, got[i].Description(), want[i].Description())
		}
	}
}

func TestMacroRejectsUnsupportedText(t *testing.T) {
	_, err := encodeMacro([]MacroAction{{Kind: MacroText, Text: "café"}})
	if err == nil {
		t.Fatal("expected unsupported character error")
	}
}

func TestKeyMapChunking(t *testing.T) {
	fake := &fakeExchange{fn: func(report []byte, expected byte) ([]byte, error) {
		if expected != 22 || report[2] != 21 {
			return nil, fmt.Errorf("unexpected request %v", report)
		}
		section := int(report[5])
		length := 52
		if section == 4 {
			length = 44
		}
		response := []byte{0x66, byte(length), 22, 0, 5, byte(section)}
		for i := 0; i < length/2; i++ {
			response = append(response, byte(AssignmentKeyboard), byte(section*26+i))
		}
		return response, nil
	}}
	client := NewClient(fake)
	mapping, err := client.KeyMap(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(mapping) != KeyMapEntries || mapping[125].Code != 125 {
		t.Fatalf("invalid mapping tail: len=%d last=%+v", len(mapping), mapping[len(mapping)-1])
	}
	if fake.calls != 5 {
		t.Fatalf("got %d requests, want 5", fake.calls)
	}
}

func TestLightingCodec(t *testing.T) {
	fake := &fakeExchange{fn: func(report []byte, expected byte) ([]byte, error) {
		if expected == 6 {
			return []byte{0x66, 9, 6, 13, 4, 10, 20, 30, 8, 2, 0, 7}, nil
		}
		if expected != 8 {
			return nil, fmt.Errorf("unexpected command %d", expected)
		}
		if report[3] != 13 || report[4] != 4 || report[8] != 8 {
			return nil, fmt.Errorf("wrong encoded lighting report %v", report)
		}
		return []byte{0x66, 1, 8, 0}, nil
	}}
	client := NewClient(fake)
	settings, err := client.Lighting(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if settings.Speed != 60 || settings.Brightness != 75 || settings.Variant != 2 {
		t.Fatalf("unexpected decoded settings: %+v", settings)
	}
	if err := client.SetLighting(context.Background(), settings); err != nil {
		t.Fatal(err)
	}
}

type fakeExchange struct {
	calls int
	fn    func([]byte, byte) ([]byte, error)
}

func (f *fakeExchange) Exchange(_ context.Context, report []byte, expected byte) ([]byte, error) {
	f.calls++
	return f.fn(report, expected)
}
