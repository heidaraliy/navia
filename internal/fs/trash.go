package fs

import (
	"errors"
	"os"
	"path/filepath"
	"time"
)

func SafeDelete(path, root string) (string, error) {
	if root != "" {
		if !IsSubpath(root, path) {
			return "", errors.New("path is outside delete root")
		}
	}
	trashDir, err := GlobalTrashDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(trashDir, 0o755); err != nil {
		return "", err
	}
	name := filepath.Base(path)
	stamp := time.Now().Format("20060102-150405")
	target := filepath.Join(trashDir, stamp+"-"+name)
	target = UniquePath(target)
	if err := os.Rename(path, target); err != nil {
		return "", err
	}
	return target, nil
}

func GlobalTrashDir() (string, error) {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "navia", "trash"), nil
}
