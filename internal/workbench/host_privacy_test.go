//go:build darwin || windows || linux

package workbench

import (
	"strings"
	"testing"
)

// Privacy mode masks every word a viewer could read off the host list, on the
// cards and in the connect menu, without changing the host itself.
func TestPrivacyModeMasksHostDetails(t *testing.T) {
	state := manyHostsState(t, 1)
	host := Host{Label: "secret-db", Address: "10.0.0.7", User: "root", Port: 2222}
	host.finish()

	leaks := func() bool {
		for _, text := range paintHostCardTexts(t, state, host, 420) {
			if strings.Contains(text.Value, host.Label) || strings.Contains(text.Value, host.Address) {
				return true
			}
		}
		return false
	}
	if !leaks() {
		t.Fatal("the card does not show the host name before privacy mode")
	}

	state.Click(hitSettings)
	state.Click(hitSettingsPrivacyMode)
	if !state.privacyMode {
		t.Fatal("the settings switch did not turn privacy mode on")
	}
	if leaks() {
		t.Fatal("the card still shows the host name or address")
	}
	masked := false
	for _, text := range paintHostCardTexts(t, state, host, 420) {
		if text.Value == privacyMask {
			masked = true
		}
	}
	if !masked {
		t.Fatal("the card shows no mask at all")
	}

	state.eachConnectPickerCandidate(func(candidate connectPickerCandidate) {
		if candidate.label == host.Label || candidate.detail == host.Address {
			t.Fatal("the connect menu still shows the host name or address")
		}
	})
	if host.Label != "secret-db" || host.Address != "10.0.0.7" {
		t.Fatal("privacy mode changed the host data itself")
	}
}
