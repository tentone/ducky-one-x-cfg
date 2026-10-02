package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	fyneTest "fyne.io/fyne/v2/test"
	"github.com/tentone/ducky-drv/internal/i18n"
	"github.com/tentone/ducky-drv/internal/protocol"
)

func macroEditorForTest(t *testing.T) (*macroControls, fyne.App) {
	t.Helper()
	app := fyneTest.NewApp()
	t.Cleanup(app.Quit)
	u := &UI{app: app, window: app.NewWindow("macros"), i18n: i18n.New()}
	c := u.buildMacros()
	u.window.SetContent(c.root)
	u.window.Resize(fyne.NewSize(1000, 700))
	u.window.Show()
	return c, app
}

func TestMacroClickLoadsAndUpdatesExistingActions(t *testing.T) {
	c, _ := macroEditorForTest(t)
	c.actions = []protocol.MacroAction{
		{Kind: protocol.MacroClick, Keys: []byte{5}},
		{Kind: protocol.MacroDelay, DelayMS: 250, RandomMS: 40},
		{Kind: protocol.MacroText, Text: "hello"},
	}
	c.list.Refresh()
	if !c.update.Disabled() {
		t.Fatal("update enabled without a selection")
	}
	fyneTest.Tap(c.list.rows[0])
	if c.key.Selected != "B" || c.selected != 0 {
		t.Fatal("click did not load the key action")
	}
	c.key.SetSelected("C")
	fyneTest.Tap(c.update)
	if c.actions[0].Keys[0] != 6 {
		t.Fatal("key edit was not stored")
	}
	fyneTest.Tap(c.list.rows[1])
	if c.value.Text != "250" || !c.key.Disabled() {
		t.Fatal("delay editor did not load")
	}
	c.value.SetText("400")
	fyneTest.Tap(c.update)
	if c.actions[1].DelayMS != 400 || c.actions[1].RandomMS != 40 {
		t.Fatal("delay edit lost firmware fields")
	}
	fyneTest.Tap(c.list.rows[2])
	if c.value.Text != "hello" {
		t.Fatal("text editor did not load")
	}
	c.value.SetText("world")
	fyneTest.Tap(c.update)
	if len(c.actions) != 3 || c.actions[2].Text != "world" {
		t.Fatal("text edit did not replace the selected action")
	}
	c.list.UnselectAll()
	if c.selected != -1 || !c.update.Disabled() {
		t.Fatal("selection was not reset")
	}
}

func TestMacroDragInsertsReordersAndRejectsOutsideDrop(t *testing.T) {
	c, app := macroEditorForTest(t)
	list := c.list
	origin := app.Driver().AbsolutePositionForObject(list.scroll)
	list.drag(protocol.MacroClick, -1, origin.Add(fyne.NewPos(20, 30)), true)
	if len(c.actions) != 1 || c.selected != 0 {
		t.Fatal("drop into empty list failed")
	}
	for _, kind := range []protocol.MacroActionKind{protocol.MacroPress, protocol.MacroRelease, protocol.MacroDelay, protocol.MacroText} {
		list.insert(kind, len(c.actions))
	}
	beforeFirst := app.Driver().AbsolutePositionForObject(list.rows[0]).Add(fyne.NewPos(20, 1))
	list.rows[4].Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{AbsolutePosition: beforeFirst}})
	list.rows[4].DragEnd()
	if c.actions[0].Kind != protocol.MacroText || c.selected != 0 {
		t.Fatal("upward drag did not move and select the action")
	}
	last := list.rows[len(list.rows)-1]
	afterLast := app.Driver().AbsolutePositionForObject(last).Add(fyne.NewPos(20, last.Size().Height-1))
	list.drag(protocol.MacroText, 0, afterLast, true)
	if c.actions[4].Kind != protocol.MacroText || c.selected != 4 {
		t.Fatal("downward drag did not preserve ordering")
	}
	list.drag(protocol.MacroDelay, -1, beforeFirst, true)
	if c.actions[0].Kind != protocol.MacroDelay || c.actions[0].DelayMS != 100 {
		t.Fatal("palette insertion at the start failed")
	}
	count := len(c.actions)
	list.drag(protocol.MacroClick, -1, origin.Subtract(fyne.NewPos(30, 30)), true)
	if len(c.actions) != count {
		t.Fatal("outside drop changed the macro")
	}
}
