package protocol

import (
	"context"
	"errors"
	"fmt"
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
