package workbench

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

func groupToolbarHit(name string) gfx.ID {
	return tree.ID(tree.Feature("host-group-toolbar"), name)
}

// flatHostCatalog is the state a new user sees: hosts, but not one named group,
// so no group row exists to right-click for "new child group".
func flatHostCatalog(t *testing.T) *State {
	t.Helper()
	state := newTestState(t, testOptions(t))
	for index := range state.hosts {
		state.hosts[index].Group = ""
	}
	state.hostDataChanged()
	if state.namedHostGroups() != 0 {
		t.Fatalf("fixture starts with %d named groups, want none", state.namedHostGroups())
	}
	return state
}

func hostTreeHostRowCount(state *State) int {
	count := 0
	for _, row := range state.hostTreeRows {
		if row.Kind == hostTreeHost {
			count++
		}
	}
	return count
}

func TestToolbarNewFolderCreatesTheFirstFolderWithoutChangingHosts(t *testing.T) {
	state := flatHostCatalog(t)
	before := append([]Host(nil), state.hosts...)
	state.rebuildHostTreeRows()
	rowsBefore := hostTreeHostRowCount(state)

	state.Click(hitHostNewFolder)
	if state.modal != modalMoveHostGroup || state.groupActions.op != hostGroupCreateChild || state.groupActions.path != "" {
		t.Fatalf("toolbar did not open a root folder dialog: modal=%d op=%d path=%q", state.modal, state.groupActions.op, state.groupActions.path)
	}
	submitHostGroupName(state, "公司内部")
	if state.modal != modalNone || state.dialogError != "" {
		t.Fatalf("creating the first folder left modal=%d error=%q", state.modal, state.dialogError)
	}
	index := state.hostGroupIndex("公司内部")
	if index < 0 || !state.hostGroups[index].Explicit {
		t.Fatalf("new folder is missing or is not an explicit group: %+v", state.hostGroups)
	}
	if !reflect.DeepEqual(state.hosts, before) {
		t.Fatal("creating a folder changed host records")
	}
	if !state.hasHostFilter() {
		t.Fatal("the new folder did not produce a filter chip")
	}
	state.rebuildHostTreeRows()
	if got := hostTreeHostRowCount(state); got != rowsBefore {
		t.Fatalf("host rows changed from %d to %d", rowsBefore, got)
	}
	restored := newTestState(t, state.options)
	if restored.hostGroupIndex("公司内部") < 0 {
		t.Fatal("the new folder was not persisted")
	}
}

func TestToolbarNewFolderWorksInAnEmptyCatalog(t *testing.T) {
	state := flatHostCatalog(t)
	state.hosts = nil
	state.hostDataChanged()
	state.Click(hitHostNewFolder)
	submitHostGroupName(state, "公司内部")
	if state.modal != modalNone || state.dialogError != "" {
		t.Fatalf("empty catalog cannot create a folder: modal=%d error=%q", state.modal, state.dialogError)
	}
	if state.hostGroupIndex("公司内部") < 0 {
		t.Fatalf("folder missing in an empty catalog: %+v", state.hostGroups)
	}
}

func folderCardCatalog(t *testing.T, names ...string) *State {
	t.Helper()
	state := flatHostCatalog(t)
	state.hosts = nil
	state.hostDataChanged()
	for _, name := range names {
		if !state.createHostChildGroup("", name) {
			t.Fatalf("cannot create folder %q: %s", name, state.dialogError)
		}
	}
	state.rebuildHostTreeRows()
	return state
}

func TestHostGridFoldersShareOneGridLine(t *testing.T) {
	state := folderCardCatalog(t, "公司内部", "共享电源", "货币文化")
	if len(state.hostTreeRows) != 3 {
		t.Fatalf("expected three folder rows, got %d", len(state.hostTreeRows))
	}
	columns := hostGridColumns(1000)
	want := float32((3+columns-1)/columns) * (hostCardHeight + hostCardGap)
	if got := state.hostGridHeight(1000); got != want {
		t.Fatalf("grid height %v, want %v (one line of %d folders)", got, want, columns)
	}
	if lines := (3 + columns - 1) / columns; lines != 1 {
		t.Fatalf("three folders took %d grid lines, want one", lines)
	}
}

func TestHostCardLinesSeparateFoldersFromServers(t *testing.T) {
	state := flatHostCatalog(t)
	// A folder, one of its servers, another folder, one of its servers: the
	// old single grid let the server card sit between the two folder cards.
	state.hostTreeRows = []hostTreeRow{
		{Kind: hostTreeGroup, GroupIndex: 0},
		{Kind: hostTreeHost, HostIndex: 0, GroupIndex: 0},
		{Kind: hostTreeGroup, GroupIndex: 1},
		{Kind: hostTreeHost, HostIndex: 1, GroupIndex: 1},
	}
	lines := state.hostCardLines(3)
	if len(lines) != 3 {
		t.Fatalf("got %d card lines, want 3: %+v", len(lines), lines)
	}
	if got := lines[0].entries; !reflect.DeepEqual(got, []int{0, 2}) {
		t.Fatalf("folder line = %v, want both folders side by side", got)
	}
	if got := lines[1].entries; !reflect.DeepEqual(got, []int{1}) {
		t.Fatalf("first folder's servers = %v, want a line of its own", got)
	}
	if got := lines[2].entries; !reflect.DeepEqual(got, []int{3}) {
		t.Fatalf("second folder's servers = %v, want a separate line", got)
	}
	for index, line := range lines {
		kinds := make(map[hostTreeRowKind]bool)
		for _, entry := range line.entries {
			kinds[state.hostTreeRows[entry].Kind] = true
		}
		if len(kinds) != 1 {
			t.Fatalf("line %d mixes folders and hosts", index)
		}
	}
	if got := state.hostGridHeight(1000); got != 3*(hostCardHeight+hostCardGap) {
		t.Fatalf("grid height %v, want three lines", got)
	}
}

func TestHostGroupCardKeepsGroupControls(t *testing.T) {
	state := folderCardCatalog(t, "公司内部")
	state.hostView = hostViewGrid
	index := state.hostGroupIndex("公司内部")
	canvas, _ := paintedTexts(state)
	for _, id := range []gfx.ID{
		hitHostGroupOpenList.At(index),
		hitHostGroupCheck.At(index),
		hitHostGroupMore.At(index),
	} {
		if _, ok := hitByID(canvas, id); !ok {
			t.Fatalf("folder card is missing %s", tree.Text(id))
		}
	}
	before := state.hostGroups[index].Expanded
	state.Click(hitHostGroupOpenList.At(index))
	if state.hostGroups[index].Expanded == before {
		t.Fatal("clicking a folder card did not toggle it")
	}
}

func TestHostGroupToolbarExpandCollapseAllPersistsNestedGroups(t *testing.T) {
	for _, mode := range []hostViewMode{hostViewList, hostViewGrid} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			state := nestedHostGroupState(t)
			state.hostView = mode
			state.toggleHostGroup(state.hostGroupIndex("Team/Prod"))
			state.Click(groupToolbarHit("collapse-all"))
			for _, group := range state.hostGroups {
				if group.Expanded {
					t.Fatalf("collapse all left %q expanded", group.Name)
				}
			}
			if len(state.hostTreeRows) != 2 {
				t.Fatalf("collapsed tree has %d rows, want two roots", len(state.hostTreeRows))
			}
			restored := newTestState(t, state.options)
			for _, group := range restored.hostGroups {
				if group.Expanded {
					t.Fatal("collapse all was not persisted")
				}
			}
			state.Click(groupToolbarHit("expand-all"))
			for _, group := range state.hostGroups {
				if !group.Expanded {
					t.Fatalf("expand all left %q collapsed", group.Name)
				}
			}
			if len(state.hostTreeRows) != 8 {
				t.Fatalf("expanded tree has %d rows, want five groups and three hosts", len(state.hostTreeRows))
			}
			state.Click(groupToolbarHit("expand-all"))
			if len(state.hostTreeRows) != 8 {
				t.Fatal("repeated expand changed the tree")
			}
		})
	}
}

func TestHostGroupToolbarSearchAndAvailability(t *testing.T) {
	state := nestedHostGroupState(t)
	typeSearch(state, state.hosts[0].Label)
	state.Click(groupToolbarHit("collapse-all"))
	if len(state.hostTreeRows) != 4 {
		t.Fatal("bulk collapse hid the search match or its ancestors")
	}
	state.search = text.Editor{}
	state.syncVisible()
	if len(state.hostTreeRows) != 2 {
		t.Fatal("clearing search did not restore bulk-collapsed state")
	}
	state.setHostFilter(true, "Team")
	canvas, _ := paintedTexts(state)
	if _, ok := hitByID(canvas, groupToolbarHit("expand-all")); ok {
		t.Fatal("flat group filter offers group-tree controls")
	}
	state.Click(groupToolbarHit("expand-all"))
	for _, group := range state.hostGroups {
		if group.Expanded {
			t.Fatal("hidden expand action changed filtered group state")
		}
	}
	state.setHostFilter(false, "")
	for index := range state.hosts {
		state.hosts[index].Group = ""
	}
	state.hostDataChanged()
	canvas, _ = paintedTexts(state)
	if _, ok := hitByID(canvas, groupToolbarHit("collapse-all")); ok {
		t.Fatal("unnamed-only hosts offer group controls")
	}
}

func TestHostGroupToolbarSelectionKeepsFiltersAndAddsOnlyOneRow(t *testing.T) {
	for _, mode := range []hostViewMode{hostViewList, hostViewGrid} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			state := newTestState(t, testOptions(t))
			state.hostView = mode
			size := gfx.Size{Width: 960, Height: 600}
			paintAt(state, size)
			listTop := state.chrome.hostList.Y
			first, second := state.hosts[0].ID, state.hosts[1].ID
			state.Click(hitHostCheckList.At(int(first)))
			for _, count := range []int{1, 2} {
				if count == 2 {
					state.Click(hitHostCheckList.At(int(second)))
				}
				canvas, texts := paintAt(state, size)
				filter, ok := hitByID(canvas, hitHostFilterAll)
				if !ok {
					t.Fatal("checking hosts removed the group filters")
				}
				if !contains(texts, state.hostSelectionText) {
					t.Fatal("selection count is absent")
				}
				if state.chrome.hostList.Y <= listTop {
					t.Fatal("selection toolbar did not reserve its own row")
				}
				for _, id := range []gfx.ID{hitExportHosts, hitEditHost, hitCopyHost, hitMoveHostGroup, hitDashboardAddHosts, hitDeleteHost} {
					hit, ok := hitByID(canvas, id)
					if !ok || hit.Bounds.Y < filter.Bounds.Y+filter.Bounds.Height || hit.Bounds.Y+hit.Bounds.Height > state.chrome.hostList.Y {
						t.Fatalf("batch action %s overlaps filters or host list: %+v", tree.Text(id), hit)
					}
					if (id == hitEditHost || id == hitCopyHost) && hit.Disabled != (count > 1) {
						t.Fatal("single-host action enablement regressed")
					}
				}
				if count == 1 {
					oneRowTop := state.chrome.hostList.Y
					state.Click(hitHostCheckList.At(int(second)))
					paintAt(state, size)
					if state.chrome.hostList.Y != oneRowTop {
						t.Fatal("another check added another toolbar row")
					}
					state.Click(hitHostCheckList.At(int(second)))
				}
			}
			state.Click(hitHostCheckList.At(int(first)))
			state.Click(hitHostCheckList.At(int(second)))
			canvas, _ := paintAt(state, size)
			if state.chrome.hostList.Y != listTop {
				t.Fatal("clearing checks did not restore the list viewport")
			}
			if _, ok := hitByID(canvas, hitDeleteHost); ok {
				t.Fatal("empty selection still shows batch actions")
			}
		})
	}
}
