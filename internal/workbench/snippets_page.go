package workbench

import (
	"fmt"
	"math"
	"strings"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// refreshSnippets makes what the page shows of each snippet and the count under
// it, after the list or the language changed.
func (state *State) refreshSnippets() {
	form := &state.snippets
	for index := range form.list {
		entry := &form.list[index]
		command := entry.Command
		shortcut := ""
		if entry.Shortcut.Valid() {
			shortcut = chordLabel(entry.Shortcut)
		}
		entry.cells = [4]string{entry.Name, entry.Group, shortcut, strings.ReplaceAll(command, "\n", " ↵ ")}
	}
	form.selected = max(0, min(form.selected, len(form.list)-1))
	form.deleteArmed = -1
	form.refreshSnippetGroups()
	form.countText = fmt.Sprintf(state.messages.Tools.SnippetCountFmt, len(form.list))
	if len(form.visible) != len(form.list) {
		form.countText = fmt.Sprintf(state.messages.Tools.SnippetCountFilteredFmt, len(form.visible), len(form.list))
	}
}

// enterKeyLabel is the keycap of a snippet that presses Enter after it is sent.
const enterKeyLabel = "↵"

func (state *State) clickSnippetsPage(id gfx.ID) bool {
	if state.activeSection != sectionSnippets || state.modal != modalNone {
		return false
	}
	if state.clickSnippetFilter(id) {
		return true
	}
	if id.Scope() == hitSnippetSort {
		column := item(id)
		if column >= 0 && column < len(snippet{}.cells) && len(state.snippets.list) > 0 {
			if state.snippets.sort.Column == column && state.snippets.sort.Direction == components.TableSortDescending {
				state.snippets.sort = components.TableSortState{}
			} else {
				state.snippets.sort = state.snippets.sort.Toggle(column)
			}
			state.refreshSnippets()
		}
		return true
	}
	switch {
	case id == hitSnippetAdd:
		state.openSnippetDialog(-1)
	case id == hitSnippetSend:
		state.sendSnippetFromPage(state.snippets.selected, state.snippets.sendAt)
	case id == hitSnippetEdit:
		state.openSnippetDialog(state.snippets.selected)
	case id == hitSnippetRemove:
		state.deleteSnippet(state.snippets.selected)
	default:
		return false
	}
	return true
}

// paintSnippetsWorkspace is the Snippets page: the commands kept to send to a
// terminal, with the shortcut of each, and the way to add, edit and remove them.
func (state *State) paintSnippetsWorkspace(canvas *gfx.Canvas, bounds gfx.Rect) {
	pal, m := state.pal, state.messages.Tools
	form := &state.snippets
	canAdd := len(form.list) < maxSnippets
	hasSelection := len(form.list) > 0
	// The title keeps a readable column beside the buttons; a card too narrow for
	// them shows the buttons as their icons, whose tooltips name them.
	const (
		titleLeast = float32(220)
		gap        = float32(10)
	)
	labeled := toolbarButtonWidth(canvas, m.SnippetDelete, false) + toolbarButtonWidth(canvas, m.SnippetEdit, false) +
		toolbarButtonWidth(canvas, m.SnippetSend, true) + toolbarButtonWidth(canvas, m.SnippetAdd, true) + 3*gap + gap + titleLeast
	condensed := labeled > bounds.Width-36
	removeWidth, removeIcon := toolbarButtonLayout(canvas, m.SnippetDelete, 0, iconTrash, condensed)
	sendWidth, sendIcon := toolbarButtonLayout(canvas, m.SnippetSend, iconPlay, iconPlay, condensed)
	editWidth, editIcon := toolbarButtonLayout(canvas, m.SnippetEdit, 0, iconPencil, condensed)
	addWidth, addIcon := toolbarButtonLayout(canvas, m.SnippetAdd, iconPlus, iconPlus, condensed)

	var content, filter, emptyIcon, emptyAdd, remove, edit, send, add ui.Box
	f := form.layout.Begin(canvas, state.Theme(), bounds)
	f.Column().Fill().PadSides(12, 18, 0, 18).Children(func() {
		f.Row().MinHeight(58).Gap(gap).MarginSides(0, 0, 12, 0).Items(ui.Start).Children(func() {
			f.Column().Grow(1).Gap(3).MarginSides(4, 0, 0, 0).Children(func() {
				f.Text(m.SnippetsTitle).FontSize(18).Weight(gfx.WeightSemibold).Color(pal.text).Ellipsis()
				f.Paragraph(m.SnippetsPageDescription, 2).FontSize(metrics.FontSizeS).Color(pal.muted)
			})
			remove = f.Slot().Size(removeWidth, toolbarHeight)
			edit = f.Slot().Size(editWidth, toolbarHeight)
			send = f.Slot().Size(sendWidth, toolbarHeight)
			add = f.Slot().Size(addWidth, toolbarHeight)
		})
		if len(form.list) == 0 {
			emptyIcon, emptyAdd = state.declareEmptyPage(f, bounds, m.SnippetEmpty, m.SnippetEmptySub, m.SnippetAdd)
		} else {
			if len(form.groups) > 0 {
				f.Row().Height(toolbarHeight).MarginSides(0, 0, 9, 0).Children(func() {
					filter = f.Slot().Size(220, toolbarHeight)
				})
			}
			content = f.Slot().Grow(1).MarginSides(0, 0, 9, 0)
		}
		f.Divider().Background(pal.line)
		f.Row().Height(50).PadSides(8, 0, 20, 0).Items(ui.Center).Children(func() {
			f.Text(form.countText).FontSize(metrics.FontSizeS).Weight(gfx.WeightMedium).Color(pal.faint).Width(162)
			if hasSelection {
				f.Text(form.list[form.selected].Name).FontSize(metrics.FontSizeS).Color(pal.muted).Grow(1).Ellipsis()
			}
		})
	})
	f.End()

	removeLabel := m.SnippetDelete
	if state.snippets.deleteArmed == state.snippets.selected {
		removeLabel = m.SnippetDeleteConfirm
	}
	state.paintToolbarButton(canvas, hitSnippetRemove, remove.Rect().X, remove.Rect().Y, removeWidth, removeLabel, removeIcon, false, hasSelection, condensed)
	state.paintToolbarButton(canvas, hitSnippetEdit, edit.Rect().X, edit.Rect().Y, editWidth, m.SnippetEdit, editIcon, false, hasSelection, condensed)
	form.sendAt = gfx.Point{X: send.Rect().X, Y: send.Rect().Y + toolbarHeight + 4}
	state.paintToolbarButton(canvas, hitSnippetSend, send.Rect().X, send.Rect().Y, sendWidth, m.SnippetSend, sendIcon, false, hasSelection, condensed)
	state.paintToolbarButton(canvas, hitSnippetAdd, add.Rect().X, add.Rect().Y, addWidth, m.SnippetAdd, addIcon, true, canAdd, condensed)

	if len(form.list) == 0 {
		state.paintEmptyPage(canvas, emptyIcon, emptyAdd, iconNote, hitSnippetAdd, m.SnippetAdd, canAdd)
		return
	}
	// The command column leaves room before its text for the Enter keycap of the
	// snippets that press it, so the commands of all rows start at the same place.
	theme := state.Theme()
	keyHeight := theme.Metrics.FontSizeS + 9
	keyWidth := max(canvas.TextWidthWeight(enterKeyLabel, theme.Metrics.FontSizeS, gfx.WeightMedium)+8, keyHeight)
	manualOrder := form.sort.Direction == components.TableSortNone && form.filter.all() && len(form.list) > 1
	nameInset := float32(0)
	if manualOrder {
		nameInset = 24
	}
	columns := [...]components.TableColumn{{Width: 200, Inset: nameInset}, {Width: 140}, {Width: 150}, {Width: 0, Inset: keyWidth + 6}}
	headings := [...]string{m.SnippetColumnName, m.SnippetColumnGroup, m.SnippetColumnShortcut, m.SnippetColumnCommand}
	var headers [4]components.TableHeader
	for index, heading := range headings {
		headers[index] = components.TableHeader{ID: hitSnippetSort.At(index), Label: heading, Description: m.SnippetSortAction}
		if form.sort.Direction != components.TableSortNone && form.sort.Column == index {
			headers[index].Sort = form.sort.Direction
			if form.sort.Direction == components.TableSortAscending {
				headers[index].Description = m.SnippetSortAscending
			} else {
				headers[index].Description = m.SnippetSortDescending
			}
		}
	}
	// The heading strip stays fixed and the rows scroll under it: however long
	// the list grows, every snippet stays reachable.
	rowHeight, headerHeight := components.TableRowHeight(theme), components.TableHeaderHeight(theme)
	headerBounds := content.Rect()
	headerBounds.Height = headerHeight
	bodyBounds := content.Rect()
	bodyBounds.Y += headerHeight
	bodyBounds.Height = max(bodyBounds.Height-headerHeight, 0)
	header := components.BeginTable(canvas, theme, headerBounds, columns[:]).HeaderHeight(headerHeight).RowHeight(rowHeight)
	header.SortableHeader(headers[:])

	contentHeight := max(float32(len(form.visible))*rowHeight, bodyBounds.Height)
	area := components.BeginScrollArea(canvas, theme, bodyBounds, gfx.Size{Width: bodyBounds.Width, Height: contentHeight}).
		Offset(0, form.scroll).
		Viewport(hitSnippetTable, m.SnippetsTitle).
		VerticalScrollbar(hitSnippetScroll, m.SnippetsTitle, "")
	geometry := area.Geometry()
	form.scroll = geometry.OffsetY
	form.scrollBar = geometry
	geometry = area.BeginContent()
	form.scroll = geometry.OffsetY

	first := min(max(int(math.Floor(float64(geometry.OffsetY/rowHeight))), 0), len(form.visible))
	last := min(first+int(math.Ceil(float64(geometry.Viewport.Height/rowHeight)))+1, len(form.visible))
	remainder := float32(math.Mod(float64(geometry.OffsetY), float64(rowHeight)))
	tableBounds := gfx.Rect{
		X: geometry.Viewport.X, Y: geometry.Viewport.Y - remainder,
		Width:  geometry.Viewport.Width,
		Height: min(geometry.Viewport.Height+rowHeight, float32(last-first)*rowHeight),
	}
	table := components.BeginTable(canvas, theme, tableBounds, columns[:]).RowHeight(rowHeight)

	var grid components.ReorderGrid
	if manualOrder {
		grid = components.BeginReorderGrid(canvas, theme, tableBounds, &form.grid)
		for index := range form.list {
			grid.Order(hitSnippetHandles.At(index), 0, index, false)
		}
	} else {
		form.grid.Cancel()
	}
	for _, index := range form.visible[first:last] {
		rowID := hitSnippetRowList.At(index)
		var cellRects [4]gfx.Rect
		row := table.RowLayout(rowID, form.list[index].cells[:], components.TableRowState{Selected: index == form.selected}, cellRects[:0])
		if manualOrder && row.Bounds.Height > 0 {
			grid.Item(hitSnippetHandles.At(index), form.list[index].Name, 0, index, row.Bounds,
				gfx.Rect{X: row.Bounds.X + 7, Y: row.Bounds.Y + (row.Bounds.Height-24)/2, Width: 24, Height: 24}, false)
		}
		if clicked, ok := canvas.ClickedIn(hitSnippetRowList); ok && clicked == index && state.modal == modalNone {
			form.selected = index
		}
		if form.list[index].Run && len(row.Cells) == len(cellRects) {
			command := row.Cells[3]
			components.Kbd(canvas, theme, gfx.Rect{X: command.X + theme.Metrics.GapItem, Y: command.Y + (command.Height-keyHeight)/2, Height: keyHeight}, enterKeyLabel)
		}
		state.sendSnippetOn(canvas, rowID, index)
		state.openSnippetMenuOn(canvas, rowID, index)
	}
	if manualOrder {
		grid.End()
	}
	area.EndContent()
	if len(form.groups) > 0 {
		state.paintSnippetFilter(canvas, filter.Rect())
	}
}

// snippetsScrollPointer drags the snippets table's scrollbar. It only speaks
// while the snippets page is in front and the list actually overflows.
func (state *State) snippetsScrollPointer(event gfx.PointerEvent) (gfx.EventResult, bool) {
	if state.activeSection != sectionSnippets || state.modal != modalNone {
		return gfx.EventResult{}, false
	}
	form := &state.snippets
	if form.scrollBar.MaxOffsetY <= 0 && !form.scrollDragging {
		return gfx.EventResult{}, false
	}
	geometry := form.scrollBar
	before := form.scroll
	switch event.Kind {
	case gfx.PointerDown:
		if event.Button != gfx.PointerButtonPrimary || event.Target != hitSnippetScroll {
			return gfx.EventResult{}, false
		}
		form.scrollDragging = true
		form.scrollGrab = geometry.VerticalThumb.Height / 2
		if event.Y >= geometry.VerticalThumb.Y && event.Y < geometry.VerticalThumb.Y+geometry.VerticalThumb.Height {
			form.scrollGrab = event.Y - geometry.VerticalThumb.Y
		} else {
			form.scroll = geometry.VerticalOffsetForThumb(event.Y - form.scrollGrab)
		}
	case gfx.PointerMove:
		if !form.scrollDragging || !event.Captured {
			return gfx.EventResult{}, false
		}
		form.scroll = geometry.VerticalOffsetForThumb(event.Y - form.scrollGrab)
	case gfx.PointerUp:
		if !form.scrollDragging {
			return gfx.EventResult{}, false
		}
		form.scrollDragging = false
		return gfx.EventResult{Handled: true, Repaint: true}, true
	case gfx.PointerExit:
		if !form.scrollDragging || !event.Captured {
			return gfx.EventResult{}, false
		}
		return gfx.EventResult{Handled: true}, true
	default:
		return gfx.EventResult{}, false
	}
	return gfx.EventResult{Handled: true, Repaint: before != form.scroll}, true
}

// scrollSnippets moves the snippets table with the wheel.
func (state *State) scrollSnippets(event gfx.ScrollEvent) gfx.EventResult {
	form := &state.snippets
	if form.scrollBar.MaxOffsetY <= 0 {
		return gfx.EventResult{}
	}
	before := form.scroll
	step := event.StepY(components.TableRowHeight(state.Theme()))
	form.scroll = min(max(form.scroll+step, 0), form.scrollBar.MaxOffsetY)
	return gfx.EventResult{Handled: true, Repaint: form.scroll != before}
}
