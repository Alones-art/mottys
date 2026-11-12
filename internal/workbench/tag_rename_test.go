package workbench

import (
	"testing"

	"github.com/vivaui/viva/text"
)

// The tag manager renames and deletes a tag on every host that carries it.
func TestHostTagSettingsRenameAndDelete(t *testing.T) {
	s := newTestState(t, testOptions(t))
	s.hosts = []Host{
		{ID: 1, Label: "One", Tags: "web, db"},
		{ID: 2, Label: "Two", Tags: "web"},
	}
	// The dialog starts from the tag's current name.
	s.openTagRename("db")
	if s.hostTagEditor.String() != "db" {
		t.Fatalf("the dialog did not start from the old name: %q", s.hostTagEditor.String())
	}
	s.hostTagEditor = text.Editor{}
	s.hostTagEditor.Insert("data")
	s.clickTagRenameDialog(hitTagRenameSubmit)
	if s.hosts[0].Tags != "web, data" || s.hosts[1].Tags != "web" {
		t.Fatalf("rename = %q / %q", s.hosts[0].Tags, s.hosts[1].Tags)
	}
	// Renaming onto a name a host already has merges the two into one.
	s.openTagRename("data")
	s.hostTagEditor = text.Editor{}
	s.hostTagEditor.Insert("web")
	s.clickTagRenameDialog(hitTagRenameSubmit)
	if s.hosts[0].Tags != "web" || s.hosts[1].Tags != "web" {
		t.Fatalf("merge = %q / %q", s.hosts[0].Tags, s.hosts[1].Tags)
	}
	// Deleting arms the button first, then removes the tag everywhere.
	s.clickHostTagDelete("web")
	if s.hostTagArmed != "web" || len(s.hostTagList()) != 1 {
		t.Fatal("the first press must only arm the delete button")
	}
	s.clickHostTagDelete("web")
	if s.hosts[0].Tags != "" || s.hosts[1].Tags != "" || s.hostTagArmed != "" {
		t.Fatalf("delete = %q / %q armed=%q", s.hosts[0].Tags, s.hosts[1].Tags, s.hostTagArmed)
	}
	// An empty name is refused and the dialog stays open.
	s.hosts[0].Tags = "web"
	s.openTagRename("web")
	s.hostTagEditor = text.Editor{}
	s.clickTagRenameDialog(hitTagRenameSubmit)
	if s.dialogError == "" || s.modal != modalTagRename {
		t.Fatal("an empty name must keep the dialog open with an error")
	}
}
