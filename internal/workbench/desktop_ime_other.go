//go:build !windows

package workbench

// Other platforms keep the composition events their own input source delivers;
// only Windows needs the window-level input context detached.
func setLocalIMEEnabled(bool) {}
