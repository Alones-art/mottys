package sftpclient

import (
	"errors"
	"io"
	"os"
	"testing"
)

// createFileRemote answers transfers with a set of existing paths, so a
// create-file request can be checked against an occupied directory.
type createFileRemote struct {
	*fakeRemote
	exists map[string]bool
	empty  string
	emptyE error
}

func (remote *createFileRemote) Lstat(path string) (os.FileInfo, error) {
	if remote.exists[path] {
		return fakeInfo{name: path, size: 0}, nil
	}
	return nil, os.ErrNotExist
}

func (remote *createFileRemote) OpenTransferFile(path string, flags int) (TransferFile, error) {
	if flags&os.O_EXCL != 0 && remote.exists[path] {
		return nil, os.ErrExist
	}
	if path == remote.empty {
		return nil, remote.emptyE
	}
	remote.exists[path] = true
	return &createFileHandle{}, nil
}

func (remote *createFileRemote) Mkdir(string) error               { return os.ErrInvalid }
func (remote *createFileRemote) Rename(string, string) error      { return os.ErrInvalid }
func (remote *createFileRemote) PosixRename(string, string) error { return os.ErrInvalid }
func (remote *createFileRemote) Remove(string) error              { return os.ErrInvalid }
func (remote *createFileRemote) Chmod(string, os.FileMode) error  { return os.ErrInvalid }

type createFileHandle struct {
	offset int64
}

func (file *createFileHandle) Read(destination []byte) (int, error) { return 0, io.EOF }
func (file *createFileHandle) Write(data []byte) (int, error)       { return len(data), nil }
func (file *createFileHandle) Seek(offset int64, whence int) (int64, error) {
	file.offset = offset
	return offset, nil
}
func (file *createFileHandle) Close() error               { return nil }
func (file *createFileHandle) Stat() (os.FileInfo, error) { return fakeInfo{name: "new"}, nil }
func (file *createFileHandle) Sync() error                { return nil }
func (file *createFileHandle) Truncate(size int64) error  { return nil }

func TestCreateFileMakesAnEmptyFileAndKeepsExistingOnes(t *testing.T) {
	remote := &createFileRemote{fakeRemote: &fakeRemote{closed: make(chan struct{})}, exists: map[string]bool{"/srv/notes.txt": true}}
	worker := NewWorker(func() (Remote, error) { return remote, nil }, nil)
	defer worker.Close()

	result := requireQueuedResult(t, worker.CreateFile("/srv/new.md"), worker, OperationCreateFile)
	if result.Err != nil || result.Path != "/srv/new.md" {
		t.Fatalf("creation failed: %+v", result)
	}
	if !remote.exists["/srv/new.md"] {
		t.Fatal("the new file was not recorded on the remote")
	}

	// An existing file is left alone.
	result = requireQueuedResult(t, worker.CreateFile("/srv/notes.txt"), worker, OperationCreateFile)
	if !errors.Is(result.Err, ErrExists) {
		t.Fatalf("existing file error = %v, want ErrExists", result.Err)
	}
}

func TestCreateFileReportsRefusalsFromTheBackend(t *testing.T) {
	failure := os.ErrPermission
	remote := &createFileRemote{fakeRemote: &fakeRemote{closed: make(chan struct{})}, exists: map[string]bool{}, empty: "/srv/denied.txt", emptyE: failure}
	worker := NewWorker(func() (Remote, error) { return remote, nil }, nil)
	defer worker.Close()

	result := requireQueuedResult(t, worker.CreateFile("/srv/denied.txt"), worker, OperationCreateFile)
	if !errors.Is(result.Err, failure) {
		t.Fatalf("backend error = %v, want %v", result.Err, failure)
	}
}
