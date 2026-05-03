package fs

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type OperationResult struct {
	Message string
	Command string
}

func Rename(path, newName string) (string, error) {
	if err := validateLeafName(newName); err != nil {
		return "", err
	}
	target := filepath.Join(filepath.Dir(path), newName)
	if _, err := os.Lstat(target); err == nil {
		return "", os.ErrExist
	}
	return target, os.Rename(path, target)
}

func CreateFile(dir, name string) (string, error) {
	if err := validateLeafName(name); err != nil {
		return "", err
	}
	path := filepath.Join(dir, name)
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", err
	}
	return path, file.Close()
}

func CreateDir(dir, name string) (string, error) {
	if err := validateLeafName(name); err != nil {
		return "", err
	}
	path := filepath.Join(dir, name)
	return path, os.Mkdir(path, 0o755)
}

func CopyPath(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		if err := rejectDescendantDestination(src, dst); err != nil {
			return err
		}
	}
	if info.IsDir() {
		return copyDir(src, dst)
	}
	return copyFile(src, dst, info.Mode())
}

func MovePath(src, dst string) error {
	if info, err := os.Stat(src); err == nil && info.IsDir() {
		if err := rejectDescendantDestination(src, dst); err != nil {
			return err
		}
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := CopyPath(src, dst); err != nil {
		return err
	}
	return os.RemoveAll(src)
}

func validateLeafName(name string) error {
	if name == "" {
		return errors.New("name cannot be empty")
	}
	if filepath.IsAbs(name) || name == "." || name == ".." || filepath.Clean(name) != name {
		return errors.New("name must be a single path segment")
	}
	if strings.ContainsAny(name, `/\`) {
		return errors.New("name must not contain path separators")
	}
	return nil
}

func rejectDescendantDestination(src, dst string) error {
	srcAbs, err := filepath.Abs(src)
	if err != nil {
		return err
	}
	dstAbs, err := filepath.Abs(dst)
	if err != nil {
		return err
	}
	if IsSubpath(srcAbs, dstAbs) {
		return errors.New("destination cannot be inside source")
	}
	return nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func copyDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.Mkdir(dst, info.Mode()); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == src {
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.Mkdir(target, info.Mode())
		}
		return copyFile(path, target, info.Mode())
	})
}
