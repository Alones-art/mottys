package workbench

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"motty/internal/sftpclient"
)

func TestSFTPDirectoryExists(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"local os error", fmt.Errorf("mkdir %s: %w", "/tmp/demo", os.ErrExist), true},
		{"sftp client sentinel", fmt.Errorf("mkdir: %w", sftpclient.ErrExists), true},
		{"sftp status text", errors.New("sftp: file exists"), true},
		{"server wording", errors.New("SSH_FX_FAILURE: directory already exists"), true},
		{"permission denied", os.ErrPermission, false},
		{"unrelated failure", errors.New("connection reset by peer"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sftpDirectoryExists(tc.err); got != tc.want {
				t.Fatalf("sftpDirectoryExists(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
