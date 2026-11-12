package workbench

import (
	"fmt"
	"slices"
	"strings"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

var hostTagHits = tree.Feature("settings-host-tags")

var (
	hitSettingsHostTagRename = tree.Sub(hostTagHits, tree.Name("rename"))
	hitSettingsHostTagDelete = tree.Sub(hostTagHits, tree.Name("delete"))
	hitTagRenameField        = tree.ID(hostTagHits, "rename-field")
	hitTagRenameSubmit       = tree.ID(hostTagHits, "rename-submit")
	hitTagRenameCancel       = tree.ID(hostTagHits, "rename-cancel")
	hitTagRenameClose        = tree.ID(hostTagHits, "rename-close")
)

// hostTagEntry is one distinct tag and how many hosts carry it.
type hostTagEntry struct {
	tag   string
	hosts int
}

// hostTagList collects the distinct tags across the hosts, alphabetically.
func (state *State) hostTagList() []hostTagEntry {
	counts := map[string]int{}
	for _, host := range state.hosts {
		for _, tag := range splitHostTags(host) {
			counts[tag]++
		}
	}
	list := make([]hostTagEntry, 0, len(counts))
	for tag, count := range counts {
		list = append(list, hostTagEntry{tag: tag, hosts: count})
	}
	slices.SortFunc(list, func(a, b hostTagEntry) int { return strings.Compare(a.tag, b.tag) })
	return list
}

// renameHostTag renames a tag on every host that carries it. A host ends up
// with the tag once: renaming onto a name the host already has merges the two.
func (state *State) renameHostTag(from, to string) {
	if from == "" || to == "" || from == to {
		return
	}
	for index := range state.hosts {
		tags := splitHostTags(state.hosts[index])
		changed := false
		for at, tag := range tags {
			if tag == from {
				tags[at], changed = to, true
			}
		}
		if !changed {
			continue
		}
		uniq := make([]string, 0, len(tags))
		for _, tag := range tags {
			if !slices.Contains(uniq, tag) {
				uniq = append(uniq, tag)
			}
		}
		setHostTags(&state.hosts[index], uniq)
	}
}

// deleteHostTag removes a tag from every host that carries it.
func (state *State) deleteHostTag(tag string) {
	for index := range state.hosts {
		tags := splitHostTags(state.hosts[index])
		if !slices.Contains(tags, tag) {
			continue
		}
		kept := slices.DeleteFunc(slices.Clone(tags), func(value string) bool { return value == tag })
		setHostTags(&state.hosts[index], kept)
	}
	state.hostTagArmed = ""
	state.save()
}

// clickHostTagDelete deletes on the second press: the first one arms the
// button, since the tag leaves every host that uses it at once.
func (state *State) clickHostTagDelete(tag string) {
	if state.hostTagArmed != tag {
		state.hostTagArmed = tag
		return
	}
	state.deleteHostTag(tag)
}

// openTagRename asks for a new name for a tag, starting from its current one.
func (state *State) openTagRename(tag string) {
	state.hostTagRenaming = tag
	state.hostTagEditor = text.Editor{}
	state.hostTagEditor.Insert(tag)
	state.hostTagEditor.SetSelection(text.Range{End: state.hostTagEditor.RuneCount()})
	state.dialogError = ""
	state.modal = modalTagRename
}

func (state *State) clickTagRenameDialog(id gfx.ID) {
	switch id {
	case hitTagRenameClose, hitTagRenameCancel:
		state.modal = modalNone
		state.dialogError = ""
		state.hostTagRenaming = ""
	case hitTagRenameSubmit:
		to := strings.TrimSpace(state.hostTagEditor.String())
		if to == "" {
			state.dialogError = state.messages.HostTagsNameRequired
			return
		}
		from := state.hostTagRenaming
		state.modal = modalNone
		state.dialogError = ""
		state.hostTagRenaming = ""
		state.hostTagArmed = ""
		state.renameHostTag(from, to)
		state.save()
	}
}

// tagRenameKey makes Enter in the field, or on the button, submit.
func (state *State) tagRenameKey(event gfx.KeyEvent) (gfx.EventResult, bool) {
	if state.modal != modalTagRename || event.Key != gfx.KeyEnter || event.Repeat {
		return gfx.EventResult{}, false
	}
	if event.Target != hitTagRenameField && event.Target != hitTagRenameSubmit {
		return gfx.EventResult{}, false
	}
	state.clickTagRenameDialog(hitTagRenameSubmit)
	return gfx.EventResult{Handled: true, Repaint: true}, true
}

func (state *State) paintTagRenameDialog(canvas *gfx.Canvas, viewport gfx.Rect) {
	m := state.messages
	// No scrim dismiss: a press beside the form must not discard its input.
	dialog := components.BeginDialog(canvas, state.Theme(), viewport, components.DialogOptions{
		Layer: dialogLayer, Scope: dialogScope,
		CloseID: hitTagRenameClose, InitialFocusID: hitTagRenameField,
		ContentSize: gfx.Size{Width: 460}, FitBody: true, BodyHeight: 62,
	})
	layout := dialog.Begin(m.HostTagsRenameTitle, "")
	var slot ui.Box
	f := state.chrome.dialog.Begin(canvas, state.Theme(), layout.Body)
	f.Column().Fill().Children(func() { slot = f.Slot().Height(62) })
	f.End()
	field := components.BeginField(canvas, state.Theme(), slot.Rect())
	if state.dialogError != "" {
		field = field.Invalid()
	}
	content := field.Paint(m.HostTagsRenameField, "", state.dialogError)
	value := state.hostTagEditor.String()
	focused := canvas.Focused() == hitTagRenameField
	content.Height = 32
	components.PaintInput(canvas, state.Theme(), content, components.InputSnapshot{
		ID: hitTagRenameField, Label: m.HostTagsRenameField, Value: value,
		Placeholder: state.hostTagRenaming, Focused: focused, PlatformEditing: focused && value != "",
	})
	dialog.Actions(layout,
		components.ButtonAction{ID: hitTagRenameCancel, Label: state.messages.Cancel, Variant: components.ButtonSecondary},
		components.ButtonAction{ID: hitTagRenameSubmit, Label: m.HostTagsRenameSubmit},
	)
	dialog.End()
}

// paintHostTagsSettings lists the distinct tags with the host count and the
// rename and delete actions, inside the connection settings page.
func (state *State) paintHostTagsSettings(page *settingsPage) {
	m := state.messages
	tags := state.hostTagList()
	if len(tags) == 0 {
		page.actionRow(m.HostTagsSettingsEmpty, "", true)
		return
	}
	for _, entry := range tags {
		deleteLabel := m.HostTagsDelete
		if state.hostTagArmed == entry.tag {
			deleteLabel = m.HostTagsDeleteArmed
		}
		page.actionRow(entry.tag, fmt.Sprintf(m.HostTagsCountFmt, entry.hosts), false,
			components.ButtonAction{ID: hitSettingsHostTagRename.At(indexOfHostTag(tags, entry.tag)), Label: m.HostTagsRenameSubmit, Variant: components.ButtonOutline},
			components.ButtonAction{ID: hitSettingsHostTagDelete.At(indexOfHostTag(tags, entry.tag)), Label: deleteLabel, Variant: components.ButtonDestructive},
		)
	}
}

func indexOfHostTag(tags []hostTagEntry, tag string) int {
	for index, entry := range tags {
		if entry.tag == tag {
			return index
		}
	}
	return 0
}
