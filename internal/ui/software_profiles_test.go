package ui

import (
	"context"
	"testing"

	"github.com/joseferrao/ducky-drv/internal/profiles"
	"github.com/joseferrao/ducky-drv/internal/protocol"
)

type profileWriteRecorder struct {
	commands map[byte]int
}

func (r *profileWriteRecorder) Exchange(_ context.Context, report []byte, expectedCommand byte) ([]byte, error) {
	if len(report) >= 3 {
		r.commands[report[2]]++
	}
	return []byte{0x66, 1, expectedCommand, 1, 0x0d, 0x0a}, nil
}

func TestWriteCompleteConfigurationWritesEveryProfileComponent(t *testing.T) {
	recorder := &profileWriteRecorder{commands: make(map[byte]int)}
	client := protocol.NewClient(recorder)
	configuration := completeTestConfiguration()
	u := &UI{}
	if err := u.writeCompleteConfiguration(context.Background(), client, configuration); err != nil {
		t.Fatal(err)
	}

	want := map[byte]int{
		27: 10, // five key-map packets for each of two layers
		7:  1,  // lighting
		43: 7,  // actuation
		49: protocol.MPTPresets,
		31: protocol.MacroSlots * 8,
		23: 1, // restore the saved active layer
	}
	for command, count := range want {
		if recorder.commands[command] != count {
			t.Errorf("command 0x%02x count = %d, want %d", command, recorder.commands[command], count)
		}
	}
}

func completeTestConfiguration() profiles.Configuration {
	configuration := profiles.Configuration{
		KeyMaps:   make([][]protocol.Assignment, 2),
		Lighting:  protocol.LightingSettings{Effect: 13, Brightness: 100},
		Actuation: make([]protocol.ActuationSetting, protocol.MaxMatrixKeys),
		MPT:       make([][]protocol.MPTStage, protocol.MPTPresets),
		Macros:    make([][]protocol.MacroAction, protocol.MacroSlots),
	}
	for layer := range configuration.KeyMaps {
		configuration.KeyMaps[layer] = make([]protocol.Assignment, protocol.KeyMapEntries)
	}
	for preset := range configuration.MPT {
		configuration.MPT[preset] = make([]protocol.MPTStage, 4)
	}
	return configuration
}
