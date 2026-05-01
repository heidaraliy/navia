package git

import (
	"os"
	"path/filepath"
)

func FindRoot(start string) string {
	dir, err := filepath.Abs(start)
	if err != nil {
		return ""
	}
	for {
		if exists(filepath.Join(dir, ".git")) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func Rel(root, path string) string {
	if root == "" {
		return path
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." {
		return "."
	}
	return rel
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
