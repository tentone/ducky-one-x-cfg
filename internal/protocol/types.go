package protocol

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

const (
	VendorID        uint16 = 0x3233
	ConfigUsagePage uint16 = 0x008c
	MaxMatrixKeys          = 126
	KeyMapEntries          = 126
	MacroSlots             = 14
	MPTPresets             = 14
)

type DeviceMetadata struct {
	FirmwareMajor uint16
	FirmwareMinor uint16
	FirmwarePatch byte
	PluginCode    string
}

func (m DeviceMetadata) Firmware() string {
	return fmt.Sprintf("%d.%d.%d", m.FirmwareMajor, m.FirmwareMinor, m.FirmwarePatch)
}

type AssignmentKind byte

const (
	AssignmentDefault  AssignmentKind = 0
	AssignmentKeyboard AssignmentKind = 1
	AssignmentMacro    AssignmentKind = 2
	AssignmentMouse    AssignmentKind = 3
	AssignmentMPT      AssignmentKind = 4
)

type Assignment struct {
	Kind AssignmentKind
	Code byte
}

func (a Assignment) String() string {
	switch a.Kind {
	case AssignmentDefault:
		if a.Code == 0xff {
			return "Disabled"
		}
		return "Default"
	case AssignmentKeyboard:
		return KeyName(a.Code)
	case AssignmentMacro:
		return fmt.Sprintf("Macro M%d", int(a.Code))
	case AssignmentMouse:
		return KeyName(a.Code)
	case AssignmentMPT:
		return fmt.Sprintf("MPT%d", int(a.Code)+1)
	default:
		return fmt.Sprintf("Unknown (%d:%d)", a.Kind, a.Code)
	}
}

type ActuationSetting struct {
	Mode        byte
	ActuationMM float64
	ReleaseMM   float64
}

func (s ActuationSetting) RapidTrigger() bool    { return s.Mode == 1 }
func (s ActuationSetting) ControlledByMPT() bool { return s.Mode == 3 }

type LightingSettings struct {
	Effect     byte
	Speed      int
	Red        byte
	Green      byte
	Blue       byte
	Brightness int
	Variant    byte
	ColorMode  byte
}

const CustomStaticLightingEffect byte = 75

type KeyColor struct {
	Key              byte
	Red, Green, Blue byte
}

type CustomLightingSettings struct {
	Brightness int
	Colors     []KeyColor
}

type MPTStage struct {
	PressMM   float64
	ReleaseMM float64
	Output    byte
	Mouse     bool
}

type MacroActionKind byte

const (
	MacroPress   MacroActionKind = 1
	MacroRelease MacroActionKind = 2
	MacroDelay   MacroActionKind = 3
	MacroClick   MacroActionKind = 4
	MacroText    MacroActionKind = 5
)

type MacroAction struct {
	Kind     MacroActionKind
	Keys     []byte
	DelayMS  int
	RandomMS int
	Text     string
}

func (a MacroAction) Description() string {
	switch a.Kind {
	case MacroPress:
		return "Press " + keyNames(a.Keys)
	case MacroRelease:
		return "Release " + keyNames(a.Keys)
	case MacroClick:
		return "Click " + keyNames(a.Keys)
	case MacroDelay:
		if a.RandomMS > 0 {
			return fmt.Sprintf("Delay %d–%d ms", a.DelayMS, a.DelayMS+a.RandomMS)
		}
		return fmt.Sprintf("Delay %d ms", a.DelayMS)
	case MacroText:
		return fmt.Sprintf("Text %q", a.Text)
	default:
		return "Unknown action"
	}
}

func keyNames(keys []byte) string {
	names := make([]string, len(keys))
	for i, key := range keys {
		names[i] = KeyName(key)
	}
	return strings.Join(names, " + ")
}

// DistanceCode translates millimetres to the keyboard's analog position byte.
func DistanceCode(mm float64) byte {
	mm = math.Round(mm*10) / 10
	if mm <= 0 {
		return 0
	}
	if mm <= 0.1 {
		return 13
	}
	if mm <= 0.2 {
		return 14
	}
	if mm > 3.5 {
		mm = 3.5
	}
	return byte(math.Round(mm * 70))
}

// DistanceMM translates the keyboard's analog position byte to millimetres.
func DistanceMM(code byte) float64 {
	switch code {
	case 0:
		return 0
	case 13:
		return 0.1
	case 14:
		return 0.2
	default:
		return math.Round((float64(code)/70)*10) / 10
	}
}

func encodeBrightness(percent int) byte {
	percent = clampStep(percent, 0, 100, 25)
	return byte(5 + percent/25)
}

func decodeBrightness(code byte) int {
	if code < 5 || code > 9 {
		return 0
	}
	return int(code-5) * 25
}

func encodeCustomBrightness(percent int) byte {
	if percent <= 0 {
		return 0
	}
	if percent > 100 {
		percent = 100
	}
	code := int(math.Round(float64(percent) / 25))
	if code < 1 {
		code = 1
	}
	if code > 4 {
		code = 4
	}
	return byte(code)
}

func decodeCustomBrightness(code byte) int {
	if code > 4 {
		code = 4
	}
	return int(code) * 25
}

func encodeSpeed(percent int) byte {
	percent = clampStep(percent, 0, 100, 10)
	if percent <= 10 {
		return 9
	}
	return byte(10 - percent/10)
}

func decodeSpeed(code byte) int {
	if code >= 9 {
		return 10
	}
	return 100 - int(code)*10
}

func clampStep(value, min, max, step int) int {
	if value < min {
		value = min
	}
	if value > max {
		value = max
	}
	return int(math.Round(float64(value)/float64(step))) * step
}

var keyCodes = map[string]byte{
	"A": 4, "B": 5, "C": 6, "D": 7, "E": 8, "F": 9, "G": 10,
	"H": 11, "I": 12, "J": 13, "K": 14, "L": 15, "M": 16, "N": 17,
	"O": 18, "P": 19, "Q": 20, "R": 21, "S": 22, "T": 23, "U": 24,
	"V": 25, "W": 26, "X": 27, "Y": 28, "Z": 29,
	"1": 30, "2": 31, "3": 32, "4": 33, "5": 34, "6": 35, "7": 36,
	"8": 37, "9": 38, "0": 39,
	"Enter": 40, "Escape": 41, "Backspace": 42, "Tab": 43, "Space": 44,
	"-": 45, "=": 46, "[": 47, "]": 48, "\\": 49, ";": 51, "'": 52,
	"`": 53, ",": 54, ".": 55, "/": 56, "Caps Lock": 57,
	"F1": 58, "F2": 59, "F3": 60, "F4": 61, "F5": 62, "F6": 63,
	"F7": 64, "F8": 65, "F9": 66, "F10": 67, "F11": 68, "F12": 69,
	"Print Screen": 70, "Scroll Lock": 71, "Pause": 72, "Insert": 73, "Home": 74,
	"Page Up": 75, "Delete": 76, "End": 77, "Page Down": 78, "Right Arrow": 79,
	"Left Arrow": 80, "Down Arrow": 81, "Up Arrow": 82, "Num Lock": 83,
	"Numpad /": 84, "Numpad *": 85, "Numpad -": 86, "Numpad +": 87,
	"Numpad Enter": 88, "Numpad 1": 89, "Numpad 2": 90, "Numpad 3": 91,
	"Numpad 4": 92, "Numpad 5": 93, "Numpad 6": 94, "Numpad 7": 95,
	"Numpad 8": 96, "Numpad 9": 97, "Numpad 0": 98, "Numpad .": 99,
	"Application": 101, "Left Ctrl": 224, "Left Shift": 225, "Left Alt": 226,
	"Left Windows": 227, "Right Ctrl": 228, "Right Shift": 229, "Right Alt": 230,
	"Right Windows": 231, "Mouse Left": 244, "Mouse Right": 245, "Mouse Middle": 246,
}

var namesByCode map[byte]string

func init() {
	namesByCode = make(map[byte]string, len(keyCodes))
	for name, code := range keyCodes {
		if _, exists := namesByCode[code]; !exists {
			namesByCode[code] = name
		}
	}
}

func KeyCode(name string) (byte, bool) { code, ok := keyCodes[name]; return code, ok }

func KeyName(code byte) string {
	if name, ok := namesByCode[code]; ok {
		return name
	}
	return fmt.Sprintf("Code %d", code)
}

func KeyOptions(includeMouse bool) []string {
	options := make([]string, 0, len(keyCodes))
	for name, code := range keyCodes {
		if !includeMouse && code >= 244 && code <= 246 {
			continue
		}
		options = append(options, name)
	}
	sort.Slice(options, func(i, j int) bool {
		ai, aok := numericKeyOption(options[i])
		aj, bok := numericKeyOption(options[j])
		if aok && bok {
			return ai < aj
		}
		if aok != bok {
			return aok
		}
		return options[i] < options[j]
	})
	return options
}

func numericKeyOption(value string) (int, bool) {
	n, err := strconv.Atoi(value)
	return n, err == nil
}

var matrixLabels = [MaxMatrixKeys]string{
	"Esc", "—", "F1", "F2", "F3", "F4", "F5", "F6", "F7", "F8", "F9", "F10", "F11", "F12", "PrtSc", "Scroll", "Pause", "Mail", "Media", "Calc", "Computer",
	"`", "1", "2", "3", "4", "5", "6", "7", "8", "9", "0", "-", "=", "Backspace", "Insert", "Home", "PgUp", "Num", "Num /", "Num *", "Num -",
	"Tab", "Q", "W", "E", "R", "T", "Y", "U", "I", "O", "P", "[", "]", "\\", "Delete", "End", "PgDn", "Num 7", "Num 8", "Num 9", "Num +",
	"Caps Lock", "A", "S", "D", "F", "G", "H", "J", "K", "L", ";", "'", "—", "Enter", "—", "—", "—", "Num 4", "Num 5", "Num 6", "—",
	"Left Shift", "—", "Z", "X", "C", "V", "B", "N", "M", ",", ".", "/", "Right Shift", "—", "—", "Up", "—", "Num 1", "Num 2", "Num 3", "Num Enter",
	"Left Ctrl", "Left Win", "Left Alt", "—", "—", "Space", "—", "—", "—", "Right Alt", "Right Win", "Fn", "Right Ctrl", "—", "Left", "Down", "Right", "—", "Num 0", "Num .", "—",
}

func MatrixKeyLabel(index int) string {
	if index < 0 || index >= len(matrixLabels) {
		return fmt.Sprintf("Key %d", index)
	}
	return matrixLabels[index]
}

func MatrixKeyOptions() []string {
	options := make([]string, 0, MaxMatrixKeys)
	for i, label := range matrixLabels {
		if label == "—" {
			continue
		}
		options = append(options, fmt.Sprintf("%03d · %s", i, label))
	}
	return options
}
