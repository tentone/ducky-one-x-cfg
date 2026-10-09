package ui

import (
	"context"
	"errors"
	"image/color"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	fyneTest "fyne.io/fyne/v2/test"
	"github.com/tentone/ducky-drv/internal/i18n"
	"github.com/tentone/ducky-drv/internal/protocol"
)

// Fyne's standard test driver runs Do inline. Queue background callbacks so
// tests can exercise real frame writes while keeping UI state on one thread.
type gifQueuedDriver struct {
	fyne.Driver
	events chan func()
}

func (d *gifQueuedDriver) DoFromGoroutine(fn func(), _ bool) { d.events <- fn }

type gifQueuedApp struct {
	fyne.App
	driver fyne.Driver
}

func (a *gifQueuedApp) Driver() fyne.Driver { return a.driver }

func newGIFTestUI(t *testing.T) (*UI, *gifQueuedDriver) {
	t.Helper()
	base := fyneTest.NewApp()
	driver := &gifQueuedDriver{Driver: base.Driver(), events: make(chan func(), 100)}
	app := &gifQueuedApp{App: base, driver: driver}
	fyne.SetCurrentApp(app)
	u := &UI{app: app, window: app.NewWindow("GIF test"), i18n: i18n.New(), done: make(chan struct{})}
	u.build()
	u.lighting.animator.Stop()
	u.lighting.effect.SetSelected(effectName(protocol.CustomStaticLightingEffect, u.i18n.T))
	u.window.SetContent(u.content())
	u.window.Hide()
	t.Cleanup(func() {
		u.stopLightingGIF()
		u.cancelAutoSync()
		u.lighting.animator.Stop()
		close(u.done)
		if u.lightingFrameCancel != nil {
			u.lightingFrameCancel()
		}
		if err := waitLightingFrame(context.Background(), u.lightingFrameDone); err != nil {
			t.Error(err)
		}
		base.Quit()
	})
	return u, driver
}

func (d *gifQueuedDriver) next(t *testing.T) {
	t.Helper()
	select {
	case fn := <-d.events:
		fn()
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for playback callback")
	}
}

type gifFrameRecorder struct {
	frames  chan []byte
	started chan struct{}
	release <-chan struct{}
	raw     []byte
	err     error
}

func (r *gifFrameRecorder) Exchange(ctx context.Context, report []byte, command byte) ([]byte, error) {
	if r.err != nil {
		return nil, r.err
	}
	if report[2] != 17 {
		return nil, errors.New("unexpected command during GIF playback")
	}
	if report[5] == 0 {
		r.raw = nil
		if r.started != nil {
			r.started <- struct{}{}
		}
		if r.release != nil {
			select {
			case <-r.release:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}
	r.raw = append(r.raw, report[6:len(report)-2]...)
	if report[5]+1 == report[4] {
		r.frames <- append([]byte(nil), r.raw...)
	}
	return []byte{0x66, 1, command, 1, 0x0d, 0x0a}, nil
}

func testGIFAnimation(loopCount int, colors ...color.NRGBA) *lightingImage {
	animation := &lightingImage{loopCount: loopCount}
	for _, c := range colors {
		frame := lightingImageFrame{colors: make(map[int]color.NRGBA), delay: 10 * time.Millisecond}
		for _, key := range fullSizeKeyboardLayout() {
			frame.colors[key.index] = c
		}
		animation.frames = append(animation.frames, frame)
	}
	return animation
}

func TestLightingGIFUploadsEveryFrameWhileHiddenAndAutoSyncOff(t *testing.T) {
	u, driver := newGIFTestUI(t)
	u.autoSync.SetChecked(false)
	recorder := &gifFrameRecorder{frames: make(chan []byte, 10)}
	u.client = protocol.NewClient(recorder)
	colors := []color.NRGBA{{R: 255, A: 255}, {G: 255, A: 255}, {B: 255, A: 255}}
	u.startLightingGIF(testGIFAnimation(1, colors...)) // two complete passes
	for i := 0; i < 6; i++ {
		var raw []byte
		select {
		case raw = <-recorder.frames:
		case <-time.After(2 * time.Second):
			t.Fatal("frame was not uploaded")
		}
		count := int(raw[5])
		if count != len(fullSizeKeyboardLayout()) || len(raw) != 6+count*4 {
			t.Fatalf("incomplete frame: %d colors in %d bytes", count, len(raw))
		}
		want := colors[i%3]
		wantBrightness := byte(2)
		if i == 0 {
			wantBrightness = 4
		}
		if raw[1] != wantBrightness {
			t.Fatalf("frame %d brightness = %d, want %d", i, raw[1], wantBrightness)
		}
		for j := 0; j < count; j++ {
			offset := 6 + j*4
			if raw[offset+1] != want.R || raw[offset+2] != want.G || raw[offset+3] != want.B {
				t.Fatalf("frame %d contains wrong key colors", i)
			}
		}
		// Upload completion advances the frame, then its timer starts the next.
		driver.next(t)
		if i == 0 {
			u.lighting.brightness.SetValue(50)
		}
		if i < 5 {
			driver.next(t)
		}
	}
	if u.lightingGIF != nil || u.lighting.stopGIF.Visible() {
		t.Fatal("finite GIF did not stop after its repeats")
	}
	if got := u.lighting.keyColors[0]; got != colors[2] {
		t.Fatalf("last frame = %v, want blue", got)
	}
}

func TestLightingGIFWaitsForConfigurationWritesAndStopsOnEffectChange(t *testing.T) {
	u, driver := newGIFTestUI(t)
	release := make(chan struct{})
	recorder := &gifFrameRecorder{frames: make(chan []byte, 5), started: make(chan struct{}, 5), release: release}
	u.client = protocol.NewClient(recorder)
	u.startLightingGIF(testGIFAnimation(0, color.NRGBA{R: 255, A: 255}, color.NRGBA{B: 255, A: 255}))
	select {
	case <-recorder.started:
	case <-time.After(2 * time.Second):
		t.Fatal("frame upload did not start")
	}
	workStarted := make(chan struct{})
	u.run(func(ctx context.Context) (func(), error) {
		close(workStarted)
		return nil, nil
	})
	select {
	case <-workStarted:
		t.Fatal("configuration work interleaved with an incomplete frame")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	select {
	case <-workStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("configuration write did not resume")
	}
	// Drain upload completion and normal operation completion, in either order.
	driver.next(t)
	driver.next(t)
	if u.busy {
		t.Fatal("configuration work stayed busy")
	}
	u.lighting.effect.SetSelected(effectName(13, u.i18n.T))
	if u.lightingGIF != nil {
		t.Fatal("changing effects did not stop playback")
	}
	select {
	case <-recorder.frames:
	default:
		t.Fatal("first frame was not completed")
	}
	select {
	case <-recorder.frames:
		t.Fatal("another frame uploaded after playback stopped")
	case <-time.After(30 * time.Millisecond):
	}
}

func TestLightingGIFStopsOnUploadError(t *testing.T) {
	u, driver := newGIFTestUI(t)
	u.client = protocol.NewClient(&gifFrameRecorder{err: errors.New("keyboard disconnected")})
	u.startLightingGIF(testGIFAnimation(0, color.NRGBA{R: 255, A: 255}, color.NRGBA{B: 255, A: 255}))
	driver.next(t)
	if u.lightingGIF != nil || u.lighting.stopGIF.Visible() {
		t.Fatal("failed playback remained active")
	}
	if u.detail.Text == "" {
		t.Fatal("upload failure was not reported")
	}
}

func TestLightingGIFPaintingStopsPlayback(t *testing.T) {
	u, _ := newGIFTestUI(t)
	animation := testGIFAnimation(0, color.NRGBA{R: 255, A: 255}, color.NRGBA{B: 255, A: 255})
	animation.frames[0].delay = time.Hour
	u.startLightingGIF(animation)
	u.lighting.keyboard.onTapped(17)
	if u.lightingGIF != nil {
		t.Fatal("painting did not stop playback")
	}
	if got := u.lighting.keyColors[17]; got.R != u.lighting.red || got.G != u.lighting.green || got.B != u.lighting.blue {
		t.Fatalf("paint color was overwritten: %v", got)
	}
}

func TestLightingGIFKeepsPlayingAcrossLanguageChange(t *testing.T) {
	u, _ := newGIFTestUI(t)
	red, blue := color.NRGBA{R: 255, A: 255}, color.NRGBA{B: 255, A: 255}
	animation := testGIFAnimation(0, red, blue)
	for i := range animation.frames {
		animation.frames[i].delay = time.Hour
	}
	u.startLightingGIF(animation)
	if got := u.lighting.keyColors[0]; got != red {
		t.Fatalf("first frame = %v, want red", got)
	}
	u.lighting.gifSpeed.SetValue(2)
	u.changeLanguage(i18n.NativeName(i18n.French))
	u.lighting.animator.Stop()
	if u.lightingGIF == nil || !u.lighting.stopGIF.Visible() || !u.lighting.imageRow.Visible() {
		t.Fatal("language change lost active GIF controls")
	}
	if got := u.lighting.keyColors[0]; got != blue {
		t.Fatalf("resumed frame = %v, want blue", got)
	}
	if u.lightingGIF.repeats != 1 {
		t.Fatal("language change reset the playback position")
	}
	if u.lighting.gifSpeed.Value != 2 || !u.lighting.gifSpeedRow.Visible() {
		t.Fatal("language change lost playback speed")
	}
}

func TestLightingGIFSpeedAdjustsFrameTiming(t *testing.T) {
	for _, tc := range []struct {
		speed         float64
		elapsed, want time.Duration
	}{
		{0.25, 0, 400 * time.Millisecond},
		{1, 0, 100 * time.Millisecond},
		{2, 20 * time.Millisecond, 30 * time.Millisecond},
		{4, 0, 25 * time.Millisecond},
		{4, 50 * time.Millisecond, 0}, // USB uploads can exceed the chosen frame delay.
	} {
		if got := lightingGIFFrameWait(100*time.Millisecond, tc.speed, tc.elapsed); got != tc.want {
			t.Errorf("speed %v elapsed %v: %v, want %v", tc.speed, tc.elapsed, got, tc.want)
		}
	}
}

func TestLightingGIFSpeedReschedulesWaitingFrame(t *testing.T) {
	u, _ := newGIFTestUI(t)
	animation := testGIFAnimation(0, color.NRGBA{R: 255, A: 255}, color.NRGBA{B: 255, A: 255})
	for i := range animation.frames {
		animation.frames[i].delay = time.Hour
	}
	u.startLightingGIF(animation)
	playback := u.lightingGIF
	oldTimer, revision := playback.timer, playback.timerRevision
	u.lighting.gifSpeed.SetValue(0.5)
	if playback.timer == oldTimer || playback.timerRevision <= revision {
		t.Fatal("speed change did not reschedule the waiting frame")
	}
	if playback.frame != 1 || u.lighting.keyColors[0].R != 255 {
		t.Fatal("speed change restarted or advanced playback")
	}
	u.stopLightingGIF()
	if u.lighting.gifSpeedRow.Visible() {
		t.Fatal("playback speed stayed visible after stopping")
	}
}
