package workbench

import "testing"

// Hiding local shells is a menu-only setting: the connect picker loses its
// Local Terminal row, and the choice survives a restart.
func TestHideLocalShellsLeavesConnectMenu(t *testing.T) {
	state := newTestState(t, testOptions(t))
	localRows := func() int {
		count := 0
		state.eachConnectPickerCandidate(func(candidate connectPickerCandidate) {
			if candidate.target == hitConnectPickLocal {
				count++
			}
		})
		return count
	}
	if localRows() != 1 {
		t.Fatalf("the connect menu should offer one local terminal, got %d", localRows())
	}

	state.Click(hitSettings)
	state.Click(hitSettingsHideLocalShells)
	if !state.hideLocalShells {
		t.Fatal("the settings switch did not turn hiding on")
	}
	if localRows() != 0 {
		t.Fatal("the connect menu still lists a local terminal")
	}

	state.save()
	restored := newTestState(t, state.options)
	if !restored.hideLocalShells {
		t.Fatal("the setting was lost on restore")
	}

	restored.hideLocalShells = false
	if got := func() int {
		count := 0
		restored.eachConnectPickerCandidate(func(candidate connectPickerCandidate) {
			if candidate.target == hitConnectPickLocal {
				count++
			}
		})
		return count
	}(); got != 1 {
		t.Fatalf("turning the switch off did not restore the row: %d", got)
	}
}
