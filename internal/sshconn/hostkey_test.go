package sshconn

import (
	"net"
	"testing"
)

// A link-local remote whose Windows zone carries brackets parses after
// parseableRemote, and a remote that already parses passes through untouched.
func TestParseableRemoteSurvivesABracketedZone(t *testing.T) {
	linkLocal := &net.TCPAddr{IP: net.ParseIP("fe80::1234"), Port: 22,
		Zone: `Example VPN Adapter [0123456789abcdef]`}
	if _, _, err := net.SplitHostPort(linkLocal.String()); err == nil {
		t.Skip("this system writes the zone in a form that already parses")
	}
	got := parseableRemote(linkLocal)
	if _, _, err := net.SplitHostPort(got.String()); err != nil {
		t.Fatalf("the knownhosts check still cannot parse the remote: %v", err)
	}
	plain := &net.TCPAddr{IP: net.ParseIP("192.168.1.1"), Port: 22}
	if parseableRemote(plain) != net.Addr(plain) {
		t.Fatal("a plain remote was rebuilt")
	}
}
