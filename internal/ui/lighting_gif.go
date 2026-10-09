package ui

import (
	"context"
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"github.com/tentone/ducky-drv/internal/protocol"
)

// Playback state belongs to the Fyne thread, independent of window visibility.
// Only a single complete frame transaction is allowed in flight.
type lightingGIFPlayback struct {
	image         *lightingImage
	frame         int
	repeats       int
	timer         *time.Timer
	generation    uint64
	timerRevision uint64
	waiting       bool
	frameDelay    time.Duration
	frameStarted  time.Time
}

func (u *UI) stopLightingGIF() {
	if playback := u.lightingGIF; playback != nil {
		playback.generation++
		playback.timerRevision++
		if playback.timer != nil {
			playback.timer.Stop()
		}
		u.lightingGIF = nil
	}
	if u.lighting.stopGIF != nil {
		u.lighting.stopGIF.Hide()
	}
	if u.lighting.gifSpeedRow != nil {
		u.lighting.gifSpeedRow.Hide()
	}
}

func (u *UI) startLightingGIF(animation *lightingImage) {
	u.stopLightingGIF()
	u.cancelAutoSyncFeature("lighting")
	u.lightingGIF = &lightingGIFPlayback{image: animation}
	u.lighting.stopGIF.Show()
	u.lighting.gifSpeedRow.Show()
	u.playLightingGIFFrame(u.lightingGIF)
}

func (u *UI) scheduleLightingGIFFrame(playback *lightingGIFPlayback, delay time.Duration) {
	playback.timerRevision++
	revision := playback.timerRevision
	playback.timer = time.AfterFunc(max(delay, 0), func() {
		select {
		case <-u.done:
			return
		default:
		}
		fyne.Do(func() {
			if u.lightingGIF == playback && playback.timerRevision == revision {
				u.playLightingGIFFrame(playback)
			}
		})
	})
}

func (u *UI) playLightingGIFFrame(playback *lightingGIFPlayback) {
	if u.lightingGIF != playback {
		return
	}
	playback.waiting = false
	select {
	case <-u.done:
		u.stopLightingGIF()
		return
	default:
	}
	if effectCode(u.lighting.effect.Selected, u.i18n.T) != protocol.CustomStaticLightingEffect {
		u.stopLightingGIF()
		return
	}
	if u.busy || lightingFramePending(u.lightingFrameDone) {
		u.scheduleLightingGIFFrame(playback, 25*time.Millisecond)
		return
	}
	frame := playback.image.frames[playback.frame]
	clear(u.lighting.keyColors)
	for index, c := range frame.colors {
		u.lighting.keyColors[index] = c
	}
	u.lighting.animator.SetCustomColors(frame.colors)
	settings := lightingSettingsFromControls(&u.lighting, u.i18n.T)
	u.lighting.animator.Set(settings)
	u.lighting.animator.draw(settings, 0, time.Now(), nil, frame.colors)
	started := time.Now()
	if u.client == nil {
		u.finishLightingGIFFrame(playback, frame.delay, started)
		return
	}
	client := u.client
	custom := customLightingFromControls(&u.lighting)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	done := make(chan struct{})
	u.lightingFrameDone = done
	u.lightingFrameCancel = cancel
	generation := playback.generation
	go func() {
		err := client.SetCustomLighting(ctx, custom)
		cancel()
		close(done)
		select {
		case <-u.done:
			return
		default:
		}
		fyne.Do(func() {
			if u.lightingFrameDone == done {
				u.lightingFrameDone = nil
				u.lightingFrameCancel = nil
			}
			if u.lightingGIF != playback || playback.generation != generation {
				return
			}
			if err != nil {
				u.stopLightingGIF()
				message := fmt.Sprintf("%s: %v", u.i18n.T("lighting.gif_error"), err)
				u.detail.SetText(message)
				u.app.SendNotification(fyne.NewNotification(u.i18n.T("tab.lighting"), message))
				return
			}
			u.finishLightingGIFFrame(playback, frame.delay, started)
		})
	}()
}

func (u *UI) finishLightingGIFFrame(playback *lightingGIFPlayback, delay time.Duration, started time.Time) {
	playback.frame++
	if playback.frame == len(playback.image.frames) {
		if playback.image.loopCount < 0 || (playback.image.loopCount > 0 && playback.repeats >= playback.image.loopCount) {
			u.stopLightingGIF()
			return
		}
		playback.frame = 0
		playback.repeats++
	}
	// Slow devices extend playback instead of skipping frames or accumulating writes.
	playback.waiting = true
	playback.frameDelay = delay
	playback.frameStarted = started
	u.scheduleLightingGIFFrame(playback, lightingGIFFrameWait(delay, u.lighting.gifSpeed.Value, time.Since(started)))
}

func lightingGIFFrameWait(delay time.Duration, speed float64, elapsed time.Duration) time.Duration {
	speed = max(0.25, min(4, speed))
	return max(time.Duration(float64(delay)/speed)-elapsed, 0)
}

func (u *UI) changeLightingGIFSpeed() {
	playback := u.lightingGIF
	if playback == nil || !playback.waiting {
		return
	}
	if playback.timer != nil {
		playback.timer.Stop()
	}
	u.scheduleLightingGIFFrame(playback, lightingGIFFrameWait(playback.frameDelay, u.lighting.gifSpeed.Value, time.Since(playback.frameStarted)))
}

func lightingFramePending(done <-chan struct{}) bool {
	if done == nil {
		return false
	}
	select {
	case <-done:
		return false
	default:
		return true
	}
}

func waitLightingFrame(ctx context.Context, done <-chan struct{}) error {
	if done == nil {
		return nil
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
