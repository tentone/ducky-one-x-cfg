package ui

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/tentone/ducky-drv/internal/protocol"
)

// macroDragItem handles both a click and a drag without moving the source
// widget. The drop position is measured against the scroll viewport.
type macroDragItem struct {
	widget.BaseWidget
	label      *widget.Label
	background *canvas.Rectangle
	onTap      func()
	onDrag     func(fyne.Position, bool)
	last       fyne.Position
	dragging   bool
}

func newMacroDragItem(text string, tap func(), drag func(fyne.Position, bool)) *macroDragItem {
	item := &macroDragItem{label: widget.NewLabel("≡  " + text), background: canvas.NewRectangle(theme.InputBackgroundColor()), onTap: tap, onDrag: drag}
	item.label.Truncation = fyne.TextTruncateEllipsis
	item.ExtendBaseWidget(item)
	return item
}

func (i *macroDragItem) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewStack(i.background, i.label))
}

func (i *macroDragItem) Tapped(*fyne.PointEvent) { i.onTap() }
func (i *macroDragItem) Dragged(e *fyne.DragEvent) {
	i.dragging = true
	i.last = e.AbsolutePosition
	i.onDrag(i.last, false)
}
func (i *macroDragItem) DragEnd() {
	if i.dragging {
		i.onDrag(i.last, true)
	}
	i.dragging = false
}

type macroActionList struct {
	widget.BaseWidget
	controls *macroControls
	t        func(string) string
	onSelect func(int)
	onChange func()
	content  *fyne.Container
	scroll   *container.Scroll
	rows     []*macroDragItem
	markers  []*canvas.Rectangle
}

func newMacroActionList(controls *macroControls, t func(string) string, selectAction func(int), changed func()) *macroActionList {
	l := &macroActionList{controls: controls, t: t, onSelect: selectAction, onChange: changed, content: container.NewVBox()}
	l.scroll = container.NewVScroll(l.content)
	l.scroll.SetMinSize(fyne.NewSize(280, 240))
	l.ExtendBaseWidget(l)
	return l
}

func (l *macroActionList) CreateRenderer() fyne.WidgetRenderer {
	l.rebuild()
	return widget.NewSimpleRenderer(l.scroll)
}

func (l *macroActionList) Refresh() {
	l.rebuild()
	l.BaseWidget.Refresh()
}

func (l *macroActionList) rebuild() {
	l.content.Objects = nil
	l.rows = nil
	l.markers = nil
	for id, action := range l.controls.actions {
		id, kind := id, action.Kind
		row := newMacroDragItem(fmt.Sprintf("%02d  %s", id+1, action.Description()), func() {
			l.onSelect(id)
			l.Refresh()
		}, func(pos fyne.Position, end bool) { l.drag(kind, id, pos, end) })
		if id == l.controls.selected {
			row.background.FillColor = theme.SelectionColor()
		}
		marker := canvas.NewRectangle(color.Transparent)
		marker.SetMinSize(fyne.NewSize(1, 3))
		l.markers = append(l.markers, marker)
		l.rows = append(l.rows, row)
		l.content.Add(container.NewVBox(marker, row))
	}
	marker := canvas.NewRectangle(color.Transparent)
	marker.SetMinSize(fyne.NewSize(1, 3))
	l.markers = append(l.markers, marker)
	l.content.Add(marker)
	hint := widget.NewLabel(l.t("macro.drop"))
	hint.Wrapping = fyne.TextWrapWord
	l.content.Add(hint)
	l.content.Refresh()
}

func (l *macroActionList) UnselectAll()    { l.onSelect(-1); l.Refresh() }
func (l *macroActionList) ScrollToBottom() { l.scroll.ScrollToBottom() }

func (l *macroActionList) dropIndex(pos fyne.Position) (int, bool) {
	driver := fyne.CurrentApp().Driver()
	origin := driver.AbsolutePositionForObject(l.scroll)
	size := l.scroll.Size()
	if pos.X < origin.X || pos.X > origin.X+size.Width || pos.Y < origin.Y || pos.Y > origin.Y+size.Height {
		return 0, false
	}
	for id, row := range l.rows {
		p := driver.AbsolutePositionForObject(row)
		if pos.Y < p.Y+row.Size().Height/2 {
			return id, true
		}
	}
	return len(l.controls.actions), true
}

func (l *macroActionList) drag(kind protocol.MacroActionKind, source int, pos fyne.Position, end bool) {
	index, inside := l.dropIndex(pos)
	if inside && !end {
		origin := fyne.CurrentApp().Driver().AbsolutePositionForObject(l.scroll)
		offset := l.scroll.Offset
		if pos.Y < origin.Y+24 {
			offset.Y -= 16
		}
		if pos.Y > origin.Y+l.scroll.Size().Height-24 {
			offset.Y += 16
		}
		l.scroll.ScrollToOffset(offset)
		index, inside = l.dropIndex(pos)
	}
	for id, marker := range l.markers {
		marker.FillColor = color.Transparent
		if !end && inside && id == index {
			marker.FillColor = theme.PrimaryColor()
		}
		marker.Refresh()
	}
	if !end || !inside {
		return
	}
	if source < 0 {
		l.insert(kind, index)
		return
	}
	if source >= len(l.controls.actions) {
		return
	}
	// The insertion position refers to the list before removing the source.
	if index > source {
		index--
	}
	if index == source {
		return
	}
	action := l.controls.actions[source]
	l.controls.actions = append(l.controls.actions[:source], l.controls.actions[source+1:]...)
	l.controls.actions = append(l.controls.actions, protocol.MacroAction{})
	copy(l.controls.actions[index+1:], l.controls.actions[index:])
	l.controls.actions[index] = action
	l.onSelect(index)
	l.Refresh()
	l.onChange()
}

func (l *macroActionList) insert(kind protocol.MacroActionKind, index int) {
	action := protocol.MacroAction{Kind: kind}
	switch kind {
	case protocol.MacroDelay:
		action.DelayMS = 100
	case protocol.MacroText:
		action.Text = "text"
	default:
		action.Keys = []byte{4}
	}
	l.controls.actions = append(l.controls.actions, protocol.MacroAction{})
	copy(l.controls.actions[index+1:], l.controls.actions[index:])
	l.controls.actions[index] = action
	l.onSelect(index)
	l.Refresh()
	l.onChange()
}

func macroKindID(kind protocol.MacroActionKind) string {
	switch kind {
	case protocol.MacroPress:
		return "macro.press"
	case protocol.MacroRelease:
		return "macro.release"
	case protocol.MacroDelay:
		return "macro.delay"
	case protocol.MacroText:
		return "macro.text"
	default:
		return "macro.click"
	}
}
