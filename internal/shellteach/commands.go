package shellteach

import (
	"path/filepath"
	"strings"
)

func RenameCommand(oldPath, newPath string) string {
	return "Shell equivalent: mv " + Quote(oldPath) + " " + Quote(newPath)
}

func CopyCommand(src, dst string, isDir bool) string {
	if isDir {
		return "Shell equivalent: cp -r " + Quote(src) + " " + Quote(dst)
	}
	return "Shell equivalent: cp " + Quote(src) + " " + Quote(dst)
}

func MoveCommand(src, dst string) string {
	return "Shell equivalent: mv " + Quote(src) + " " + Quote(dst)
}

func MkdirCommand(path string) string {
	return "Shell equivalent: mkdir " + Quote(path)
}

func TouchCommand(path string) string {
	return "Shell equivalent: touch " + Quote(path)
}

func DeleteCommand(path string, isDir bool) string {
	cmd := "rm " + Quote(path)
	if isDir {
		cmd = "rm -rf " + Quote(path)
	}
	return "Navia safe-deleted this item. Dangerous shell equivalent would be: " + cmd
}

func OpenCommand(path string, editor string) string {
	if editor == "" {
		editor = "$EDITOR"
	}
	return "Shell equivalent: " + editor + " " + Quote(path)
}

func Quote(path string) string {
	if path == "" {
		return "''"
	}
	if !strings.ContainsAny(path, " \t\n'\"\\$&;|()<>*?[]{}!") {
		return filepath.Clean(path)
	}
	return "'" + strings.ReplaceAll(path, "'", "'\\''") + "'"
}
