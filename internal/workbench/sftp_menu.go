package workbench

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// sftpMenuHits is the place of the controls this file paints.
var sftpMenuHits = tree.Feature("sftp-menu")

// The file panel's right-click menu. Each row stands for a button of the bar under
// the list and runs the same action; it has its own identity because both are on
// the screen at once.
var (
	hitSFTPMenuDismiss      = tree.ID(sftpMenuHits, "sftp-menu-dismiss")
	hitSFTPMenuOpen         = tree.ID(sftpMenuHits, "sftp-menu-open")
	hitSFTPMenuDownload     = tree.ID(sftpMenuHits, "sftp-menu-download")
	hitSFTPMenuDownloadHere = tree.ID(sftpMenuHits, "sftp-menu-download-here")
	hitSFTPMenuSend         = tree.ID(sftpMenuHits, "sftp-menu-send")
	hitSFTPMenuInsertPath   = tree.ID(sftpMenuHits, "sftp-menu-insert-path")
	hitSFTPMenuEdit         = tree.ID(sftpMenuHits, "sftp-menu-edit")
	hitSFTPMenuRename       = tree.ID(sftpMenuHits, "sftp-menu-rename")
	hitSFTPMenuMove         = tree.ID(sftpMenuHits, "sftp-menu-move")
	hitSFTPMenuChmod        = tree.ID(sftpMenuHits, "sftp-menu-chmod")
	hitSFTPMenuExtract      = tree.ID(sftpMenuHits, "sftp-menu-extract")
	hitSFTPMenuCompress     = tree.ID(sftpMenuHits, "sftp-menu-compress")
	hitSFTPMenuDelete       = tree.ID(sftpMenuHits, "sftp-menu-delete")
	hitSFTPMenuSudo         = tree.ID(sftpMenuHits, "sftp-menu-sudo")
)

// sftpMenuAction is the button of the bar that a menu row stands for.
func sftpMenuAction(id gfx.ID) (gfx.ID, bool) {
	switch id {
	case hitSFTPMenuOpen:
		return hitSFTPOpenEntry, true
	case hitSFTPMenuDownload:
		return hitSFTPDownload, true
	case hitSFTPMenuDownloadHere:
		return hitSFTPDownloadHere, true
	case hitSFTPMenuSend:
		return hitSFTPSend, true
	case hitSFTPMenuInsertPath:
		return hitSFTPInsertPath, true
	case hitSFTPMenuEdit:
		return hitSFTPEditExternal, true
	case hitSFTPMenuRename:
		return hitSFTPRename, true
	case hitSFTPMenuMove:
		return hitSFTPMove, true
	case hitSFTPMenuChmod:
		return hitSFTPChmod, true
	case hitSFTPMenuExtract:
		return hitSFTPExtract, true
	case hitSFTPMenuCompress:
		return hitSFTPCompress, true
	case hitSFTPMenuDelete:
		return hitSFTPDelete, true
	case hitSFTPMenuSudo:
		return hitSFTPMenuSudo, true
	case hitSFTPNewDirectory, hitSFTPNewFile, hitSFTPUpload, hitSFTPUploadDirectory, hitSFTPNewLink,
		hitSFTPCopyPath, hitSFTPProperties:
		return id, true
	}
	return gfx.ID{}, false
}

// sftpMenu is the right-click menu of the file list: which session's panel it was
// asked in, and where. Closed, it costs nothing.
type sftpMenu struct {
	open    bool
	session uint32
	at      gfx.Point
}

// sftpMenuDismissPointer closes an open menu on a secondary press outside it; the
// backdrop swallows the press, and only a primary one is also a click.
func (state *State) sftpMenuDismissPointer(event gfx.PointerEvent) (gfx.EventResult, bool) {
	if event.Kind != gfx.PointerDown || event.Button == gfx.PointerButtonNone || event.Button == gfx.PointerButtonPrimary {
		return gfx.EventResult{}, false
	}
	if state.sftpMenu.open && event.Target == hitSFTPMenuDismiss {
		state.sftpMenu.open = false
		return gfx.EventResult{Handled: true, Repaint: true}, true
	}
	return gfx.EventResult{}, false
}

// sftpMenuPointer opens the menu on a secondary press anywhere in the list. On
// a row it selects that row first; on the empty space below the rows the menu
// belongs to the folder, so the selection is dropped.
func (state *State) sftpMenuPointer(event gfx.PointerEvent) (gfx.EventResult, bool) {
	if event.Kind != gfx.PointerDown || event.Button != gfx.PointerButtonSecondary {
		return gfx.EventResult{}, false
	}
	onRow := event.Target.Scope() == hitSFTPEntryList
	if !onRow && event.Target != hitSFTPListBlank {
		return gfx.EventResult{}, false
	}
	session, ok := state.sessionByIDPointer(state.activeSessionID)
	if !ok || state.activeSection != sectionTerminal || state.modal != modalNone || session.files == nil || session.files.editPath != "" {
		return gfx.EventResult{}, false
	}
	view := session.files
	folder := view.active()
	if view.dialog != sftpDialogNone || view.busy && !view.local {
		return gfx.EventResult{Handled: true}, true
	}
	if folder.searching() {
		// The actions in the menu are the folder's; a result is not in it.
		if onRow {
			folder.search.selectRow(folder.first + item(event.Target))
		}
		return gfx.EventResult{Handled: true, Repaint: true}, true
	}
	if onRow {
		index := folder.first + item(event.Target)
		if index >= len(folder.shownRows()) {
			return gfx.EventResult{Handled: true}, true
		}
		if !folder.rows[index].selected {
			folder.selectRow(index, 0)
			view.setItemCountStatus(state.messages.SFTPItemsFmt, selectedSFTPCount(view))
		}
	} else {
		// A press on blank space is about the folder, not its entries: keep the
		// menu free of actions that would delete or copy rows nobody picked.
		clearSFTPSelection(folder)
		view.setItemCountStatus(state.messages.SFTPItemsFmt, 0)
	}
	state.sftpMenu = sftpMenu{open: true, session: session.ID, at: state.terminalMenu.pointer}
	return gfx.EventResult{Handled: true, Repaint: true}, true
}

// clearSFTPSelection drops the checkmarks of the rows on screen and of the rows
// a filter currently hides. A filter rebuilds the shown rows from allRows, so
// clearing only one of the two would bring a discarded selection back later.
func clearSFTPSelection(folder *sftpFolder) {
	for index := range folder.allRows {
		folder.allRows[index].selected = false
		folder.allRows[index].cells[0] = ""
	}
	for index := range folder.rows {
		folder.rows[index].selected = false
		folder.rows[index].cells[0] = ""
	}
	folder.selected, folder.anchor = -1, -1
}

// sftpMenuView is the panel an open menu acts on, if it is still there to act on.
func (state *State) sftpMenuView() (*Session, *sftpView, bool) {
	if !state.sftpMenu.open || state.activeSection != sectionTerminal || state.modal != modalNone || state.sftpMenu.session != state.activeSessionID {
		return nil, nil, false
	}
	session, ok := state.sessionByIDPointer(state.sftpMenu.session)
	if !ok || session.files == nil || session.files.dialog != sftpDialogNone || session.files.editPath != "" {
		return nil, nil, false
	}
	return session, session.files, true
}

func (state *State) paintSFTPMenu(canvas *gfx.Canvas, viewport gfx.Rect) {
	session, view, ok := state.sftpMenuView()
	if !ok {
		state.sftpMenu.open = false
		return
	}
	rows := state.sftpMenuRows(session, view)
	width, height := rows.size(canvas, state.contextMenuTheme())
	width = min(width, 480)
	bounds := state.popupAt(canvas, viewport, state.sftpMenu.at, width, height, 6)
	menu := components.BeginContextMenu(canvas, state.contextMenuTheme(), bounds, components.ContextMenuOptions{
		Layer: dialogLayer, Scope: dialogScope, DismissID: hitSFTPMenuDismiss, Viewport: viewport, Scroll: state.menuScrollFor(hitSFTPMenuDismiss), RowHeight: contextMenuRow,
	})
	state.menuPlaced(menu.Bounds())
	rows.paint(&menu, nil)
	menu.End()
}

func (state *State) sftpMenuRows(session *Session, view *sftpView) menuRows {
	messages := state.messages
	count, selected := selectedSFTP(view)
	one, local := count == 1, view.local
	busy := view.busy && !local
	icon := func(kind uint8) gfx.Icon { return lucideIcons[kind] }
	var rows menuRows
	if one {
		rows.add(components.ContextMenuItem{ID: hitSFTPMenuOpen, Label: messages.SFTPOpen, Icon: icon(iconFolderOpen), Disabled: busy})
		if local && selected.Mode.IsRegular() {
			rows.add(components.ContextMenuItem{ID: hitSFTPMenuEdit, Label: messages.Tools.ExtEditButton, Icon: icon(iconExternal)})
		}
		rows.separate()
	}
	// These act on the selection. A right click on blank space has no selection,
	// so its menu is only the folder-level actions further down.
	if count > 0 {
		switch {
		case view.localOnly:
			rows.add(components.ContextMenuItem{ID: hitSFTPMenuInsertPath, Label: messages.SFTPInsertPath, Icon: icon(iconTerminal), Disabled: session.term == nil || session.term.shell == nil})
		case local:
			rows.add(components.ContextMenuItem{ID: hitSFTPMenuSend, Label: messages.Tools.SendToServer,
				Description: view.path, Value: view.path, Hint: view.path, Icon: icon(iconUpload), Disabled: view.path == ""})
		default:
			rows.add(components.ContextMenuItem{ID: hitSFTPMenuDownload, Label: messages.SFTPDownload, Icon: icon(iconDownload), Disabled: busy})
			if view.lfolder.path != "" {
				rows.add(components.ContextMenuItem{ID: hitSFTPMenuDownloadHere, Label: messages.Tools.DownloadHere, Icon: icon(iconFileDown), Disabled: busy})
			}
		}
	}
	if one && !local {
		if !selected.IsDir() {
			rows.add(components.ContextMenuItem{ID: hitSFTPMenuEdit, Label: messages.Tools.ExtEditButton, Icon: icon(iconExternal), Disabled: busy})
		}
		rows.separate()
		rows.add(components.ContextMenuItem{ID: hitSFTPMenuRename, Label: messages.SFTPRename, Icon: icon(iconPencil), Disabled: busy})
		rows.add(components.ContextMenuItem{ID: hitSFTPMenuMove, Label: messages.SFTPMove, Icon: icon(iconFolderOpen), Disabled: busy})
		rows.add(components.ContextMenuItem{ID: hitSFTPMenuChmod, Label: messages.SFTPPermissions, Icon: icon(iconLock), Disabled: busy})
		if _, archive := state.selectedArchive(session, view); archive {
			rows.add(components.ContextMenuItem{ID: hitSFTPMenuExtract, Label: messages.Tools.Extract, Icon: icon(iconArchive), Disabled: busy})
		}
		if state.canCompressSFTP(session, view) {
			rows.add(components.ContextMenuItem{ID: hitSFTPMenuCompress, Label: messages.Tools.Compress, Icon: icon(iconArchive), Disabled: busy})
		}
		rows.separate()
	}
	if count > 0 && !local {
		rows.add(components.ContextMenuItem{ID: hitSFTPMenuDelete, Label: messages.SFTPDelete, Icon: icon(iconTrash), Variant: components.ContextMenuItemDestructive, Disabled: busy})
	}
	rows.separate()
	rows.add(components.ContextMenuItem{ID: hitSFTPNewDirectory, Label: messages.SFTPNewDirectory, Icon: icon(iconFolderPlus), Disabled: busy})
	rows.add(components.ContextMenuItem{ID: hitSFTPNewFile, Label: messages.SFTPNewFile, Icon: icon(iconFilePlus), Disabled: busy})
	if !local {
		rows.add(components.ContextMenuItem{ID: hitSFTPUpload, Label: messages.SFTPUpload, Icon: icon(iconUpload), Disabled: busy})
		rows.add(components.ContextMenuItem{ID: hitSFTPUploadDirectory, Label: messages.SFTPUploadDirectory, Icon: icon(iconFolderUp), Disabled: busy})
		rows.add(components.ContextMenuItem{ID: hitSFTPNewLink, Label: messages.Tools.TipNewLink, Icon: icon(iconLink), Disabled: busy})
	}
	if count > 0 {
		rows.add(components.ContextMenuItem{ID: hitSFTPCopyPath, Label: messages.SFTPWindow.CopyPath, Icon: icon(iconCopy)})
	}
	if one {
		rows.add(components.ContextMenuItem{ID: hitSFTPProperties, Label: messages.SFTPWindow.PropertiesTitle, Icon: icon(iconInfo)})
	}
	if sudoSFTPAvailable(session, view) {
		label := messages.SFTPSudoEnable
		if view.sudo {
			label = messages.SFTPSudoDisable
		}
		rows.separate()
		rows.add(components.ContextMenuItem{ID: hitSFTPMenuSudo, Label: label, Icon: icon(iconLock)})
	}

	return rows
}

// clickSFTPMenu runs a menu choice. A press anywhere else only closes the menu.
func (state *State) clickSFTPMenu(id gfx.ID) bool {
	if !state.sftpMenu.open {
		return false
	}
	state.sftpMenu.open = false
	if action, ok := sftpMenuAction(id); ok {
		state.clickSFTP(action)
	}
	return true
}
