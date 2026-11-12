package workbench

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// The hosts page's scrollbar is the strip the scroll area paints beside the
// rows. It shows itself while the pointer is over the page, and the thumb can
// be dragged to move the list; the wheel keeps working as it did.
var hostListBarHits = tree.Feature("host-list-bar")

var hitHostListBar = tree.ID(hostListBarHits, "bar")

// hostListBarGeometry rebuilds the bar the page painted, from the same helper:
// its track and thumb are where the pointer events aim.
func (state *State) hostListBarGeometry() (components.ScrollAreaGeometry, bool) {
	if state.activeSection != sectionHosts || state.modal != modalNone ||
		state.hostsContent <= 0 || state.chrome.hostList.Height <= 0 {
		return components.ScrollAreaGeometry{}, false
	}
	_, geometry := hostListScrollArea(nil, state.Theme(), state.chrome.hostList, state.hostsContent, state.hostsScroll,
		state.hostListBarHover || state.hostListBarDrag)
	return geometry, geometry.MaxOffsetY > 0
}

// trackHostListBar keeps the hover verdict current. It reports whether a
// repaint is needed and never consumes the event: the rows underneath keep
// their own hover and clicks, exactly as the window bar's scrollbar does.
func (state *State) trackHostListBar(event gfx.PointerEvent) bool {
	_, scrolls := state.hostListBarGeometry()
	over := scrolls && event.Kind != gfx.PointerExit &&
		(pointInRect(event.X, event.Y, state.chrome.hostList) || state.hostListBarDrag)
	if over == state.hostListBarHover {
		return false
	}
	state.hostListBarHover = over
	return true
}

// hostListBarPointer drags the thumb. A press on the track grabs the thumb at
// the pointer; moving it maps the thumb's place back onto the list's offset
// through the scroll area's own geometry.
func (state *State) hostListBarPointer(event gfx.PointerEvent) (gfx.EventResult, bool) {
	geometry, scrolls := state.hostListBarGeometry()
	if !scrolls && state.hostListBarDrag {
		state.hostListBarDrag = false
	}
	track := geometry.VerticalTrack
	switch event.Kind {
	case gfx.PointerDown:
		if !scrolls || event.Button != gfx.PointerButtonPrimary || !pointInRect(event.X, event.Y, track) {
			return gfx.EventResult{}, false
		}
		state.hostListBarDrag = true
		state.hostListBarGrab = event.Y - geometry.VerticalThumb.Y
		return gfx.EventResult{Handled: true, Repaint: true}, true
	case gfx.PointerMove:
		if !state.hostListBarDrag {
			return gfx.EventResult{}, false
		}
		state.hostsScroll = geometry.VerticalOffsetForThumb(event.Y - state.hostListBarGrab)
		return gfx.EventResult{Handled: true, Repaint: true}, true
	case gfx.PointerUp:
		if !state.hostListBarDrag {
			return gfx.EventResult{}, false
		}
		state.hostListBarDrag = false
		return gfx.EventResult{Handled: true, Repaint: true}, true
	}
	return gfx.EventResult{}, false
}
