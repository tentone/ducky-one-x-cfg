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
	keyboardUnit      float32 = 38
	keyboardKeyHeight float32 = 38
)

type keyboardKey struct {
	widget.BaseWidget
	index    int
	label    string
	overlay  string
	width    float32
	fill     color.Color
	selected bool
	flashing bool
	onTapped func(int)
}

func newKeyboardKey(index int, width float32, onTapped func(int)) *keyboardKey {
	key := &keyboardKey{
		index: index, label: protocol.MatrixKeyLabel(index), width: width,
		fill: theme.InputBackgroundColor(), onTapped: onTapped,
	}
	key.ExtendBaseWidget(key)
	return key
}

func (k *keyboardKey) CreateRenderer() fyne.WidgetRenderer {
	background := canvas.NewRectangle(k.fill)
	background.CornerRadius = 4
	background.StrokeWidth = 1
	label := canvas.NewText(k.label, theme.ForegroundColor())
	label.Alignment = fyne.TextAlignCenter
	label.TextSize = 10
	overlay := canvas.NewText(k.overlay, theme.ForegroundColor())
	overlay.Alignment = fyne.TextAlignCenter
	overlay.TextSize = 8
	overlay.TextStyle = fyne.TextStyle{Bold: true}
	return &keyboardKeyRenderer{key: k, background: background, label: label, overlay: overlay}
}

func (k *keyboardKey) Tapped(*fyne.PointEvent) {
	k.flashing = true
	k.Refresh()
	if k.onTapped != nil {
		k.onTapped(k.index)
	}
	time.AfterFunc(180*time.Millisecond, func() {
		fyne.Do(func() {
			k.flashing = false
			k.Refresh()
		})
	})
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
	label      *canvas.Text
	overlay    *canvas.Text
}

func (r *keyboardKeyRenderer) Layout(size fyne.Size) {
	r.background.Resize(size)
	r.overlay.Move(fyne.NewPos(2, 2))
	r.overlay.Resize(fyne.NewSize(size.Width-4, 12))
	r.label.Move(fyne.NewPos(2, 14))
	r.label.Resize(fyne.NewSize(size.Width-4, size.Height-16))
}

func (r *keyboardKeyRenderer) MinSize() fyne.Size {
	return fyne.NewSize(keyboardUnit*r.key.width, keyboardKeyHeight)
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
	r.label.Refresh()
	r.overlay.Refresh()
}

func (r *keyboardKeyRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.background, r.label, r.overlay}
}

func (r *keyboardKeyRenderer) Destroy() {}

type keySpec struct {
	index int
	width float32
	gap   float32
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
	rows := [][]keySpec{
		appendSpecs(spec(0, 1), gap(0.65), keyRange(2, 5, 1), gap(0.3), keyRange(6, 9, 1), gap(0.3), keyRange(10, 13, 1), gap(0.45), keyRange(14, 16, 1), gap(0.45), keyRange(17, 20, 1)),
		appendSpecs(keyRange(21, 33, 1), spec(34, 2), gap(0.45), keyRange(35, 37, 1), gap(0.45), keyRange(38, 41, 1)),
		appendSpecs(spec(42, 1.5), keyRange(43, 54, 1), spec(55, 1.5), gap(0.45), keyRange(56, 58, 1), gap(0.45), keyRange(59, 62, 1)),
		appendSpecs(spec(63, 1.75), keyRange(64, 74, 1), spec(76, 2.25), gap(3.9), keyRange(80, 82, 1)),
		appendSpecs(spec(84, 2.25), keyRange(86, 95, 1), spec(96, 2.75), gap(1.45), spec(99, 1), gap(1.45), keyRange(101, 104, 1)),
		appendSpecs(keyRange(105, 107, 1.25), spec(110, 6.25), keyRange(114, 117, 1.25), gap(0.45), keyRange(119, 121, 1), gap(0.45), spec(123, 2), spec(124, 1)),
	}
	rowObjects := make([]fyne.CanvasObject, 0, len(rows))
	for _, row := range rows {
		items := make([]fyne.CanvasObject, 0, len(row))
		for _, item := range row {
			if item.index < 0 {
				spacer := canvas.NewRectangle(color.Transparent)
				spacer.SetMinSize(fyne.NewSize(keyboardUnit*item.gap, keyboardKeyHeight))
				items = append(items, spacer)
				continue
			}
			key := newKeyboardKey(item.index, item.width, func(index int) {
				view.Select(index)
				if view.onTapped != nil {
					view.onTapped(index)
				}
			})
			view.keys[item.index] = key
			items = append(items, key)
		}
		rowObjects = append(rowObjects, container.NewHBox(items...))
	}
	view.root = container.NewVBox(rowObjects...)
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

func (v *KeyboardView) ForEach(fn func(index int, key *keyboardKey)) {
	for index, key := range v.keys {
		fn(index, key)
	}
}

func spec(index int, width float32) []keySpec { return []keySpec{{index: index, width: width}} }
func gap(width float32) []keySpec             { return []keySpec{{index: -1, gap: width}} }

func keyRange(first, last int, width float32) []keySpec {
	result := make([]keySpec, 0, last-first+1)
	for index := first; index <= last; index++ {
		result = append(result, keySpec{index: index, width: width})
	}
	return result
}

func appendSpecs(groups ...[]keySpec) []keySpec {
	var result []keySpec
	for _, group := range groups {
		result = append(result, group...)
	}
	return result
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
	done     chan struct{}
	stopOnce sync.Once
	start    time.Time
}

func NewLightingAnimator(keyboard *KeyboardView) *LightingAnimator {
	animator := &LightingAnimator{
		keyboard: keyboard, done: make(chan struct{}), start: time.Now(),
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
			a.mu.RUnlock()
			elapsed := now.Sub(a.start).Seconds()
			fyne.Do(func() { a.draw(settings, elapsed) })
		}
	}
}

func (a *LightingAnimator) draw(settings protocol.LightingSettings, elapsed float64) {
	brightness := float64(settings.Brightness) / 100
	if brightness <= 0 && settings.Effect != 103 {
		brightness = 0.05
	}
	baseHue, saturation, value := rgbToHSV(settings.Red, settings.Green, settings.Blue)
	speed := 0.35 + float64(settings.Speed)/55
	a.keyboard.ForEach(func(index int, key *keyboardKey) {
		keyValue := value * brightness
		hue := baseHue
		switch settings.Effect {
		case 3: // Breathing
			keyValue *= 0.18 + 0.82*(math.Sin(elapsed*speed*math.Pi)+1)/2
		case 15: // Color cycle
			hue = math.Mod(baseHue+elapsed*speed*35, 360)
			saturation = 1
		case 25: // Reactive
			wave := math.Mod(elapsed*speed*5, 12)
			keyValue *= 0.2 + 0.8*math.Exp(-math.Abs(float64(index%21)-wave))
		case 39: // Ripple
			distance := math.Abs(float64(index%21)-10) + math.Abs(float64(index/21)-2.5)*2
			wave := math.Mod(elapsed*speed*7, 16)
			keyValue *= 0.15 + 0.85*math.Exp(-math.Abs(distance-wave)*0.8)
		case 21: // Rainbow
			hue = math.Mod(float64(index)*7+elapsed*speed*55, 360)
			saturation = 1
			keyValue = brightness
		case 49: // Analog reactive
			hue = math.Mod(baseHue+float64(index%21)*4+elapsed*speed*18, 360)
			keyValue *= 0.35 + 0.65*(math.Sin(elapsed*speed*2+float64(index)*0.18)+1)/2
		case 103: // Off
			keyValue = 0.035
			saturation = 0
		}
		red, green, blue := hsvToRGB(hue, saturation, math.Min(1, keyValue))
		key.SetColor(color.NRGBA{R: red, G: green, B: blue, A: 255})
	})
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
