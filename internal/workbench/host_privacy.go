package workbench

// Privacy mode masks host names and connection details while the window is on
// show, for a demo or a recording. It is a view-only, session-level switch:
// it is never saved, so a restart always shows the real names again. The host
// data itself is untouched; connecting still uses the real address.
const privacyMask = "••••••"

var hitSettingsPrivacyMode = tree.ID(settingsHits, "privacy-mode")

func (state *State) hostDisplayName(host Host) string {
	if state.privacyMode {
		return privacyMask
	}
	return host.Label
}

func (state *State) hostConnectionText(host Host) string {
	if state.privacyMode {
		return privacyMask
	}
	return state.hostCardConnection(host)
}

func (state *State) hostAddressText(host Host) string {
	if state.privacyMode {
		return privacyMask
	}
	return host.target
}

func (state *State) hostSubtitleText(host Host) string {
	if state.privacyMode {
		return privacyMask
	}
	return state.hostSubtitle(host)
}

func (state *State) hostTagText(tag string) string {
	if state.privacyMode {
		return privacyMask
	}
	return tag
}

func (state *State) hostAddressForPicker(host Host) string {
	if state.privacyMode {
		return privacyMask
	}
	return state.sessionAddress(host)
}
