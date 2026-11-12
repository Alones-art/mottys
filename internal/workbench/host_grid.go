package workbench

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// The hosts page shows its hosts as a list or as cards. Both read the same rows
// (hostTreeRows), use the same ids for selecting, checking and opening a host and
// share one scroll position; only the layout differs.

type hostViewMode uint8

const (
	hostViewList hostViewMode = iota
	hostViewGrid
)

// hostGridHits is the place of the controls this file paints.
var hostGridHits = tree.Feature("host-grid")

var (
	hitHostViewList = tree.ID(hostGridHits, "host-view-list")
	hitHostViewGrid = tree.ID(hostGridHits, "host-view-grid")
	hitHostSort     = tree.ID(hostGridHits, "host-sort")
)

const (
	hostCardMinWidth = float32(280)
	hostCardHeight   = float32(64)
	hostCardGap      = float32(12)
	hostGridHeader   = float32(42)
)

func hostViewFromString(value string) hostViewMode {
	if value == "grid" {
		return hostViewGrid
	}
	return hostViewList
}

func (mode hostViewMode) saved() string {
	if mode == hostViewGrid {
		return "grid"
	}
	return ""
}

func (state *State) clickHostView(id gfx.ID) bool {
	if id == hitHostSort {
		state.toggleHostSort()
		return true
	}
	var mode hostViewMode
	switch id {
	case hitHostViewList:
		mode = hostViewList
	case hitHostViewGrid:
		mode = hostViewGrid
	default:
		return false
	}
	if state.activeSection == sectionHosts && state.modal == modalNone && state.hostView != mode {
		state.hostView, state.hostsScroll = mode, 0
		state.save()
	}
	return true
}

// paintHostViewToggle draws the view buttons and the host-sort mode toggle from
// x, centred on the line. The caller is paintHostsHeader, which hands over the
// line, not a box.
func (state *State) paintHostViewToggle(canvas *gfx.Canvas, x, center float32, withOrder bool) {
	m := state.messages.Tools
	buttons := [...]struct {
		id    gfx.ID
		icon  uint8
		label string
		mode  hostViewMode
	}{
		{hitHostViewList, iconRows2, m.HostViewList, hostViewList},
		{hitHostViewGrid, iconHome, m.HostViewGrid, hostViewGrid},
	}
	var slots [len(buttons)]ui.Box
	var sort, order ui.Box
	width := float32(86)
	if withOrder {
		width += 32
	}
	f := state.hostToggleLayout.Begin(canvas, state.Theme(), gfx.Rect{X: x, Y: center, Width: width})
	f.Row().Fill().Items(ui.Center).Children(func() {
		f.Row().Width(54).Gap(2).Children(func() {
			for index := range buttons {
				slots[index] = f.Slot().Size(26, 26)
			}
		})
		sort = f.Slot().Size(26, 26).MarginSides(0, 0, 0, 6)
		if withOrder {
			order = f.Slot().Size(26, 26).MarginSides(0, 0, 0, 6)
		}
	})
	f.End()
	for index, button := range buttons {
		style := iconButtonQuiet
		if state.hostView == button.mode {
			style = iconButtonActive
		}
		state.paintIconButton(canvas, button.id, button.icon, slots[index].Rect(), button.label, false, style, state.pal.workspace)
	}
	label := m.HostSort
	style := iconButtonQuiet
	if state.hostSort.enabled {
		label = m.HostSortDone
		style = iconButtonActive
	}
	disabled := !state.hostSort.enabled && (state.hostOrder != hostOrderManual || len(state.hosts) < 2 || state.typedQuery() != "")
	state.paintIconButton(canvas, hitHostSort, iconTransfer, sort.Rect(), label, disabled, style, state.pal.workspace)
	if withOrder {
		state.paintHostOrderButton(canvas, order.Rect())
	}
}

// hostGridColumns is how many cards fit across width.
func hostGridColumns(width float32) int {
	return max(int((width+hostCardGap)/(hostCardMinWidth+hostCardGap)), 1)
}

// hostCardLine is one row of cards. Folder cards are gathered onto lines of
// their own above the host cards, and one folder's hosts never share a line
// with another folder's, so a server card reads as belonging to a folder.
type hostCardLine struct {
	entries []int // indices into hostTreeRows
}

func (state *State) hostCardLines(columns int) []hostCardLine {
	rows := state.hostTreeRows
	var lines []hostCardLine
	var folders []int
	for index, row := range rows {
		if row.Kind == hostTreeGroup {
			folders = append(folders, index)
		}
	}
	for start := 0; start < len(folders); start += columns {
		end := min(start+columns, len(folders))
		lines = append(lines, hostCardLine{entries: append([]int(nil), folders[start:end]...)})
	}
	// Hosts keep their order, but crossing into another folder starts a new
	// line: two folders' servers must not sit side by side with no label.
	var hosts []int
	current := -1
	flush := func() {
		for start := 0; start < len(hosts); start += columns {
			end := min(start+columns, len(hosts))
			lines = append(lines, hostCardLine{entries: append([]int(nil), hosts[start:end]...)})
		}
		hosts = nil
	}
	for index, row := range rows {
		if row.Kind != hostTreeHost {
			continue
		}
		if current != row.GroupIndex && len(hosts) > 0 {
			flush()
		}
		current = row.GroupIndex
		hosts = append(hosts, index)
	}
	flush()
	return lines
}

// hostGridHeight is the height of the whole card layout at width.
func (state *State) hostGridHeight(width float32) float32 {
	return float32(len(state.hostCardLines(hostGridColumns(width)))) * (hostCardHeight + hostCardGap)
}

// hostGridItem is a card or a group header that is on screen, with the box
// the layout gave it.
type hostGridItem struct {
	box   ui.Box
	entry int // the index into hostTreeRows
}

// paintHostGrid draws the cards inside bounds, scrolled by hostsScroll. Only
// what is on screen is declared: the rows above the window are skipped by their
// known height and the layout is shifted up by what is left of the first one.
func (state *State) paintHostGrid(canvas *gfx.Canvas, bounds gfx.Rect) {
	state.beginHostSortPaint(canvas, bounds)
	columns := hostGridColumns(bounds.Width)
	rows := state.hostTreeRows
	items := state.hostGridItems[:0]
	y := bounds.Y - state.hostsScroll
	bottom := bounds.Y + bounds.Height
	shift, first := float32(0), true
	f := state.hostGridLayout.Begin(canvas, state.Theme(), bounds)
	lines := state.hostCardLines(columns)
	list := f.Column().FillWidth().Children(func() {
		for lineIndex := 0; lineIndex < len(lines) && y < bottom; lineIndex++ {
			line := lines[lineIndex]
			if y+hostCardHeight >= bounds.Y {
				if first {
					shift, first = y-bounds.Y, false
				}
				f.Row().Height(hostCardHeight).Gap(hostCardGap).MarginSides(0, 0, hostCardGap, 0).Shrink(0).Children(func() {
					for column := 0; column < columns; column++ {
						card := f.Slot().Grow(1).Basis(0)
						if column < len(line.entries) {
							items = append(items, hostGridItem{card, line.entries[column]})
						}
					}
				})
			}
			y += hostCardHeight + hostCardGap
		}
	})
	list.MarginSides(shift, 0, 0, 0)
	f.End()
	state.hostGridItems = items

	for _, item := range items {
		entry := rows[item.entry]
		if entry.Kind == hostTreeGroup {
			state.paintHostGroupCard(canvas, item.box.Rect(), entry.GroupIndex)
		} else {
			state.paintHostCard(canvas, item.box.Rect(), state.hosts[entry.HostIndex])
		}
	}
	state.endHostSortPaint()
}

// paintHostGroupCard draws a folder as a card the same size as a host card, so
// several folders share one grid line. It keeps every hit the old full-width
// header had: the card toggles the folder, the checkbox selects its hosts, and
// the "..." button and secondary-click menu still offer the group actions.
func (state *State) paintHostGroupCard(canvas *gfx.Canvas, card gfx.Rect, groupIndex int) {
	pal, group := state.pal, state.hostGroups[groupIndex]
	id := hitHostGroupOpenList.At(groupIndex)
	background, edge, edgeWidth := pal.surface, withAlpha(pal.line, 0.8), float32(1)
	if canvas.Hovered() == id {
		background, edge = pal.surfaceSoft, withAlpha(pal.line, 0.9)
	}
	canvas.RoundedBorder(card, radius.surface, edgeWidth, edge, background)
	canvas.HitWithOptions(id, card, group.Name, gfx.HitOptions{Role: gfx.RoleButton, Expanded: group.Expanded})
	if canvas.FocusVisible() == id {
		canvas.RoundedBorder(card, radius.surface, 1.5, pal.accent, gfx.Color{})
	}
	label := group.Label
	if group.Name == "" {
		label = state.messages.HostGroupDefault
	}
	chevron := iconChevronRight
	if group.Expanded || state.visibleQuery != "" {
		chevron = iconChevronDown
	}
	var icon, chevronBox, groupCheck ui.Box
	f := state.partLayout.Begin(canvas, state.Theme(), card)
	f.Row().Fill().PadSides(0, 40, 0, 14).Gap(10).Items(ui.Center).Children(func() {
		icon = f.Slot().Size(32, 32).Shrink(0)
		f.Column().Grow(1).Basis(0).Children(func() {
			f.Row().Gap(6).Items(ui.Center).Children(func() {
				chevronBox = f.Slot().Size(14, 14).Shrink(0)
				f.Text(label).FontSize(metrics.FontSizeM).Weight(gfx.WeightSemibold).Color(pal.text).LineHeight(22).Ellipsis().Shrink(1)
			})
			f.Text(group.Count).FontSize(metrics.FontSizeS).Color(pal.muted).LineHeight(19).Ellipsis()
		})
		groupCheck = f.Slot().Size(40, 28).Shrink(0)
	})
	f.End()
	canvas.RoundedBorder(icon.Rect(), radius.item, 0, gfx.Color{}, pal.surfaceSoft)
	paintIcon(canvas, uint8(iconFolderOpen), icon.Rect(), pal.accent)
	paintIcon(canvas, uint8(chevron), chevronBox.Rect(), pal.muted)
	state.paintHostScopeCheckbox(canvas, hitHostGroupCheck.At(groupIndex), groupCheck.Rect(), state.hostScopeSelection.groups[groupIndex])
	state.hostGroupRowEvents(canvas, id, groupIndex, card)
}

func (state *State) paintHostCard(canvas *gfx.Canvas, card gfx.Rect, host Host) {
	pal := state.pal
	id := hitHostList.At(int(host.ID))
	selected := host.ID == state.selectedHostID
	background, edge, edgeWidth := pal.surface, withAlpha(pal.line, 0.8), float32(1)
	switch {
	case state.hostSortDragging(host.ID):
		background, edge, edgeWidth = pal.surfaceSoft, pal.accent, 1.5
	case state.hostSortTarget(host.ID):
		edge, edgeWidth = pal.accent, 2
	case selected:
		background, edge, edgeWidth = pal.accentSoft, pal.accent, 1.5
	case state.hostSortHovered(canvas, host.ID):
		background, edge, edgeWidth = pal.surfaceSoft, pal.accent, 1.5
	case canvas.Hovered() == id:
		background, edge = pal.surfaceSoft, withAlpha(pal.line, 0.9)
	}
	// The icon badge picks its fill from the exact card background
	// (hostBadgeFill); give it the pre-tint one so a selected card keeps the
	// contrasting badge instead of accentSoft-on-accentSoft.
	badgeBackground := background
	background = tintHostSurface(background, host.Color, .10)
	if state.hostSortDragging(host.ID) {
		canvas.Elevate(card, radius.surface, gfx.ShadowMD)
	}
	canvas.RoundedBorder(card, radius.surface, edgeWidth, edge, background)
	paintHostColorMark(canvas, card, host.Color)
	if !state.hostSort.enabled {
		canvas.HitWithOptions(id, card, state.hostDisplayName(host), gfx.HitOptions{Role: gfx.RoleButton, Selected: selected})
		state.hostRowEvents(canvas, id, host)
	}
	if canvas.FocusVisible() == id {
		canvas.RoundedBorder(card, radius.surface, 1.5, pal.accent, gfx.Color{})
	}

	// The avatar and the two lines of text share the card's vertical center with
	// the checkbox. The connection's state is a light on the avatar's corner.
	_, dot := state.hostStatus(host)
	kind, _ := state.hostLight(host)
	// Keep the account and the destination on one line, each named once, then
	// show as many whole tag pills as fit before the checkbox's full interaction
	// region.
	subtitle := state.hostConnectionText(host)
	checkWidth, checkHeight := components.CheckboxHitExtent(state.Theme())
	textWidth := max(card.Width-14-12-36-12*2-checkWidth, 0)
	tagCount := 0
	var tagList [maxHostTags]string
	subtitleWidth := min(canvas.TextWidth(subtitle, metrics.FontSizeS)+1, textWidth)
	if room := textWidth - subtitleWidth - 8; room >= 40 {
		tagCount = state.fitHostTags(canvas, host, room, &tagList)
	}
	var grip, avatar, light ui.Box
	f := state.partLayout.Begin(canvas, state.Theme(), card)
	f.Row().Fill().PadSides(0, 12, 0, 14).Gap(12).Items(ui.Center).Children(func() {
		f.Stack().Size(36, 36).Shrink(0).Children(func() {
			avatar = f.Slot()
			if kind != lightIdle {
				light = f.Slot().Size(14, 14).Self(ui.End).MarginSides(0, -5, -5, 0)
			}
		})
		f.Column().Grow(1).Basis(0).Children(func() {
			f.Text(state.hostDisplayName(host)).FontSize(metrics.FontSizeM).Weight(gfx.WeightSemibold).Color(pal.text).LineHeight(22).Ellipsis()
			f.Row().Gap(8).Items(ui.Center).MarginSides(1, 0, 0, 0).Children(func() {
				f.Text(subtitle).FontSize(metrics.FontSizeS).Color(pal.muted).LineHeight(19).Width(subtitleWidth).Shrink(0).Ellipsis()
				if tagCount > 0 {
					f.Row().Gap(5).Shrink(0).Items(ui.Center).Children(func() {
						for _, tag := range tagList[:tagCount] {
							f.Row().Height(18).PadX(7).Shrink(0).Items(ui.Center).Radius(9).Background(pal.surfaceSoft).Children(func() {
								f.Text(state.hostTagText(tag)).FontSize(metrics.FontSizeS).Weight(gfx.WeightMedium).Color(pal.muted).Width(canvas.TextWidthWeight(tag, metrics.FontSizeS, gfx.WeightMedium) + 1).Shrink(0)
							})
						}
					})
				}
			})
		})
		if state.hostSort.enabled {
			// Match the card's ordinary checkbox exactly: sorting replaces it in
			// place, while the six-dot glyph is the same size as the list view.
			grip = f.Slot().Size(checkWidth, checkHeight).Shrink(0)
		} else {
			f.Checkbox(hitHostCheckList.At(int(host.ID)), state.hostSelected(host.ID)).Label(host.Label)
		}
	})
	f.End()
	if state.hostSort.enabled {
		state.declareHostSortItem(host, card, grip.Rect())
	}
	state.paintHostCatalogIcon(canvas, host, avatar.Rect(), badgeBackground)
	if kind != lightIdle {
		corner := light.Rect()
		canvas.Rounded(corner, corner.Width/2, background)
		paintLight(canvas, corner.X+corner.Width/2, corner.Y+corner.Height/2, kind, dot, background)
	}
}

// hostCardConnection is the card's second line: the account and the destination,
// each named once. A desktop target already spells its protocol, and a VNC host
// reached over SSH already carries its account, so those take the target as it
// is; a serial line has no account at all and its target holds the device and
// its settings. Only a bare destination, an SSH or telnet address, takes the
// login in front of it.
func (state *State) hostCardConnection(host Host) string {
	switch {
	case host.target == "":
		return state.hostSubtitle(host)
	case host.Protocol == protocolSerial, host.Protocol == protocolVNC:
		return host.target
	case host.Protocol == protocolRDP:
		if host.User == "" {
			return host.target
		}
		return host.User + " · " + host.target
	}
	return state.hostSubtitle(host) + " · " + host.target
}
