package protocol

import (
	"context"
	"errors"
	"fmt"
	"sort"
)

func (c *Client) Lighting(ctx context.Context) (LightingSettings, error) {
	response, err := c.request(ctx, 6, 2, 5, packetStart, 0)
	if err != nil {
		return LightingSettings{}, err
	}
	if len(response) < 12 {
		return LightingSettings{}, errors.New("short lighting response")
	}
	return LightingSettings{
		Effect: response[3], Speed: decodeSpeed(response[4]),
		Red: response[5], Green: response[6], Blue: response[7],
		Brightness: decodeBrightness(response[8]), Variant: response[9], ColorMode: response[11],
	}, nil
}

func (c *Client) SetLighting(ctx context.Context, settings LightingSettings) error {
	if settings.Effect == 21 {
		return c.send(ctx, 8, 9, 7,
			settings.Effect, encodeSpeed(settings.Speed), settings.Red, settings.Green, settings.Blue,
			encodeBrightness(settings.Brightness), settings.Variant, 2, settings.ColorMode, packetStart,
		)
	}
	return c.send(ctx, 8, 7, 7,
		settings.Effect, encodeSpeed(settings.Speed), settings.Red, settings.Green, settings.Blue,
		encodeBrightness(settings.Brightness), settings.Variant, packetStart,
	)
}

func (c *Client) CustomLighting(ctx context.Context) (CustomLightingSettings, error) {
	modeResponse, err := c.request(ctx, 13, 2, 12, packetStart)
	if err != nil {
		return CustomLightingSettings{}, err
	}
	if len(modeResponse) < 4 {
		return CustomLightingSettings{}, errors.New("short custom lighting mode response")
	}
	const customStaticMode = byte(7)
	if modeResponse[3] != customStaticMode {
		return CustomLightingSettings{}, fmt.Errorf("custom lighting mode %d is not per-key static mode", modeResponse[3])
	}

	raw := make([]byte, 0, MaxMatrixKeys*4+6)
	for section := 0; section < 16; section++ {
		response, err := c.request(ctx, 16, 2, 15, customStaticMode, 2, byte(section))
		if err != nil {
			return CustomLightingSettings{}, fmt.Errorf("read custom lighting section %d: %w", section, err)
		}
		if len(response) < 6 {
			return CustomLightingSettings{}, fmt.Errorf("short custom lighting section %d", section)
		}
		length := int(response[1])
		if len(response) < 6+length {
			return CustomLightingSettings{}, fmt.Errorf("short custom lighting payload in section %d", section)
		}
		raw = append(raw, response[6:6+length]...)
		total := int(response[4])
		current := int(response[5])
		if total <= 0 || current+1 >= total {
			break
		}
	}
	if len(raw) < 6 || raw[0] != 85 {
		return CustomLightingSettings{}, errors.New("invalid custom lighting payload")
	}
	count := int(raw[5])
	if count > MaxMatrixKeys || len(raw) < 6+count*4 {
		return CustomLightingSettings{}, errors.New("short custom lighting color data")
	}
	settings := CustomLightingSettings{Brightness: decodeCustomBrightness(raw[1]), Colors: make([]KeyColor, 0, count)}
	for i := 0; i < count; i++ {
		offset := 6 + i*4
		if int(raw[offset]) >= MaxMatrixKeys {
			continue
		}
		settings.Colors = append(settings.Colors, KeyColor{
			Key: raw[offset], Red: raw[offset+1], Green: raw[offset+2], Blue: raw[offset+3],
		})
	}
	return settings, nil
}

func (c *Client) SetCustomLighting(ctx context.Context, settings CustomLightingSettings) error {
	colorsByKey := make(map[byte]KeyColor, len(settings.Colors))
	for _, keyColor := range settings.Colors {
		if int(keyColor.Key) >= MaxMatrixKeys {
			return fmt.Errorf("custom lighting key %d is outside the keyboard matrix", keyColor.Key)
		}
		colorsByKey[keyColor.Key] = keyColor
	}
	keys := make([]int, 0, len(colorsByKey))
	for key := range colorsByKey {
		keys = append(keys, int(key))
	}
	sort.Ints(keys)
	raw := make([]byte, 0, len(keys)*4)
	for _, key := range keys {
		keyColor := colorsByKey[byte(key)]
		raw = append(raw, keyColor.Key, keyColor.Red, keyColor.Green, keyColor.Blue)
	}

	const (
		customStaticMode = byte(7)
		firstDataBytes   = 49
		continuationSize = 55
	)
	totalBytes := len(raw) + 6
	totalSections := (totalBytes + continuationSize - 1) / continuationSize
	if totalSections < 1 {
		totalSections = 1
	}
	offset := 0
	for section := 0; section < totalSections; section++ {
		var data []byte
		if section == 0 {
			end := min(len(raw), firstDataBytes)
			data = []byte{85, encodeCustomBrightness(settings.Brightness), 0, 0, 0, byte(len(keys))}
			data = append(data, raw[:end]...)
			offset = end
		} else {
			end := min(len(raw), offset+continuationSize)
			data = append(data, raw[offset:end]...)
			offset = end
		}
		packetData := []byte{customStaticMode, byte(totalSections), byte(section)}
		packetData = append(packetData, data...)
		if _, err := c.request(ctx, 18, byte(len(data)), 17, packetData...); err != nil {
			return fmt.Errorf("write custom lighting section %d: %w", section, err)
		}
	}
	return nil
}

func (c *Client) Actuation(ctx context.Context) ([]ActuationSetting, error) {
	raw := make([]byte, 0, MaxMatrixKeys*3)
	for section := 0; section < 7; section++ {
		response, err := c.request(ctx, 42, 2, 41, 7, byte(section))
		if err != nil {
			return nil, fmt.Errorf("read actuation section %d: %w", section, err)
		}
		length := int(response[1])
		if length > 54 {
			length = 54
		}
		if len(response) < 5+length {
			return nil, fmt.Errorf("short actuation section %d", section)
		}
		raw = append(raw, response[5:5+length]...)
	}
	if len(raw) < MaxMatrixKeys*3 {
		return nil, fmt.Errorf("short actuation data: got %d bytes", len(raw))
	}
	settings := make([]ActuationSetting, MaxMatrixKeys)
	for i := range settings {
		settings[i] = ActuationSetting{
			Mode: raw[i*3], ActuationMM: DistanceMM(raw[i*3+1]), ReleaseMM: DistanceMM(raw[i*3+2]),
		}
	}
	return settings, nil
}

func (c *Client) SetActuation(ctx context.Context, settings []ActuationSetting) error {
	if len(settings) != MaxMatrixKeys {
		return fmt.Errorf("actuation data must contain %d entries", MaxMatrixKeys)
	}
	raw := make([]byte, 0, MaxMatrixKeys*3)
	for _, setting := range settings {
		raw = append(raw, setting.Mode, DistanceCode(setting.ActuationMM), DistanceCode(setting.ReleaseMM))
	}
	for section := 0; section < 7; section++ {
		start := section * 54
		data := []byte{7, byte(section)}
		data = append(data, raw[start:start+54]...)
		if _, err := c.request(ctx, 44, 54, 43, data...); err != nil {
			return fmt.Errorf("write actuation section %d: %w", section, err)
		}
	}
	return nil
}

func (c *Client) SetAllActuation(ctx context.Context, setting ActuationSetting) error {
	mode := byte(0)
	if setting.RapidTrigger() {
		mode = 1
	}
	_, err := c.request(ctx, 46, 3, 45, mode, DistanceCode(setting.ActuationMM), DistanceCode(setting.ReleaseMM))
	return err
}

func (c *Client) ResetActuation(ctx context.Context) error {
	_, err := c.request(ctx, 46, 3, 45, 1, 0, 0)
	return err
}

func (c *Client) MPT(ctx context.Context, preset int) ([]MPTStage, error) {
	if preset < 0 || preset >= MPTPresets {
		return nil, fmt.Errorf("MPT preset must be between 0 and %d", MPTPresets-1)
	}
	response, err := c.request(ctx, 48, 1, 47, byte(preset))
	if err != nil {
		return nil, err
	}
	if len(response) < 24 {
		return nil, errors.New("short MPT response")
	}
	stages := make([]MPTStage, 4)
	for i := range stages {
		offset := 4 + i*5
		stages[i] = MPTStage{
			PressMM: DistanceMM(response[offset]), ReleaseMM: DistanceMM(response[offset+1]),
			Mouse: response[offset+2] == 1, Output: response[offset+3],
		}
	}
	return stages, nil
}

func (c *Client) SetMPT(ctx context.Context, preset int, stages []MPTStage) error {
	if preset < 0 || preset >= MPTPresets {
		return fmt.Errorf("MPT preset must be between 0 and %d", MPTPresets-1)
	}
	if len(stages) != 4 {
		return errors.New("an MPT preset must contain four stages")
	}
	data := []byte{byte(preset)}
	for _, stage := range stages {
		mouse := byte(0)
		if stage.Mouse || (stage.Output >= 244 && stage.Output <= 246) {
			mouse = 1
		}
		data = append(data, DistanceCode(stage.PressMM), DistanceCode(stage.ReleaseMM), mouse, stage.Output, 0)
	}
	_, err := c.request(ctx, 50, 20, 49, data...)
	return err
}

func (c *Client) ResetMPT(ctx context.Context) error {
	_, err := c.request(ctx, 52, 1, 51, packetStart)
	return err
}
