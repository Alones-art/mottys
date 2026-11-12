package selfupdate

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// applyMac swaps the new bundle in beside the old one so the swap is two
// renames on one volume, and puts the old one back if the second fails.
func (job Job) applyMac() error {
	parent := filepath.Dir(job.Root)
	next := filepath.Join(parent, "."+filepath.Base(job.Root)+".new")
	old := filepath.Join(parent, "."+filepath.Base(job.Root)+".old")
	os.RemoveAll(next)
	os.RemoveAll(old)
	if err := moveTree(job.Staged, next); err != nil {
		os.RemoveAll(next)
		return fmt.Errorf("placing the new version: %w", err)
	}
	if err := os.Rename(job.Root, old); err != nil {
		os.RemoveAll(next)
		return fmt.Errorf("setting the old version aside: %w", err)
	}
	if err := os.Rename(next, job.Root); err != nil {
		rollback := os.Rename(old, job.Root)
		os.RemoveAll(next)
		return errors.Join(fmt.Errorf("installing the new version: %w", err), rollback)
	}
	os.RemoveAll(old)
	return nil
}

// moveTree renames src to dst, or copies it when they are on different volumes.
func moveTree(src, dst string) error {
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}
	if !errors.Is(err, syscall.EXDEV) {
		return err
	}
	if err := copyTree(src, dst); err != nil {
		return err
	}
	return os.RemoveAll(src)
}

// copyTree copies directories and ordinary files, keeping permissions.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, relative)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		switch {
		case entry.IsDir():
			return os.MkdirAll(target, info.Mode().Perm())
		case info.Mode().IsRegular():
			in, err := os.Open(path)
			if err != nil {
				return err
			}
			defer in.Close()
			out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
			if err != nil {
				return err
			}
			_, err = io.Copy(out, in)
			if closeErr := out.Close(); err == nil {
				err = closeErr
			}
			return err
		}
		return fmt.Errorf("not a file: %s", path)
	})
}

// applyWindows runs the installer silently in the detected installation's exact
// directory. The helper has already waited for the running program to quit.
func (job Job) applyWindows() error {
	return replaceWindowsExecutable(job.Root, filepath.Base(job.Exe), job.runWindowsInstaller)
}

func (job Job) runWindowsInstaller() error {
	// installerCommandLine builds the whole command line: Go's argument quoting
	// would wrap /D= and NSIS would then install to the default directory.
	cmd := exec.Command(job.Staged)
	hide(cmd)
	installerCommandLine(cmd, job.Root)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("the installer failed: %w %s", err, firstLine(string(output)))
	}
	return nil
}

// applyLinux runs the archive's own install.sh over the existing prefix; the
// script copies the program beside its target and renames it into place.
func (job Job) applyLinux() error {
	cmd := exec.Command("/bin/sh", filepath.Join(job.Staged, "install.sh"), "install", "--prefix", job.Root)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("install.sh failed: %w %s", err, firstLine(string(output)))
	}
	return nil
}

// applyWindowsPortable replaces a portable copy's program in place. The zip
// holds motty.exe beside a motty-data directory; the data is the user's and is
// never overwritten, and files the new version brings that this copy does not
// have yet are added. Windows may rename a running image but not delete it, so
// the old executable is set aside first, as the setup path does.
func (job Job) applyWindowsPortable() error {
	name := filepath.Base(job.Exe)
	staged := filepath.Join(job.Staged, name)
	info, err := os.Stat(staged)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return fmt.Errorf("the package does not hold a usable %s", name)
	}
	prefix := "." + name + "-update-"
	dir, err := os.MkdirTemp(job.Root, prefix)
	if err != nil {
		return fmt.Errorf("preparing the previous executable: %w", err)
	}
	previous := filepath.Join(dir, "previous.exe")
	if err := os.Rename(job.Exe, previous); err != nil {
		os.Remove(dir)
		return fmt.Errorf("setting the previous executable aside (close other Motty processes and retry): %w", err)
	}
	err = installPortableFiles(job.Root, job.Staged, name)
	if err != nil {
		if restoreErr := restoreExecutable(job.Exe, previous, dir, os.Remove); restoreErr != nil {
			return errors.Join(err, restoreErr)
		}
		return err
	}
	cleanRetiredExecutables(job.Root, prefix)
	return nil
}

// installPortableFiles copies the new program into place and adds the files the
// new version carries that this copy does not have yet. Existing files are
// never overwritten: motty-data is the user's, and anything a portable copy
// gained by hand stays.
func installPortableFiles(root, staged, name string) error {
	exe := filepath.Join(root, name)
	temp := exe + ".new"
	if err := copyFile(filepath.Join(staged, name), temp); err != nil {
		return fmt.Errorf("placing the new executable: %w", err)
	}
	if err := os.Rename(temp, exe); err != nil {
		os.Remove(temp)
		return fmt.Errorf("installing the new executable: %w", err)
	}
	return filepath.WalkDir(staged, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		relative, err := filepath.Rel(staged, path)
		if err != nil || relative == name {
			return err
		}
		target := filepath.Join(root, filepath.FromSlash(relative))
		if _, err := os.Stat(target); err == nil {
			return nil // already here: the user's, never overwritten
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return copyFile(path, target)
	})
}

// copyFile writes source to destination through a plain copy, keeping the mode.
func copyFile(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return err
	}
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		output.Close()
		return err
	}
	return output.Close()
}
