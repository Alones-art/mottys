package workbench

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"motty/internal/sftpclient"
)

// sftpDirectoryExists reports whether a failed mkdir means the name is taken.
// Local filesystems answer with os.ErrExist, while SFTP servers answer with a
// status error whose text is not a typed Go error ("file exists", "already
// exists", or a localized equivalent of those). Both sides must be readable to
// the user instead of leaking the backend's wording.
func sftpDirectoryExists(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrExist) || errors.Is(err, sftpclient.ErrExists) {
		return true
	}
	message := strings.ToLower(err.Error())
	for _, phrase := range []string{"file exists", "already exists", "file already exists", "directory exists"} {
		if strings.Contains(message, phrase) {
			return true
		}
	}
	return false
}

// sftpCreateDirectoryErrorText turns a failed mkdir into one readable line.
// name is the name the user typed (or the directory the browser pair tried to
// create); it keeps the message short enough for the dialog.
func (state *State) sftpCreateDirectoryErrorText(name string, err error) string {
	if sftpDirectoryExists(err) {
		return fmt.Sprintf(state.messages.SFTPDirectoryExistsFmt, name)
	}
	return fmt.Sprintf(state.messages.SFTPErrorFmt, err)
}
