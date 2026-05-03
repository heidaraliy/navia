package git

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

type ChangeKind int

const (
	ChangeModified ChangeKind = iota
	ChangeAdded
	ChangeDeleted
	ChangeUntracked
	ChangeRenamed
)

type Change struct {
	Path           string
	OldPath        string
	IndexStatus    byte
	WorktreeStatus byte
	Kind           ChangeKind
}

type Summary struct {
	FilesChanged   int
	FilesAdded     int
	FilesRemoved   int
	LinesChanged   int
	LinesAdded     int
	LinesRemoved   int
	TotalLineFiles int
}

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

func Status(root string) ([]Change, Summary, error) {
	out, err := gitOutput(root, "status", "--porcelain=v1", "-z")
	if err != nil {
		return nil, Summary{}, err
	}
	changes, err := parsePorcelainStatus(out)
	if err != nil {
		return nil, Summary{}, err
	}
	sort.SliceStable(changes, func(i, j int) bool {
		return changes[i].Path < changes[j].Path
	})
	summary, err := summary(root, changes)
	if err != nil {
		return changes, Summary{}, err
	}
	return changes, summary, nil
}

func Diff(root string, change Change, maxBytes int) (string, error) {
	if err := validateRelPath(root, change.Path); err != nil {
		return "", err
	}
	if change.Kind == ChangeUntracked {
		return untrackedDiff(root, change.Path, maxBytes)
	}
	var out []byte
	var err error
	if hasHead(root) {
		out, err = gitOutputLimited(root, maxBytes, "diff", "--no-ext-diff", "--patch", "HEAD", "--", change.Path)
		if err != nil {
			return "", err
		}
	}
	if len(out) == 0 {
		out, err = gitOutputLimited(root, maxBytes, "diff", "--no-ext-diff", "--patch", "--cached", "--", change.Path)
		if err != nil {
			return "", err
		}
	}
	if len(out) == 0 {
		return "No diff for " + change.Path + ".", nil
	}
	return string(out), nil
}

func Stage(root, path string) error {
	if err := validateRelPath(root, path); err != nil {
		return err
	}
	return gitRun(root, "add", "--", path)
}

func Unstage(root, path string) error {
	if err := validateRelPath(root, path); err != nil {
		return err
	}
	return gitRun(root, "restore", "--staged", "--", path)
}

func Restore(root string, change Change) error {
	if err := validateRelPath(root, change.Path); err != nil {
		return err
	}
	if change.Kind == ChangeUntracked {
		return removeRel(root, change.Path)
	}
	if change.IndexStatus != ' ' {
		if err := gitRun(root, "restore", "--staged", "--", change.Path); err != nil {
			return err
		}
	}
	if change.WorktreeStatus != ' ' || change.IndexStatus != ' ' {
		return gitRun(root, "restore", "--worktree", "--", change.Path)
	}
	return nil
}

func Remove(root string, change Change) error {
	if err := validateRelPath(root, change.Path); err != nil {
		return err
	}
	if change.Kind == ChangeUntracked {
		return removeRel(root, change.Path)
	}
	return gitRun(root, "rm", "-f", "--", change.Path)
}

func Commit(root, message string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		return errors.New("commit message is required")
	}
	return gitRun(root, "commit", "-m", message)
}

func Push(root string) error {
	branchBytes, err := gitOutput(root, "branch", "--show-current")
	if err != nil {
		return err
	}
	branch := strings.TrimSpace(string(branchBytes))
	if branch == "" {
		return errors.New("cannot push detached HEAD")
	}
	return gitRun(root, "push", "-u", "origin", branch)
}

func parsePorcelainStatus(out []byte) ([]Change, error) {
	if len(out) == 0 {
		return nil, nil
	}
	parts := bytes.Split(out, []byte{0})
	changes := make([]Change, 0, len(parts))
	for i := 0; i < len(parts); i++ {
		part := parts[i]
		if len(part) == 0 {
			continue
		}
		if len(part) < 4 {
			return nil, fmt.Errorf("malformed git status entry %q", string(part))
		}
		index, worktree := part[0], part[1]
		path := string(part[3:])
		change := Change{Path: path, IndexStatus: index, WorktreeStatus: worktree, Kind: classify(index, worktree)}
		if index == 'R' || index == 'C' {
			i++
			if i >= len(parts) {
				return nil, fmt.Errorf("missing old path for renamed git status entry %q", path)
			}
			change.OldPath = string(parts[i])
		}
		changes = append(changes, change)
	}
	return changes, nil
}

func classify(index, worktree byte) ChangeKind {
	switch {
	case index == '?' && worktree == '?':
		return ChangeUntracked
	case index == 'R' || worktree == 'R':
		return ChangeRenamed
	case index == 'A' || worktree == 'A':
		return ChangeAdded
	case index == 'D' || worktree == 'D':
		return ChangeDeleted
	default:
		return ChangeModified
	}
}

func summary(root string, changes []Change) (Summary, error) {
	var s Summary
	for _, change := range changes {
		switch change.Kind {
		case ChangeAdded, ChangeUntracked:
			s.FilesAdded++
		case ChangeDeleted:
			s.FilesRemoved++
		default:
			s.FilesChanged++
		}
	}
	if hasHead(root) {
		out, err := gitOutput(root, "diff", "--numstat", "HEAD", "--")
		if err != nil {
			return Summary{}, err
		}
		if err := addNumstat(&s, out); err != nil {
			return Summary{}, err
		}
	} else {
		out, err := gitOutput(root, "diff", "--numstat", "--cached", "--")
		if err == nil {
			if err := addNumstat(&s, out); err != nil {
				return Summary{}, err
			}
		}
	}
	for _, change := range changes {
		if change.Kind != ChangeUntracked {
			continue
		}
		lines, ok := countTextLines(filepath.Join(root, filepath.FromSlash(change.Path)))
		if ok {
			s.LinesAdded += lines
			s.TotalLineFiles++
		}
	}
	s.LinesChanged = s.LinesAdded + s.LinesRemoved
	return s, nil
}

func addNumstat(s *Summary, out []byte) error {
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		added, removed := fields[0], fields[1]
		if added == "-" || removed == "-" {
			continue
		}
		a, err := strconv.Atoi(added)
		if err != nil {
			return err
		}
		r, err := strconv.Atoi(removed)
		if err != nil {
			return err
		}
		s.LinesAdded += a
		s.LinesRemoved += r
		s.TotalLineFiles++
	}
	return scanner.Err()
}

func untrackedDiff(root, path string, maxBytes int) (string, error) {
	full := filepath.Join(root, filepath.FromSlash(path))
	info, err := os.Stat(full)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "Untracked directory: " + path, nil
	}
	file, err := os.Open(full)
	if err != nil {
		return "", err
	}
	defer file.Close()
	limit := int64(maxBytes)
	if limit <= 0 {
		limit = 256 * 1024
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return "", err
	}
	truncated := false
	if int64(len(data)) > limit {
		data = data[:limit]
		truncated = true
	}
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return "Binary untracked file: " + path, nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "diff --git a/%s b/%s\nnew file mode 100644\n--- /dev/null\n+++ b/%s\n", path, path, path)
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if i == len(lines)-1 && line == "" {
			break
		}
		b.WriteString("+")
		b.WriteString(line)
		b.WriteString("\n")
	}
	if truncated {
		b.WriteString("+... truncated ...\n")
	}
	return b.String(), nil
}

func countTextLines(path string) (int, bool) {
	data, err := os.ReadFile(path)
	if err != nil || !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return 0, false
	}
	if len(data) == 0 {
		return 0, true
	}
	lines := bytes.Count(data, []byte{'\n'})
	if !bytes.HasSuffix(data, []byte{'\n'}) {
		lines++
	}
	return lines, true
}

func validateRelPath(root, path string) error {
	if root == "" {
		return errors.New("not inside a git repository")
	}
	if filepath.IsAbs(path) || path == "" {
		return fmt.Errorf("invalid git path %q", path)
	}
	full := filepath.Clean(filepath.Join(root, filepath.FromSlash(path)))
	rootClean := filepath.Clean(root)
	rel, err := filepath.Rel(rootClean, full)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path escapes git root: %s", path)
	}
	return nil
}

func removeRel(root, path string) error {
	if err := validateRelPath(root, path); err != nil {
		return err
	}
	full := filepath.Join(root, filepath.FromSlash(path))
	return os.RemoveAll(full)
}

func gitOutput(root string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return out, nil
}

func gitOutputLimited(root string, maxBytes int, args ...string) ([]byte, error) {
	if maxBytes <= 0 {
		return gitOutput(root, args...)
	}
	var stdout limitedBuffer
	stdout.limit = maxBytes
	var stderr bytes.Buffer
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if stdout.truncated {
		out := stdout.Bytes()
		out = append(out, []byte("\n... truncated ...\n")...)
		return out, nil
	}
	if err != nil {
		return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

type limitedBuffer struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func (b *limitedBuffer) Bytes() []byte {
	return b.buf.Bytes()
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.limit <= 0 || b.truncated {
		return len(p), nil
	}
	remaining := b.limit - b.buf.Len()
	if remaining <= 0 {
		b.truncated = true
		return len(p), nil
	}
	if len(p) > remaining {
		b.buf.Write(p[:remaining])
		b.truncated = true
		return len(p), nil
	}
	if _, err := b.buf.Write(p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func gitRun(root string, args ...string) error {
	_, err := gitOutput(root, args...)
	return err
}

func hasHead(root string) bool {
	cmd := exec.Command("git", "-C", root, "rev-parse", "--verify", "HEAD")
	return cmd.Run() == nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
