package workbench

import "testing"

// The interactive-only switch must survive the whole form -> host -> saved file
// -> host round trip, or a gateway loses the setting on the next launch and
// gets its connection dropped by a probe command again.
func TestInteractiveOnlyHostRoundTrip(t *testing.T) {
	var editor hostEditor
	editor.load(Host{Protocol: protocolSSH, InteractiveOnly: true})
	if !editor.interactiveOnly {
		t.Fatal("the form did not restore the switch")
	}
	if !editor.advancedOpen {
		t.Fatal("a host with the switch on must open its advanced section")
	}

	host := editor.host(7)
	if !host.InteractiveOnly {
		t.Fatal("the form did not write the switch into the host")
	}
	saved := host.transport()
	if !saved.InteractiveOnly {
		t.Fatal("the switch was not written to the saved transport")
	}
	if sameSavedHost(savedHost{savedTransport: saved}, savedHost{savedTransport: host.transport()}) == false {
		t.Fatal("two identical interactive-only hosts compare as different")
	}
	without := host.transport()
	without.InteractiveOnly = false
	if sameSavedHost(savedHost{savedTransport: saved}, savedHost{savedTransport: without}) {
		t.Fatal("the switch does not take part in saved-host comparison")
	}

	var restored Host
	if !restored.applyTransport(saved) {
		t.Fatal("the saved transport did not restore")
	}
	if !restored.InteractiveOnly {
		t.Fatal("the switch was lost on restore")
	}

	editor.interactiveOnly = false
	if editor.host(7).InteractiveOnly {
		t.Fatal("turning the switch off did not clear the host")
	}
}
