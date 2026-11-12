package workbench

// syncDesktopIME keeps the local input method out of a remote desktop session.
// A remote desktop runs its own IME; leaving this machine's IME on sends a
// composition window and preedit text into the remote session and can swallow
// keystrokes. Only the focused desktop tab turns it off: switching back to a
// terminal tab, to another section, or minimizing the window restores it.
func (state *State) syncDesktopIME() {
	active := false
	if state.activeSection == sectionTerminal {
		for index := range state.sessions {
			session := &state.sessions[index]
			view := session.desk
			if view != nil && !view.ended && view.active && session.ID == state.activeSessionID {
				active = true
				break
			}
		}
	}
	setLocalIMEEnabled(!active)
}
