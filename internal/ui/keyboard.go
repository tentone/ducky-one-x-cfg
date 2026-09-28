package ui

import (
	"image/color"
	"math"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/joseferrao/ducky-drv/internal/protocol"
)

const (
	keyboardPitch float32 = 42
	keyboardGap   float32 = 4
)

type keyboardKey struct {
	widget.BaseWidget
	index      int
	label      string
	overlay    string
	width      float32
	height     float32
	y          float32
	fill       color.Color
	depth      float64
	depthFill  color.Color
	selected   bool
	flashing   bool
	flashOnTap bool
	onTapped   func(int)
}

func newKeyboardKey(index int, width, height float32, onTapped func(int)) *keyboardKey {
	key := &keyboardKey{
		index: index, label: protocol.MatrixKeyLabel(index), width: width, height: height,
		fill: theme.InputBackgroundColor(), onTapped: onTapped, flashOnTap: true,
	}
	key.ExtendBaseWidget(key)
	return key
}

func (k *keyboardKey) CreateRenderer() fyne.WidgetRenderer {
	background := canvas.NewRectangle(k.fill)
	background.CornerRadius = 4
	background.StrokeWidth = 1
	depthFill := canvas.NewRectangle(color.Transparent)
	label := canvas.NewText(k.label, theme.ForegroundColor())
	label.Alignment = fyne.TextAlignCenter
	label.TextSize = 10
	overlay := canvas.NewText(k.overlay, theme.ForegroundColor())
	overlay.Alignment = fyne.TextAlignCenter
	overlay.TextSize = 8
	overlay.TextStyle = fyne.TextStyle{Bold: true}
	return &keyboardKeyRenderer{key: k, background: background, depthFill: depthFill, label: label, overlay: overlay}
}

func (k *keyboardKey) Tapped(*fyne.PointEvent) {
	if k.flashOnTap {
		k.flashing = true
		k.Refresh()
	}
	if k.onTapped != nil {
		k.onTapped(k.index)
	}
	if !k.flashOnTap {
		return
	}
	time.AfterFunc(180*time.Millisecond, func() {
		fyne.Do(func() {
			k.flashing = false
			k.Refresh()
		})
	})
}

func (k *keyboardKey) SetDepth(level float64, fill color.Color) {
	if level < 0 {
		level = 0
	}
	if level > 1 {
		level = 1
	}
	k.depth = level
	k.depthFill = fill
	k.Refresh()
}

func (k *keyboardKey) SetOverlay(value string) {
	k.overlay = value
	k.Refresh()
}

func (k *keyboardKey) SetColor(value color.Color) {
	k.fill = value
	k.Refresh()
}

func (k *keyboardKey) SetSelected(selected bool) {
	k.selected = selected
	k.Refresh()
}

type keyboardKeyRenderer struct {
	key        *keyboardKey
	background *canvas.Rectangle
	depthFill  *canvas.Rectangle
	label      *canvas.Text
	overlay    *canvas.Text
}

func (r *keyboardKeyRenderer) Layout(size fyne.Size) {
	r.background.Resize(size)
	depthHeight := size.Height * float32(r.key.depth)
	r.depthFill.Move(fyne.NewPos(0, size.Height-depthHeight))
	r.depthFill.Resize(fyne.NewSize(size.Width, depthHeight))
	r.overlay.Move(fyne.NewPos(2, 2))
	r.overlay.Resize(fyne.NewSize(size.Width-4, 12))
	r.label.Move(fyne.NewPos(2, 14))
	r.label.Resize(fyne.NewSize(size.Width-4, size.Height-16))
}

func (r *keyboardKeyRenderer) MinSize() fyne.Size {
	return fyne.NewSize(r.key.width*keyboardPitch-keyboardGap, r.key.height*keyboardPitch-keyboardGap)
}

func (r *keyboardKeyRenderer) Refresh() {
	fill := r.key.fill
	if fill == nil {
		fill = theme.InputBackgroundColor()
	}
	if r.key.flashing {
		fill = blendColor(fill, theme.PrimaryColor(), 0.72)
	}
	r.background.FillColor = fill
	if r.key.depth > 0 && r.key.depthFill != nil {
		r.depthFill.FillColor = r.key.depthFill
		r.depthFill.Show()
	} else {
		r.depthFill.Hide()
	}
	if r.key.selected {
		r.background.StrokeColor = theme.PrimaryColor()
		r.background.StrokeWidth = 3
	} else {
		r.background.StrokeColor = theme.SeparatorColor()
		r.background.StrokeWidth = 1
	}
	r.label.Text = r.key.label
	r.overlay.Text = r.key.overlay
	r.label.Color = contrastColor(fill)
	r.overlay.Color = contrastColor(fill)
	r.background.Refresh()
	r.depthFill.Refresh()
	r.Layout(r.key.Size())
	r.label.Refresh()
	r.overlay.Refresh()
}

func (r *keyboardKeyRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.background, r.depthFill, r.label, r.overlay}
}

func (r *keyboardKeyRenderer) Destroy() {}

type keyPlacement struct {
	index         int
	x, y          float32
	width, height float32
}

type KeyboardView struct {
	root      *fyne.Container
	keys      map[int]*keyboardKey
	selected  int
	onTapped  func(int)
	baseColor color.Color
}

func NewKeyboardView(onTapped func(int)) *KeyboardView {
	view := &KeyboardView{keys: make(map[int]*keyboardKey), selected: -1, onTapped: onTapped}
	keyboardSize := fyne.NewSize(23*keyboardPitch-keyboardGap, 6*keyboardPitch-keyboardGap)
	background := canvas.NewRectangle(color.Transparent)
	background.SetMinSize(keyboardSize)
	background.Resize(keyboardSize)
	objects := []fyne.CanvasObject{background}
	for _, item := range fullSizeKeyboardLayout() {
		key := newKeyboardKey(item.index, item.width, item.height, func(index int) {
			view.Select(index)
			if view.onTapped != nil {
				view.onTapped(index)
			}
		})
		key.y = item.y
		key.Move(fyne.NewPos(item.x*keyboardPitch, item.y*keyboardPitch))
		key.Resize(fyne.NewSize(item.width*keyboardPitch-keyboardGap, item.height*keyboardPitch-keyboardGap))
		view.keys[item.index] = key
		objects = append(objects, key)
	}
	view.root = container.NewWithoutLayout(objects...)
	view.root.Resize(keyboardSize)
	return view
}

func (v *KeyboardView) CanvasObject() fyne.CanvasObject {
	return container.NewHScroll(v.root)
}

func (v *KeyboardView) Select(index int) {
	if previous, ok := v.keys[v.selected]; ok {
		previous.SetSelected(false)
	}
	v.selected = index
	if key, ok := v.keys[index]; ok {
		key.SetSelected(true)
	}
}

func (v *KeyboardView) SetOverlay(index int, value string) {
	if key, ok := v.keys[index]; ok {
		key.SetOverlay(value)
	}
}

func (v *KeyboardView) SetColor(index int, value color.Color) {
	if key, ok := v.keys[index]; ok {
		key.SetColor(value)
	}
}

func (v *KeyboardView) SetAllColors(value color.Color) {
	for _, key := range v.keys {
		key.SetColor(value)
	}
}

func (v *KeyboardView) SetTapFlash(enabled bool) {
	for _, key := range v.keys {
		key.flashOnTap = enabled
	}
}

func (v *KeyboardView) ForEach(fn func(index int, key *keyboardKey)) {
	for index, key := range v.keys {
		fn(index, key)
	}
}

func fullSizeKeyboardLayout() []keyPlacement {
	var keys []keyPlacement
	add := func(index int, x, y, width, height float32) {
		keys = append(keys, keyPlacement{index: index, x: x, y: y, width: width, height: height})
	}
	addRange := func(first, last int, x, y float32) {
		for index := first; index <= last; index++ {
			add(index, x+float32(index-first), y, 1, 1)
		}
	}

	add(0, 0, 0, 1, 1)
	addRange(2, 5, 2, 0)
	addRange(6, 9, 6.5, 0)
	addRange(10, 13, 11, 0)
	addRange(14, 16, 15.5, 0)
	addRange(17, 20, 19, 0)

	addRange(21, 33, 0, 1)
	add(34, 13, 1, 2, 1)
	addRange(35, 37, 15.5, 1)
	addRange(38, 41, 19, 1)

	add(42, 0, 2, 1.5, 1)
	addRange(43, 54, 1.5, 2)
	add(55, 13.5, 2, 1.5, 1)
	addRange(56, 58, 15.5, 2)
	addRange(59, 61, 19, 2)
	add(62, 22, 2, 1, 2)

	add(63, 0, 3, 1.75, 1)
	addRange(64, 74, 1.75, 3)
	add(76, 12.75, 3, 2.25, 1)
	addRange(80, 82, 19, 3)

	add(84, 0, 4, 2.25, 1)
	addRange(86, 95, 2.25, 4)
	add(96, 12.25, 4, 2.75, 1)
	add(99, 16.5, 4, 1, 1)
	addRange(101, 103, 19, 4)
	add(104, 22, 4, 1, 2)

	add(105, 0, 5, 1.25, 1)
	add(106, 1.25, 5, 1.25, 1)
	add(107, 2.5, 5, 1.25, 1)
	add(110, 3.75, 5, 6.25, 1)
	add(114, 10, 5, 1.25, 1)
	add(115, 11.25, 5, 1.25, 1)
	add(116, 12.5, 5, 1.25, 1)
	add(117, 13.75, 5, 1.25, 1)
	addRange(119, 121, 15.5, 5)
	add(123, 19, 5, 2, 1)
	add(124, 21, 5, 1, 1)
	return keys
}

func blendColor(a, b color.Color, amount float64) color.Color {
	ar, ag, ab, _ := a.RGBA()
	br, bg, bb, _ := b.RGBA()
	mix := func(x, y uint32) uint8 { return uint8((float64(x)*(1-amount) + float64(y)*amount) / 257) }
	return color.NRGBA{R: mix(ar, br), G: mix(ag, bg), B: mix(ab, bb), A: 255}
}

func contrastColor(background color.Color) color.Color {
	r, g, b, _ := background.RGBA()
	luminance := 0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b)
	if luminance > 0.54*65535 {
		return color.NRGBA{R: 24, G: 24, B: 24, A: 255}
	}
	return color.NRGBA{R: 245, G: 245, B: 245, A: 255}
}

type LightingAnimator struct {
	keyboard *KeyboardView
	mu       sync.RWMutex
	settings protocol.LightingSettings
	presses  map[int]time.Time
	done     chan struct{}
	stopOnce sync.Once
	start    time.Time
}

func NewLightingAnimator(keyboard *KeyboardView) *LightingAnimator {
	animator := &LightingAnimator{
		keyboard: keyboard, done: make(chan struct{}), start: time.Now(), presses: make(map[int]time.Time),
		settings: protocol.LightingSettings{Effect: 13, Red: 234, Green: 168, Blue: 42, Brightness: 100, Speed: 50},
	}
	go animator.loop()
	return animator
}

func (a *LightingAnimator) Set(settings protocol.LightingSettings) {
	a.mu.Lock()
	a.settings = settings
	a.mu.Unlock()
}

func (a *LightingAnimator) Press(index int) {
	a.mu.Lock()
	a.presses[index] = time.Now()
	a.mu.Unlock()
}

func (a *LightingAnimator) Stop() { a.stopOnce.Do(func() { close(a.done) }) }

func (a *LightingAnimator) loop() {
	ticker := time.NewTicker(110 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-a.done:
			return
		case now := <-ticker.C:
			a.mu.RLock()
			settings := a.settings
			presses := make(map[int]time.Time, len(a.presses))
			for index, pressedAt := range a.presses {
				presses[index] = pressedAt
			}
			a.mu.RUnlock()
			elapsed := now.Sub(a.start).Seconds()
			fyne.Do(func() { a.draw(settings, elapsed, now, presses) })
		}
	}
}

func (a *LightingAnimator) draw(settings protocol.LightingSettings, elapsed float64, now time.Time, presses map[int]time.Time) {
	brightness := float64(settings.Brightness) / 100
	if brightness <= 0 && settings.Effect != 103 {
		brightness = 0.05
	}
	baseHue, saturation, value := rgbToHSV(settings.Red, settings.Green, settings.Blue)
	speed := 0.35 + float64(settings.Speed)/55
	analogDepth := 0.0
	if settings.Effect == 49 {
		for _, pressedAt := range presses {
			if depth := analogPressDepth(now.Sub(pressedAt).Seconds()); depth > analogDepth {
				analogDepth = depth
			}
		}
	}
	a.keyboard.ForEach(func(index int, key *keyboardKey) {
		key.SetDepth(0, color.Transparent)
		keyValue := value * brightness
		hue := baseHue
		switch settings.Effect {
		case 3: // Breathing
			keyValue *= 0.18 + 0.82*(math.Sin(elapsed*speed*math.Pi)+1)/2
		case 15: // Color cycle
			hue = math.Mod(baseHue+elapsed*speed*35, 360)
			saturation = 1
		case 25: // Reactive
			keyValue = 0.025
			if pressedAt, ok := presses[index]; ok {
				duration := 1.35 / speed
				age := now.Sub(pressedAt).Seconds()
				if age >= 0 && age < duration {
					keyValue = brightness * (1 - age/duration)
				}
			}
		case 39: // Ripple
			distance := math.Abs(float64(index%21)-10) + math.Abs(float64(index/21)-2.5)*2
			wave := math.Mod(elapsed*speed*7, 16)
			keyValue *= 0.15 + 0.85*math.Exp(-math.Abs(distance-wave)*0.8)
		case 21: // Rainbow
			row, column := float64(index/21), float64(index%21)
			position := column
			switch settings.ColorMode {
			case 1: // Left
				position = -column
			case 2: // Down
				position = row * 3
			case 3: // Up
				position = -row * 3
			}
			phase := math.Mod(position*0.075-elapsed*speed*0.28, 1)
			if phase < 0 {
				phase += 1
			}
			switch settings.Variant {
			case 1: // Yellow / red
				hue = 55 * triangleWave(phase)
			case 2: // Blue / red
				hue = 240 * triangleWave(phase)
			default:
				hue = phase * 360
			}
			saturation = 1
			keyValue = brightness
		case 49: // Analog reactive
			keyValue = 0.025
			if analogKeyIsLit(analogDepth, key.y, key.height) {
				keyValue = value * brightness
			}
		case 103: // Off
			keyValue = 0.035
			saturation = 0
		}
		red, green, blue := hsvToRGB(hue, saturation, math.Min(1, keyValue))
		key.SetColor(color.NRGBA{R: red, G: green, B: blue, A: 255})
	})
}

func analogPressDepth(age float64) float64 {
	if age < 0 {
		return 0
	}
	if age < 0.24 {
		return age / 0.24
	}
	if age < 0.9 {
		return 1 - (age-0.24)/0.66
	}
	return 0
}

func analogBandRow(depth float64) int {
	if depth <= 0 {
		return -1
	}
	if depth >= 1 {
		return 0
	}
	return 5 - int(math.Floor(depth*6))
}

func analogKeyIsLit(depth float64, y, height float32) bool {
	row := analogBandRow(depth)
	if row < 0 {
		return false
	}
	rowTop := float32(row)
	rowBottom := rowTop + 1
	return y < rowBottom && y+height > rowTop
}

func triangleWave(value float64) float64 {
	if value < 0.5 {
		return value * 2
	}
	return (1 - value) * 2
}

func rgbToHSV(red, green, blue byte) (float64, float64, float64) {
	r, g, b := float64(red)/255, float64(green)/255, float64(blue)/255
	maxValue := math.Max(r, math.Max(g, b))
	minValue := math.Min(r, math.Min(g, b))
	delta := maxValue - minValue
	hue := 0.0
	if delta != 0 {
		switch maxValue {
		case r:
			hue = 60 * math.Mod((g-b)/delta, 6)
		case g:
			hue = 60 * ((b-r)/delta + 2)
		default:
			hue = 60 * ((r-g)/delta + 4)
		}
	}
	if hue < 0 {
		hue += 360
	}
	saturation := 0.0
	if maxValue != 0 {
		saturation = delta / maxValue
	}
	return hue, saturation, maxValue
}

func hsvToRGB(hue, saturation, value float64) (byte, byte, byte) {
	chroma := value * saturation
	x := chroma * (1 - math.Abs(math.Mod(hue/60, 2)-1))
	m := value - chroma
	var r, g, b float64
	switch {
	case hue < 60:
		r, g = chroma, x
	case hue < 120:
		r, g = x, chroma
	case hue < 180:
		g, b = chroma, x
	case hue < 240:
		g, b = x, chroma
	case hue < 300:
		r, b = x, chroma
	default:
		r, b = chroma, x
	}
	return byte((r + m) * 255), byte((g + m) * 255), byte((b + m) * 255)
}
