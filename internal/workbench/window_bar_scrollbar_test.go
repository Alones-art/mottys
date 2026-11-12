package workbench

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// The tab scrollbar stays out of the way until the pointer is over a crowded
// bar, and dragging it moves the same offset the wheel uses.
func TestWindowBarScrollbarAppearsOnHoverAndDrags(t *testing.T) {
	state := newTestState(t, testOptions(t))
	state.tabOrientation = tabsHorizontal
	state.tabStrip.area = gfx.Rect{X: 0, Y: 0, Width: 240, Height: 40}
	state.tabStrip.max = 600
	state.tabStrip.offset = 0

	paint := func() *gfx.Canvas {
		var canvas gfx.Canvas
		canvas.SetTree(&tree)
		state.paintWindowBarScrollbar(&canvas)
		return &canvas
	}
	if _, ok := hitByID(paint(), hitWindowBarTabScroll); ok {
		t.Fatal("the scrollbar is drawn before the pointer arrives")
	}

	state.trackWindowBarScrollbar(gfx.PointerEvent{Kind: gfx.PointerMove, X: 120, Y: 38})
	if !state.tabBarHover {
		t.Fatal("hovering the crowded bar did not arm the scrollbar")
	}
	if _, ok := hitByID(paint(), hitWindowBarTabScroll); !ok {
		t.Fatal("the scrollbar has no draggable thumb")
	}

	result, handled := state.windowBarScrollbarPointer(gfx.PointerEvent{
		Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, X: 20, Y: 38,
	})
	if !handled || !result.Handled || !state.tabBarDrag {
		t.Fatalf("pressing the rail did not start a drag: %+v handled=%v", result, handled)
	}
	state.windowBarScrollbarPointer(gfx.PointerEvent{Kind: gfx.PointerMove, X: 200, Y: 38})
	if state.tabStrip.offset <= 0 || state.tabStrip.offset > state.tabStrip.max {
		t.Fatalf("dragging left the offset at %v (max %v)", state.tabStrip.offset, state.tabStrip.max)
	}

	state.windowBarScrollbarPointer(gfx.PointerEvent{Kind: gfx.PointerUp, Button: gfx.PointerButtonPrimary, X: 200, Y: 38})
	if state.tabBarDrag {
		t.Fatal("the thumb is still being dragged after the pointer went up")
	}
	state.trackWindowBarScrollbar(gfx.PointerEvent{Kind: gfx.PointerExit, X: -1, Y: -1})
	if state.tabBarHover {
		t.Fatal("the bar stayed hovered after the pointer left")
	}
	if _, ok := hitByID(paint(), hitWindowBarTabScroll); ok {
		t.Fatal("the scrollbar is still drawn after the pointer left")
	}
}
