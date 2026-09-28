package protocol

import (
	"bytes"
	"context"
	"errors"
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

func TestRainbowLightingCodec(t *testing.T) {
	fake := &fakeExchange{fn: func(report []byte, expected byte) ([]byte, error) {
		want := []byte{0x66, 9, 7, 21, 9, 10, 20, 30, 9, 2, 2, 3, 0x66, 0x0d, 0x0a}
		if !bytes.Equal(report, want) {
			return nil, fmt.Errorf("wrong rainbow lighting report %v, want %v", report, want)
		}
		return []byte{0x66, 1, 8, 0}, nil
	}}
	settings := LightingSettings{
		Effect: 21, Speed: 10, Red: 10, Green: 20, Blue: 30,
		Brightness: 100, Variant: 2, ColorMode: 3,
	}
	if err := NewClient(fake).SetLighting(context.Background(), settings); err != nil {
		t.Fatal(err)
	}
	if got := decodeSpeed(9); got != 10 {
		t.Fatalf("decodeSpeed(9) = %d, want 10", got)
	}
}

func TestCustomLightingCodec(t *testing.T) {
	step := 0
	fake := &fakeExchange{fn: func(report []byte, expected byte) ([]byte, error) {
		defer func() { step++ }()
		switch step {
		case 0:
			want := []byte{0x66, 2, 12, 0x66, 0x0d, 0x0a}
			if expected != 13 || !bytes.Equal(report, want) {
				return nil, fmt.Errorf("wrong custom mode request %v", report)
			}
			return []byte{0x66, 1, 13, 7}, nil
		case 1:
			want := []byte{0x66, 2, 15, 7, 2, 0, 0x0d, 0x0a}
			if expected != 16 || !bytes.Equal(report, want) {
				return nil, fmt.Errorf("wrong custom data request %v", report)
			}
			payload := []byte{85, 4, 0, 0, 0, 2, 0, 1, 2, 3, 124, 4, 5, 6}
			response := []byte{0x66, byte(len(payload)), 16, 7, 1, 0}
			return append(response, payload...), nil
		case 2:
			want := []byte{0x66, 14, 17, 7, 1, 0, 85, 4, 0, 0, 0, 2, 0, 1, 2, 3, 124, 4, 5, 6, 0x0d, 0x0a}
			if expected != 18 || !bytes.Equal(report, want) {
				return nil, fmt.Errorf("wrong custom write request %v, want %v", report, want)
			}
			return []byte{0x66, 1, 18, 0}, nil
		default:
			return nil, fmt.Errorf("unexpected extra request %v", report)
		}
	}}
	client := NewClient(fake)
	settings, err := client.CustomLighting(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if settings.Brightness != 100 || len(settings.Colors) != 2 || settings.Colors[1] != (KeyColor{Key: 124, Red: 4, Green: 5, Blue: 6}) {
		t.Fatalf("unexpected custom lighting settings: %+v", settings)
	}
	if err := client.SetCustomLighting(context.Background(), settings); err != nil {
		t.Fatal(err)
	}
	if fake.calls != 3 {
		t.Fatalf("got %d requests, want 3", fake.calls)
	}
}

func TestCustomLightingWriteChunking(t *testing.T) {
	section := 0
	fake := &fakeExchange{fn: func(report []byte, expected byte) ([]byte, error) {
		if expected != 18 || report[2] != 17 || report[3] != 7 || report[4] != 2 || int(report[5]) != section {
			return nil, fmt.Errorf("unexpected custom lighting section %d report %v", section, report)
		}
		if section == 0 && report[1] != 55 {
			return nil, fmt.Errorf("first chunk length = %d, want 55", report[1])
		}
		if section == 1 && report[1] != 31 {
			return nil, fmt.Errorf("second chunk length = %d, want 31", report[1])
		}
		section++
		return []byte{0x66, 1, 18, 0}, nil
	}}
	settings := CustomLightingSettings{Brightness: 75}
	for key := 0; key < 20; key++ {
		settings.Colors = append(settings.Colors, KeyColor{Key: byte(key), Red: byte(key), Green: 2, Blue: 3})
	}
	if err := NewClient(fake).SetCustomLighting(context.Background(), settings); err != nil {
		t.Fatal(err)
	}
	if fake.calls != 2 || section != 2 {
		t.Fatalf("wrote %d chunks, want 2", fake.calls)
	}
}

func TestLightingUsesWriteOnlyTransportWhenAvailable(t *testing.T) {
	fake := &fakeSender{}
	client := NewClient(fake)
	if err := client.SetLighting(context.Background(), LightingSettings{Effect: 13, Speed: 50, Brightness: 100}); err != nil {
		t.Fatal(err)
	}
	if fake.exchangeCalls != 0 || fake.sendCalls != 1 {
		t.Fatalf("exchange calls=%d send calls=%d, want 0 and 1", fake.exchangeCalls, fake.sendCalls)
	}
}

type fakeSender struct {
	exchangeCalls int
	sendCalls     int
}

func (f *fakeSender) Exchange(context.Context, []byte, byte) ([]byte, error) {
	f.exchangeCalls++
	return nil, errors.New("Exchange should not be called")
}

func (f *fakeSender) Send(_ context.Context, report []byte) error {
	f.sendCalls++
	if len(report) < 4 || report[2] != 7 {
		return fmt.Errorf("unexpected lighting report %v", report)
	}
	return nil
}

type fakeExchange struct {
	calls int
	fn    func([]byte, byte) ([]byte, error)
}

func (f *fakeExchange) Exchange(_ context.Context, report []byte, expected byte) ([]byte, error) {
	f.calls++
	return f.fn(report, expected)
}
