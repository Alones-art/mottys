package sshconn

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func testHostKeyLine(t *testing.T, host string) (string, ssh.PublicKey) {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	return knownhosts.Line([]string{host}, signer.PublicKey()), signer.PublicKey()
}

// A NUL padded tail is what an interrupted write leaves: one unusable line at
// the end of an otherwise valid file. It must not take every connection down.
func TestRepairKnownHostsDropsNULTail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "known_hosts")
	first, _ := testHostKeyLine(t, "192.0.2.1")
	second, _ := testHostKeyLine(t, "192.0.2.2")
	original := append([]byte(first+"\n"+second+"\n"), bytes.Repeat([]byte{0}, 182)...)
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := knownhosts.New(path); err == nil {
		t.Fatal("the damaged file parsed without an error")
	}

	repaired, err := repairKnownHosts(path)
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	if !repaired {
		t.Fatal("the damaged file was left alone")
	}
	backup, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("no backup was kept: %v", err)
	}
	if !bytes.Equal(backup, original) {
		t.Fatal("the backup does not hold the original bytes")
	}
	cleaned, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(cleaned, []byte{0}) {
		t.Fatal("the NUL tail is still in the file")
	}
	if !bytes.Contains(cleaned, []byte("192.0.2.1")) || !bytes.Contains(cleaned, []byte("192.0.2.2")) {
		t.Fatal("a good host key was dropped with the bad line")
	}
	if _, err := knownhosts.New(path); err != nil {
		t.Fatalf("the repaired file still fails to parse: %v", err)
	}
	// A second repair must be a no-op and must not overwrite the backup.
	if again, err := repairKnownHosts(path); err != nil || again {
		t.Fatalf("a clean file was rewritten: again=%v err=%v", again, err)
	}
	if backupAgain, _ := os.ReadFile(path + ".bak"); !bytes.Equal(backupAgain, original) {
		t.Fatal("the backup was overwritten on the second repair")
	}
}

func TestRepairKnownHostsLeavesCleanFileAlone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "known_hosts")
	line, _ := testHostKeyLine(t, "192.0.2.1")
	original := []byte("# trusted hosts\n" + line + "\n\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	repaired, err := repairKnownHosts(path)
	if err != nil || repaired {
		t.Fatalf("a clean file was touched: repaired=%v err=%v", repaired, err)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, original) {
		t.Fatal("a clean file was rewritten")
	}
	if _, err := os.Stat(path + ".bak"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a backup was made for an undamaged file")
	}
}

// Self-healing must not weaken host key verification: a different key for a
// known host is still a key mismatch, not a silently accepted connection.
func TestNewHostKeysSurvivesDamageButStillRejectsChangedKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "known_hosts")
	line, trusted := testHostKeyLine(t, "192.0.2.1")
	_, other := testHostKeyLine(t, "192.0.2.9")
	if err := os.WriteFile(path, append([]byte(line+"\n"), bytes.Repeat([]byte{0}, 182)...), 0o600); err != nil {
		t.Fatal(err)
	}
	keys, err := newHostKeys(path)
	if err != nil {
		t.Fatalf("a damaged file still blocks connections: %v", err)
	}
	remote := &net.TCPAddr{IP: net.ParseIP("192.0.2.1"), Port: 22}
	if err := keys.callback("192.0.2.1:22", remote, trusted); err != nil {
		t.Fatalf("the trusted key was refused: %v", err)
	}
	err = keys.callback("192.0.2.1:22", remote, other)
	var mismatch *knownhosts.KeyError
	if err == nil || !errors.As(err, &mismatch) {
		t.Fatalf("a changed key was not reported as a key mismatch: %v", err)
	}
}
