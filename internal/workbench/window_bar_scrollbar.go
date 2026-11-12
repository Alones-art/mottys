package workbench

import "github.com/vivaui/viva/gfx"

// The horizontal tab bar already scrolls with the wheel (tabStrip.offset).
// This adds a thin scrollbar that appears only while the pointer is over a
// crowded bar, and lets the thumb be dragged. It is drawn by hand rather than
// as a scroll area so it never covers or re-lays-out the tabs underneath.
var windowBarScrollHits = tree.Feature("window-bar-scroll")

var hitWindowBarTabScroll = tree.ID(windowBarScrollHits, "tabs")

const (
	tabScrollbarHeight     = float32(5)
	tabScrollbarMinThumb   = float32(28)
	tabScrollbarTrackInset = float32(2)
)

// tabScrollbarTrack is the strip at the bottom of the tab area, or false when
// there is nothing to scroll (or the tabs are in the sidebar instead).
func (state *State) tabScrollbarTrack() (gfx.Rect, bool) {
	if state.tabOrientation != tabsHorizontal || state.tabStrip.max <= 0 {
		return gfx.Rect{}, false
	}
	area := state.tabStrip.area
	if area.Width <= tabScrollbarMinThumb || area.Height <= tabScrollbarHeight+2 {
		return gfx.Rect{}, false
	}
	return gfx.Rect{
		X: area.X, Y: area.Y + area.Height - tabScrollbarHeight,
		Width: area.Width, Height: tabScrollbarHeight,
	}, true
}

func (state *State) tabScrollbarThumb(track gfx.Rect) gfx.Rect {
	visible := track.Width
	total := visible + state.tabStrip.max
	thumb := max(visible*visible/total, tabScrollbarMinThumb)
	thumb = min(thumb, visible)
	travel := max(visible-thumb, 0)
	fraction := float32(0)
	if state.tabStrip.max > 0 {
		fraction = min(max(state.tabStrip.offset/state.tabStrip.max, 0), 1)
	}
	return gfx.Rect{X: track.X + travel*fraction, Y: track.Y, Width: thumb, Height: track.Height}
}

// paintWindowBarScrollbar draws the rail and thumb only while the pointer is
// over the crowded bar or the thumb is being dragged. The hit target exists
// only then, so it never steals a click from a tab.
func (state *State) paintWindowBarScrollbar(canvas *gfx.Canvas) {
	track, ok := state.tabScrollbarTrack()
	if !ok || (!state.tabBarHover && !state.tabBarDrag) {
		return
	}
	canvas.Fill(gfx.Rect{
		X: track.X + tabScrollbarTrackInset, Y: track.Y,
		Width: max(track.Width-2*tabScrollbarTrackInset, 0), Height: track.Height,
	}, withAlpha(state.pal.line, .45))
	thumb := state.tabScrollbarThumb(track)
	canvas.Fill(thumb, withAlpha(state.pal.text, .8))
	canvas.HitWithOptions(hitWindowBarTabScroll, thumb,
		state.messages.Tools.WindowBarTabScrollHint, gfx.HitOptions{Role: gfx.RoleButton})
}

// trackWindowBarScrollbar keeps the hover state current. It reports whether a
// repaint is needed, and never consumes the pointer event: the tabs below must
// still receive their own hover and clicks.
func (state *State) trackWindowBarScrollbar(event gfx.PointerEvent) bool {
	track, ok := state.tabScrollbarTrack()
	over := ok && event.Kind != gfx.PointerExit &&
		(pointInRect(event.X, event.Y, track) || state.tabBarDrag)
	if over == state.tabBarHover {
		return false
	}
	state.tabBarHover = over
	if !over && !state.tabBarDrag && state.options.Invalidate != nil {
		state.options.Invalidate()
	}
	return true
}

// windowBarScrollbarPointer handles pressing and dragging the thumb.
func (state *State) windowBarScrollbarPointer(event gfx.PointerEvent) (gfx.EventResult, bool) {
	track, ok := state.tabScrollbarTrack()
	if !ok {
		return gfx.EventResult{}, false
	}
	switch event.Kind {
	case gfx.PointerDown:
		if event.Button != gfx.PointerButtonPrimary || !pointInRect(event.X, event.Y, track) {
			return gfx.EventResult{}, false
		}
		thumb := state.tabScrollbarThumb(track)
		state.tabBarGrab = thumb.Width / 2
		if pointInRect(event.X, event.Y, thumb) {
			state.tabBarGrab = event.X - thumb.X
		}
		state.tabBarDrag = true
		state.setTabScrollbarOffset(event.X-state.tabBarGrab, track)
		return gfx.EventResult{Handled: true, Repaint: true}, true
	case gfx.PointerMove:
		if !state.tabBarDrag {
			return gfx.EventResult{}, false
		}
		state.setTabScrollbarOffset(event.X-state.tabBarGrab, track)
		return gfx.EventResult{Handled: true, Repaint: true}, true
	case gfx.PointerUp, gfx.PointerExit:
		if !state.tabBarDrag {
			return gfx.EventResult{}, false
		}
		state.tabBarDrag = false
		return gfx.EventResult{Handled: true, Repaint: true}, true
	}
	return gfx.EventResult{}, false
}

func (state *State) setTabScrollbarOffset(x float32, track gfx.Rect) {
	thumb := state.tabScrollbarThumb(track)
	travel := max(track.Width-thumb.Width, 0)
	if travel <= 0 {
		return
	}
	fraction := min(max((x-track.X)/travel, 0), 1)
	state.tabStrip.offset = min(max(fraction*state.tabStrip.max, 0), state.tabStrip.max)
}
