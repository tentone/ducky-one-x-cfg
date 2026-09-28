package protocol

import (
	"context"
	"errors"
	"fmt"
)

const (
	packetStart = 0x66
	packetCR    = 0x0d
	packetLF    = 0x0a
)

// Exchanger is implemented by the native HID connection and by protocol tests.
type Exchanger interface {
	Exchange(ctx context.Context, report []byte, expectedCommand byte) ([]byte, error)
}

type Client struct{ device Exchanger }

func NewClient(device Exchanger) *Client { return &Client{device: device} }

func packet(length, command byte, data ...byte) []byte {
	report := make([]byte, 0, len(data)+5)
	report = append(report, packetStart, length, command)
	report = append(report, data...)
	return append(report, packetCR, packetLF)
}

func (c *Client) request(ctx context.Context, expected, length, command byte, data ...byte) ([]byte, error) {
	response, err := c.device.Exchange(ctx, packet(length, command, data...), expected)
	if err != nil {
		return nil, err
	}
	return normalizeResponse(response, expected)
}

func normalizeResponse(response []byte, expected byte) ([]byte, error) {
	if len(response) > 0 && response[0] == 0 {
		response = response[1:]
	}
	if len(response) < 3 || response[0] != packetStart {
		return nil, errors.New("invalid response header")
	}
	if expected != 0 && response[2] != expected {
		return nil, fmt.Errorf("unexpected response command 0x%02x (wanted 0x%02x)", response[2], expected)
	}
	return response, nil
}

func (c *Client) Metadata(ctx context.Context) (DeviceMetadata, error) {
	response, err := c.request(ctx, 2, 2, 1, packetStart)
	if err != nil {
		return DeviceMetadata{}, err
	}
	if len(response) < 9 {
		return DeviceMetadata{}, errors.New("short device information response")
	}
	end := int(response[1]) + 3
	if end > len(response) {
		end = len(response)
	}
	plugin := ""
	if end > 9 {
		plugin = string(response[9:end])
	}
	return DeviceMetadata{
		FirmwareMajor: uint16(response[4])<<8 | uint16(response[5]),
		FirmwareMinor: uint16(response[6])<<8 | uint16(response[7]),
		FirmwarePatch: response[8],
		PluginCode:    plugin,
	}, nil
}

func (c *Client) ActiveProfile(ctx context.Context) (int, error) {
	response, err := c.request(ctx, 155, 1, 154, packetStart)
	if err != nil {
		return 0, err
	}
	if len(response) < 4 {
		return 0, errors.New("short profile response")
	}
	return int(response[3]), nil
}

func (c *Client) SetProfile(ctx context.Context, profile int) error {
	if profile < 0 || profile > 1 {
		return errors.New("profile must be 0 or 1")
	}
	_, err := c.request(ctx, 157, 2, 156, byte(profile), packetStart)
	return err
}

func (c *Client) ActiveLayer(ctx context.Context) (int, error) {
	response, err := c.request(ctx, 20, 2, 19, packetStart)
	if err != nil {
		return 0, err
	}
	if len(response) < 4 {
		return 0, errors.New("short layer response")
	}
	return int(response[3]), nil
}

func (c *Client) SetLayer(ctx context.Context, layer int) error {
	if layer < 0 || layer > 1 {
		return errors.New("layer must be 0 or 1")
	}
	_, err := c.request(ctx, 24, 1, 23, byte(layer))
	return err
}

func (c *Client) KeyMap(ctx context.Context, layer int) ([]Assignment, error) {
	if layer < 0 || layer > 1 {
		return nil, errors.New("layer must be 0 or 1")
	}
	raw := make([]byte, 0, KeyMapEntries*2)
	for section := 0; section < 5; section++ {
		response, err := c.request(ctx, 22, 2, 21, byte(layer), 5, byte(section))
		if err != nil {
			return nil, fmt.Errorf("read key-map section %d: %w", section, err)
		}
		length := 52
		if section == 4 {
			length = 44
		}
		if len(response) < 6+length {
			return nil, fmt.Errorf("short key-map section %d", section)
		}
		raw = append(raw, response[6:6+length]...)
	}
	mapping := make([]Assignment, KeyMapEntries)
	for i := range mapping {
		mapping[i] = Assignment{Kind: AssignmentKind(raw[i*2]), Code: raw[i*2+1]}
	}
	return mapping, nil
}

func (c *Client) SetKeyMap(ctx context.Context, layer int, mapping []Assignment) error {
	if layer < 0 || layer > 1 {
		return errors.New("layer must be 0 or 1")
	}
	if len(mapping) != KeyMapEntries {
		return fmt.Errorf("key map must contain %d entries", KeyMapEntries)
	}
	raw := make([]byte, 0, KeyMapEntries*2)
	for _, assignment := range mapping {
		raw = append(raw, byte(assignment.Kind), assignment.Code)
	}
	for section := 0; section < 5; section++ {
		start := section * 52
		end := start + 52
		length := byte(52)
		if end > len(raw) {
			end = len(raw)
			length = byte(end - start)
		}
		data := []byte{byte(layer), 5, byte(section)}
		data = append(data, raw[start:end]...)
		if _, err := c.request(ctx, 28, length, 27, data...); err != nil {
			return fmt.Errorf("write key-map section %d: %w", section, err)
		}
	}
	return nil
}

func (c *Client) ResetKeyMap(ctx context.Context, layer int) error {
	if layer < 0 || layer > 1 {
		return errors.New("layer must be 0 or 1")
	}
	_, err := c.request(ctx, 26, 1, 25, byte(layer))
	return err
}
