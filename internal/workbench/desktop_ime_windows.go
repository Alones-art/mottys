//go:build windows

package workbench

import (
	"sync"

	"golang.org/x/sys/windows"
)

// immAssociateContext is the one Windows call needed here: it detaches the
// input context from a window and returns the previous one, which is stored so
// the same context can be attached again. Using the returned handle avoids a
// separate ImmGetContext/ImmReleaseContext pair that could race a repaint.
var immAssociateContext = windows.NewLazySystemDLL("imm32.dll").NewProc("ImmAssociateContext")

var localIME struct {
	sync.Mutex
	disabled bool
	hwnd     windows.HWND
	previous uintptr
}

// foregroundOwnWindow returns the foreground window only when it belongs to
// this process. Repaints can happen from headless tests or while another
// application is in front; its input method must never be touched.
func foregroundOwnWindow() windows.HWND {
	hwnd := windows.GetForegroundWindow()
	if hwnd == 0 {
		return 0
	}
	var pid uint32
	windows.GetWindowThreadProcessId(hwnd, &pid)
	if pid != windows.GetCurrentProcessId() {
		return 0
	}
	return hwnd
}

func restoreIME(hwnd windows.HWND, previous uintptr) {
	if hwnd != 0 && windows.IsWindow(hwnd) {
		immAssociateContext.Call(uintptr(hwnd), previous)
	}
}

// setLocalIMEEnabled disables this window's input method when enabled is false,
// and restores the exact context it had before. Repeated calls are cheap: the
// state is checked before any system call, so a repaint does not churn the IME.
func setLocalIMEEnabled(enabled bool) {
	localIME.Lock()
	defer localIME.Unlock()

	if enabled {
		if !localIME.disabled {
			return
		}
		restoreIME(localIME.hwnd, localIME.previous)
		localIME.disabled, localIME.hwnd, localIME.previous = false, 0, 0
		return
	}

	hwnd := foregroundOwnWindow()
	if hwnd == 0 {
		// The app is not in front. Leave the IME alone; when it comes back a
		// later repaint will disable it again.
		return
	}
	if localIME.disabled {
		if hwnd == localIME.hwnd {
			return
		}
		// A different window of this process took focus: restore the old one
		// before disabling the new one, or its saved context is lost.
		restoreIME(localIME.hwnd, localIME.previous)
	}
	previous, _, _ := immAssociateContext.Call(uintptr(hwnd), 0)
	localIME.disabled, localIME.hwnd, localIME.previous = true, hwnd, previous
}
