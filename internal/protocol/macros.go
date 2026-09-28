package protocol

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

const macroBytes = 401

func (c *Client) Macro(ctx context.Context, slot int) ([]MacroAction, error) {
	if slot < 1 || slot > MacroSlots {
		return nil, fmt.Errorf("macro slot must be between 1 and %d", MacroSlots)
	}
	raw := make([]byte, 0, macroBytes)
	for section := 0; section < 8; section++ {
		response, err := c.request(ctx, 30, 2, 29, byte(slot), 8, byte(section))
		if err != nil {
			return nil, fmt.Errorf("read macro section %d: %w", section, err)
		}
		length := 52
		if section == 7 {
			length = 37
		}
		if len(response) < 6+length {
			return nil, fmt.Errorf("short macro section %d", section)
		}
		raw = append(raw, response[6:6+length]...)
	}
	return decodeMacro(raw)
}

func (c *Client) SetMacro(ctx context.Context, slot int, actions []MacroAction) error {
	if slot < 1 || slot > MacroSlots {
		return fmt.Errorf("macro slot must be between 1 and %d", MacroSlots)
	}
	raw, err := encodeMacro(actions)
	if err != nil {
		return err
	}
	for section := 0; section < 8; section++ {
		start := section * 52
		end := start + 52
		length := byte(52)
		if section == 7 {
			end = macroBytes
			length = 37
		}
		data := []byte{byte(slot), 8, byte(section)}
		data = append(data, raw[start:end]...)
		if _, err := c.request(ctx, 32, length, 31, data...); err != nil {
			return fmt.Errorf("write macro section %d: %w", section, err)
		}
	}
	return nil
}

func encodeMacro(actions []MacroAction) ([]byte, error) {
	if len(actions) > 255 {
		return nil, errors.New("macro has too many actions")
	}
	raw := []byte{byte(len(actions))}
	for i, action := range actions {
		switch action.Kind {
		case MacroPress, MacroRelease, MacroClick:
			if len(action.Keys) == 0 || len(action.Keys) > 255 {
				return nil, fmt.Errorf("action %d must contain at least one key", i+1)
			}
			raw = append(raw, byte(action.Kind), byte(len(action.Keys)))
			raw = append(raw, action.Keys...)
		case MacroDelay:
			delay := clamp(action.DelayMS, 1, 999)
			random := clamp(action.RandomMS, 0, 999)
			raw = append(raw, byte(MacroDelay), 4, byte(delay), byte(delay>>8), byte(random), byte(random>>8))
		case MacroText:
			codes, err := textKeyCodes(action.Text)
			if err != nil {
				return nil, fmt.Errorf("action %d: %w", i+1, err)
			}
			if len(codes) > 20 {
				return nil, fmt.Errorf("action %d text is longer than 20 characters", i+1)
			}
			raw = append(raw, byte(MacroText), byte(len(codes)))
			raw = append(raw, codes...)
		default:
			return nil, fmt.Errorf("action %d has an unknown type", i+1)
		}
	}
	if len(raw) > macroBytes {
		return nil, fmt.Errorf("macro uses %d of %d bytes", len(raw), macroBytes)
	}
	return append(raw, make([]byte, macroBytes-len(raw))...), nil
}

func decodeMacro(raw []byte) ([]MacroAction, error) {
	if len(raw) < 1 {
		return nil, errors.New("empty macro data")
	}
	count := int(raw[0])
	actions := make([]MacroAction, 0, count)
	offset := 1
	for i := 0; i < count; i++ {
		if offset+2 > len(raw) {
			return nil, errors.New("truncated macro action")
		}
		kind := MacroActionKind(raw[offset])
		length := int(raw[offset+1])
		offset += 2
		if offset+length > len(raw) {
			return nil, errors.New("truncated macro payload")
		}
		payload := raw[offset : offset+length]
		offset += length
		action := MacroAction{Kind: kind}
		switch kind {
		case MacroPress, MacroRelease, MacroClick:
			action.Keys = append([]byte(nil), payload...)
		case MacroDelay:
			if len(payload) != 4 {
				return nil, errors.New("invalid delay action")
			}
			action.DelayMS = int(payload[0]) | int(payload[1])<<8
			action.RandomMS = int(payload[2]) | int(payload[3])<<8
		case MacroText:
			action.Text = keyCodesText(payload)
		default:
			return nil, fmt.Errorf("unknown macro action type %d", kind)
		}
		actions = append(actions, action)
	}
	return actions, nil
}

func textKeyCodes(text string) ([]byte, error) {
	codes := make([]byte, 0, len(text))
	for _, char := range strings.ToLower(text) {
		code, ok := macroTextCode[char]
		if !ok || code == 0 {
			return nil, fmt.Errorf("character %q is not supported by the keyboard macro format", char)
		}
		codes = append(codes, code)
	}
	return codes, nil
}

func keyCodesText(codes []byte) string {
	var builder strings.Builder
	for _, code := range codes {
		if char, ok := macroTextChar[code]; ok {
			builder.WriteRune(char)
		} else {
			builder.WriteRune('�')
		}
	}
	return builder.String()
}

var macroTextCode = map[rune]byte{
	'a': 4, 'b': 5, 'c': 6, 'd': 7, 'e': 8, 'f': 9, 'g': 10, 'h': 11,
	'i': 12, 'j': 13, 'k': 14, 'l': 15, 'm': 16, 'n': 17, 'o': 18, 'p': 19,
	'q': 20, 'r': 21, 's': 22, 't': 23, 'u': 24, 'v': 25, 'w': 26, 'x': 27,
	'y': 28, 'z': 29, '1': 30, '2': 31, '3': 32, '4': 33, '5': 34, '6': 35,
	'7': 36, '8': 37, '9': 38, '0': 39, ' ': 44, '-': 45, '=': 46, '[': 47,
	']': 48, '\\': 49, ';': 51, '\'': 52, '`': 53, '.': 55, '/': 84, '*': 85,
}

var macroTextChar = func() map[byte]rune {
	result := make(map[byte]rune, len(macroTextCode))
	for char, code := range macroTextCode {
		result[code] = char
	}
	return result
}()

func clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
