package sshconn

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
)

// A known_hosts line that the strict parser cannot read stops the whole file:
// knownhosts.New parses every line before it returns a callback, so one NUL
// padded or truncated line makes every SSH connection fail with "missing host
// pattern". That damage is usually a write interrupted by a power loss, a sync
// tool's placeholder or an editor that extended the file without filling it.
//
// This repair only removes lines that cannot produce a public key. It never
// removes a usable key and never relaxes host key verification: a changed
// server key is still reported by the callback as a key mismatch.

// repairKnownHosts removes unreadable lines from the app-owned known_hosts
// file. It reports whether it changed anything. A clean file is left byte for
// byte alone, so a normal connection costs no extra write.
func repairKnownHosts(path string) (bool, error) {
	original, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	cleaned, removed := cleanKnownHosts(original)
	if removed == 0 {
		return false, nil
	}
	// Keep the damaged file once, so the lost records can be inspected. Do not
	// overwrite a backup that is already there: it may hold an earlier, more
	// complete version of the file.
	backup := path + ".bak"
	if _, err := os.Stat(backup); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(backup, original, 0o600); err != nil {
			return false, fmt.Errorf("sshconn: back up known hosts: %w", err)
		}
	} else if err != nil {
		return false, fmt.Errorf("sshconn: check known hosts backup: %w", err)
	}
	if err := replaceKnownHosts(path, cleaned); err != nil {
		return false, err
	}
	return true, nil
}

// cleanKnownHosts returns the file with lines that carry no usable host key
// dropped, and how many lines it dropped. Comments, blank lines, markers such
// as @cert-authority and hashed host names are all kept.
func cleanKnownHosts(data []byte) ([]byte, int) {
	var kept []byte
	removed := 0
	for len(data) > 0 {
		line := data
		if end := bytes.IndexByte(data, '\n'); end >= 0 {
			line, data = data[:end+1], data[end+1:]
		} else {
			data = nil
		}
		body := bytes.TrimSuffix(bytes.TrimSuffix(line, []byte("\n")), []byte("\r"))
		if knownHostsLineUsable(body) {
			kept = append(kept, line...)
		} else {
			removed++
		}
	}
	return kept, removed
}

// knownHostsLineUsable reports whether a line yields a public key. A line the
// strict parser would reject (a NUL tail, a truncated record, a bad base64
// blob) yields none. Blank lines and comments are legal file content and are
// kept, even though they carry no key.
func knownHostsLineUsable(line []byte) bool {
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) == 0 || trimmed[0] == '#' {
		return true
	}
	// ParseKnownHosts skips lines with no whitespace and returns io.EOF at the
	// end of a buffer with no usable record: both mean "no key here".
	_, _, key, _, _, err := ssh.ParseKnownHosts(append(append([]byte(nil), trimmed...), '\n'))
	return err == nil && key != nil
}

// replaceKnownHosts writes the cleaned file atomically, with the same 0600
// permissions the app gives the original, so a crash cannot leave a half file.
func replaceKnownHosts(path string, data []byte) error {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".known_hosts-repair-*")
	if err != nil {
		return fmt.Errorf("sshconn: create known hosts repair file: %w", err)
	}
	name := temporary.Name()
	defer os.Remove(name)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("sshconn: set known hosts permissions: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return fmt.Errorf("sshconn: write known hosts repair: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sshconn: sync known hosts repair: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("sshconn: close known hosts repair: %w", err)
	}
	if err := os.Rename(name, path); err != nil {
		return fmt.Errorf("sshconn: replace known hosts: %w", err)
	}
	return nil
}
