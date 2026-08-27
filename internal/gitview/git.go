package gitview

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

type Kind byte

const (
	Modified    Kind = 'M'
	Untracked   Kind = 'U'
	Deleted     Kind = 'D'
	Renamed     Kind = 'R'
	Copied      Kind = 'C'
	TypeChanged Kind = 'T'
	Submodule   Kind = 'S'
	Conflicted  Kind = 'X'
)

type Change struct {
	Path, OldPath string
	Kind          Kind
	Index, Work   byte
	Submodule     bool
}

type Counts struct {
	FilesNew, FilesModified, FilesDeleted int
	LinesNew, LinesModified, LinesDeleted int
}

type LineKind byte

const (
	Context LineKind = iota
	Added
	Removed
	Hunk
	Header
)

type DiffLine struct {
	Kind     LineKind
	Old, New int
	Text     string
}

type SideLine struct {
	Old, New         int
	OldText, NewText string
	Kind             LineKind
}

type FileDiff struct {
	Path   string
	Lines  []DiffLine
	Side   []SideLine
	Counts Counts
	Binary bool
	Size   int64
}

type Commit struct {
	Hash, Short, Subject, Relative string
}

const emptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

func Commits(root string, skip, count int) ([]Commit, error) {
	out, err := command(root, "log", "--first-parent", fmt.Sprintf("--skip=%d", skip), fmt.Sprintf("-n%d", count), "--format=%H%x1f%h%x1f%s%x1f%cr%x1e")
	if err != nil {
		return nil, err
	}
	var commits []Commit
	for _, record := range strings.Split(string(out), "\x1e") {
		fields := strings.Split(strings.TrimSpace(record), "\x1f")
		if len(fields) == 4 {
			commits = append(commits, Commit{fields[0], fields[1], fields[2], fields[3]})
		}
	}
	return commits, nil
}

func commitParent(root, hash string) string {
	out, err := command(root, "rev-parse", hash+"^")
	if err != nil {
		return emptyTree
	}
	return strings.TrimSpace(string(out))
}

func StatusCommit(root, hash string) ([]Change, error) {
	parent := commitParent(root, hash)
	out, err := command(root, "diff", "--name-status", "-z", "-M", "-C", parent, hash, "--")
	if err != nil {
		return nil, err
	}
	parts := bytes.Split(out, []byte{0})
	changes := make([]Change, 0, len(parts)/2)
	for i := 0; i < len(parts) && len(parts[i]) > 0; {
		status := string(parts[i])
		i++
		if i >= len(parts) {
			break
		}
		kind := Kind(status[0])
		change := Change{Kind: kind}
		if kind == Renamed || kind == Copied {
			change.OldPath = string(parts[i])
			i++
			if i >= len(parts) {
				break
			}
		}
		change.Path = string(parts[i])
		i++
		if kind == 'A' {
			change.Kind = Untracked
		}
		changes = append(changes, change)
	}
	sort.SliceStable(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	return changes, nil
}

func AggregateCommit(root, hash string, changes []Change) (Counts, error) {
	var counts Counts
	for _, change := range changes {
		switch change.Kind {
		case Untracked, Copied:
			counts.FilesNew++
		case Deleted:
			counts.FilesDeleted++
		default:
			counts.FilesModified++
		}
	}
	out, err := diffOutput(root, 0, "--unified=0", commitParent(root, hash), hash, "--")
	if err != nil {
		return Counts{}, err
	}
	counts.LinesNew, counts.LinesModified, counts.LinesDeleted = hunkCounts(out)
	return counts, nil
}

func DiffCommit(root, hash string, change Change, limit int) (FileDiff, error) {
	raw, err := diffOutput(root, limit, "--no-ext-diff", "--patch", commitParent(root, hash), hash, "--", change.Path)
	if err != nil {
		return FileDiff{}, err
	}
	lines, binary := parseDiff(raw)
	add, mod, del := hunkCounts(raw)
	size := int64(0)
	if out, sizeErr := command(root, "cat-file", "-s", hash+":"+change.Path); sizeErr == nil {
		size, _ = strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	}
	return FileDiff{Path: change.Path, Lines: lines, Side: pairLines(lines), Counts: Counts{LinesNew: add, LinesModified: mod, LinesDeleted: del}, Binary: binary, Size: size}, nil
}

func SearchContentCommit(root, hash, query string, changes []Change) (map[string]bool, error) {
	matches := make(map[string]bool)
	if query == "" || len(changes) == 0 {
		return matches, nil
	}
	paths := make([]string, 0, len(changes))
	for _, change := range changes {
		paths = append(paths, change.Path)
	}
	for _, revision := range []string{hash, commitParent(root, hash)} {
		args := []string{"grep", "-z", "-I", "-l", "-i", "-F", "-e", query, revision, "--"}
		args = append(args, paths...)
		out, err := commandAllowNoMatches(root, args...)
		if err != nil {
			return nil, err
		}
		for _, path := range bytes.Split(out, []byte{0}) {
			name := strings.TrimPrefix(string(path), revision+":")
			if name != "" {
				matches[name] = true
			}
		}
	}
	return matches, nil
}

func Root(start string) (string, error) {
	out, err := command(start, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", errors.New("not inside a Git repository")
	}
	return strings.TrimSpace(string(out)), nil
}

func Status(root string) ([]Change, error) {
	out, err := command(root, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return nil, err
	}
	parts := bytes.Split(out, []byte{0})
	changes := make([]Change, 0, len(parts))
	for i := 0; i < len(parts); i++ {
		part := parts[i]
		if len(part) == 0 {
			continue
		}
		if len(part) < 4 {
			return nil, fmt.Errorf("malformed Git status entry %q", part)
		}
		change := Change{Index: part[0], Work: part[1], Path: string(part[3:])}
		if part[0] == 'R' || part[0] == 'C' {
			i++
			if i >= len(parts) {
				return nil, errors.New("malformed Git rename entry")
			}
			change.OldPath = string(parts[i])
		}
		change.Kind = classify(change)
		changes = append(changes, change)
	}
	gitlinks, _ := command(root, "ls-files", "--stage", "-z")
	for _, entry := range bytes.Split(gitlinks, []byte{0}) {
		fields := bytes.Fields(entry)
		if len(fields) < 4 || string(fields[0]) != "160000" {
			continue
		}
		path := string(fields[3])
		for i := range changes {
			if changes[i].Path == path && changes[i].Kind != Deleted {
				changes[i].Kind, changes[i].Submodule = Submodule, true
			}
		}
	}
	sort.SliceStable(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	return changes, nil
}

func classify(change Change) Kind {
	x, y := change.Index, change.Work
	if x == '?' && y == '?' {
		return Untracked
	}
	if x == 'U' || y == 'U' || (x == 'A' && y == 'A') || (x == 'D' && y == 'D') {
		return Conflicted
	}
	if x == 'R' || y == 'R' {
		return Renamed
	}
	if x == 'C' || y == 'C' {
		return Copied
	}
	if x == 'D' || y == 'D' {
		return Deleted
	}
	if x == 'A' || y == 'A' {
		return Untracked
	}
	if x == 'T' || y == 'T' {
		return TypeChanged
	}
	return Modified
}

func Aggregate(root string, changes []Change) (Counts, error) {
	var counts Counts
	for _, change := range changes {
		switch change.Kind {
		case Untracked, Copied:
			counts.FilesNew++
		case Deleted:
			counts.FilesDeleted++
		default:
			counts.FilesModified++
		}
	}
	out, err := diffOutput(root, 0, "--unified=0", "HEAD", "--")
	if err != nil && !isUnborn(root) {
		return Counts{}, err
	}
	add, mod, del := hunkCounts(out)
	counts.LinesNew, counts.LinesModified, counts.LinesDeleted = add, mod, del
	for _, change := range changes {
		if change.Kind != Untracked {
			continue
		}
		lines, text := countTextFile(filepath.Join(root, filepath.FromSlash(change.Path)))
		if text {
			counts.LinesNew += lines
		}
	}
	return counts, nil
}

func Diff(root string, change Change, limit int) (FileDiff, error) {
	var raw []byte
	var err error
	if change.Kind == Untracked && change.Index == '?' {
		raw, err = untrackedPatch(root, change.Path, limit)
	} else {
		raw, err = diffOutput(root, limit, "--no-ext-diff", "--patch", "HEAD", "--", change.Path)
		if err != nil && isUnborn(root) {
			raw, err = diffOutput(root, limit, "--no-ext-diff", "--patch", "--cached", "--", change.Path)
		}
	}
	if err != nil {
		return FileDiff{}, err
	}
	lines, binary := parseDiff(raw)
	add, mod, del := hunkCounts(raw)
	return FileDiff{Path: change.Path, Lines: lines, Side: pairLines(lines), Counts: Counts{LinesNew: add, LinesModified: mod, LinesDeleted: del}, Binary: binary, Size: changedFileSize(root, change)}, nil
}

// SearchContent returns changed paths whose current or HEAD contents contain
// query. Git does the searching, so Drift does not need to load every file.
func SearchContent(root, query string, changes []Change) (map[string]bool, error) {
	matches := make(map[string]bool)
	if query == "" || len(changes) == 0 {
		return matches, nil
	}
	paths := make([]string, 0, len(changes))
	for _, change := range changes {
		paths = append(paths, change.Path)
	}
	base := []string{"grep", "-z", "-I", "-l", "-i", "-F", "-e", query}
	for _, revision := range []string{"", "HEAD"} {
		args := append([]string(nil), base...)
		if revision == "" {
			args = append(args, "--untracked")
		} else {
			args = append(args, revision)
		}
		args = append(args, "--")
		args = append(args, paths...)
		out, err := commandAllowNoMatches(root, args...)
		if err != nil {
			return nil, err
		}
		for _, path := range bytes.Split(out, []byte{0}) {
			name := strings.TrimPrefix(string(path), "HEAD:")
			if name != "" {
				matches[name] = true
			}
		}
	}
	return matches, nil
}

func commandAllowNoMatches(root string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.Output()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return out, nil
	}
	return out, err
}

func changedFileSize(root string, change Change) int64 {
	if info, err := os.Stat(filepath.Join(root, filepath.FromSlash(change.Path))); err == nil {
		return info.Size()
	}
	out, err := command(root, "cat-file", "-s", "HEAD:"+change.Path)
	if err != nil {
		return 0
	}
	size, _ := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	return size
}

func pairLines(lines []DiffLine) []SideLine {
	result := make([]SideLine, 0, len(lines))
	for i := 0; i < len(lines); {
		if lines[i].Kind != Removed && lines[i].Kind != Added {
			line := lines[i]
			result = append(result, SideLine{Old: line.Old, New: line.New, OldText: line.Text, NewText: line.Text, Kind: line.Kind})
			i++
			continue
		}
		var removed, added []DiffLine
		for i < len(lines) && lines[i].Kind == Removed {
			removed = append(removed, lines[i])
			i++
		}
		for i < len(lines) && lines[i].Kind == Added {
			added = append(added, lines[i])
			i++
		}
		for row := 0; row < max(len(removed), len(added)); row++ {
			var paired SideLine
			if row < len(removed) {
				paired.Old, paired.OldText, paired.Kind = removed[row].Old, removed[row].Text, Removed
			}
			if row < len(added) {
				paired.New, paired.NewText = added[row].New, added[row].Text
				if paired.Kind != Removed {
					paired.Kind = Added
				}
			}
			result = append(result, paired)
		}
	}
	return result
}

func parseDiff(raw []byte) ([]DiffLine, bool) {
	if bytes.Contains(raw, []byte("Binary files ")) || bytes.Contains(raw, []byte("GIT binary patch")) {
		return []DiffLine{{Kind: Header, Text: "Binary file changed"}}, true
	}
	var result []DiffLine
	oldLine, newLine := 0, 0
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "@@"):
			oldLine, newLine = hunkStart(line)
			result = append(result, DiffLine{Kind: Hunk, Text: compactHunk(line)})
		case strings.HasPrefix(line, "diff --git "), strings.HasPrefix(line, "index "), strings.HasPrefix(line, "--- "), strings.HasPrefix(line, "+++ "):
			continue
		case strings.HasPrefix(line, "new file mode "), strings.HasPrefix(line, "deleted file mode "), strings.HasPrefix(line, "similarity index "), strings.HasPrefix(line, "rename from "), strings.HasPrefix(line, "rename to "):
			result = append(result, DiffLine{Kind: Header, Text: compactMetadata(line)})
		case strings.HasPrefix(line, "+"):
			result = append(result, DiffLine{Kind: Added, New: newLine, Text: strings.TrimPrefix(line, "+")})
			newLine++
		case strings.HasPrefix(line, "-"):
			result = append(result, DiffLine{Kind: Removed, Old: oldLine, Text: strings.TrimPrefix(line, "-")})
			oldLine++
		case strings.HasPrefix(line, "\\"):
			result = append(result, DiffLine{Kind: Header, Text: line})
		default:
			result = append(result, DiffLine{Kind: Context, Old: oldLine, New: newLine, Text: strings.TrimPrefix(line, " ")})
			if oldLine > 0 {
				oldLine++
			}
			if newLine > 0 {
				newLine++
			}
		}
	}
	return result, false
}

func compactHunk(line string) string {
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return "── change ──"
	}
	return "── " + fields[1] + "  " + fields[2] + " ──"
}

func compactMetadata(line string) string {
	switch {
	case strings.HasPrefix(line, "new file mode "):
		return "new file"
	case strings.HasPrefix(line, "deleted file mode "):
		return "deleted file"
	case strings.HasPrefix(line, "similarity index "):
		return "similarity " + strings.TrimPrefix(line, "similarity index ")
	case strings.HasPrefix(line, "rename from "):
		return "from " + strings.TrimPrefix(line, "rename from ")
	case strings.HasPrefix(line, "rename to "):
		return "to   " + strings.TrimPrefix(line, "rename to ")
	default:
		return line
	}
}

func hunkCounts(raw []byte) (added, modified, deleted int) {
	adds, dels := 0, 0
	flush := func() {
		paired := min(adds, dels)
		modified += paired
		added += adds - paired
		deleted += dels - paired
		adds, dels = 0, 0
	}
	inHunk := false
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "@@") {
			flush()
			inHunk = true
			continue
		}
		if strings.HasPrefix(line, "diff --git ") {
			flush()
			inHunk = false
			continue
		}
		if !inHunk {
			continue
		}
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			adds++
		}
		if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			dels++
		}
	}
	flush()
	return
}

func hunkStart(line string) (int, int) {
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return 0, 0
	}
	parse := func(value string) int {
		value = strings.TrimLeft(value, "+-")
		value = strings.SplitN(value, ",", 2)[0]
		n, _ := strconv.Atoi(value)
		return n
	}
	return parse(fields[1]), parse(fields[2])
}

func untrackedPatch(root, path string, limit int) ([]byte, error) {
	file, err := os.Open(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if limit <= 0 {
		limit = 1024 * 1024
	}
	data, err := io.ReadAll(io.LimitReader(file, int64(limit+1)))
	if err != nil {
		return nil, err
	}
	if !utf8.Valid(data) || bytes.Contains(data, []byte{0}) {
		return []byte("Binary files /dev/null and b/" + path + " differ\n"), nil
	}
	truncated := len(data) > limit
	if truncated {
		data, truncated = data[:limit], true
	}
	var out strings.Builder
	fmt.Fprintf(&out, "diff --git a/%s b/%s\nnew file mode 100644\n--- /dev/null\n+++ b/%s\n@@ -0,0 +1,%d @@\n", path, path, path, lineCount(data))
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		out.WriteByte('+')
		out.WriteString(line)
		out.WriteByte('\n')
	}
	if truncated {
		out.WriteString("+… diff truncated …\n")
	}
	return []byte(out.String()), nil
}

func diffOutput(root string, limit int, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", root, "diff"}, args...)...)
	if limit <= 0 {
		return cmd.CombinedOutput()
	}
	var out bytes.Buffer
	cmd.Stdout = &limitedWriter{w: &out, remaining: limit}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("git diff: %s", strings.TrimSpace(stderr.String()))
	}
	return out.Bytes(), nil
}

type limitedWriter struct {
	w         *bytes.Buffer
	remaining int
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	n := len(p)
	if w.remaining > 0 {
		keep := min(len(p), w.remaining)
		_, _ = w.w.Write(p[:keep])
		w.remaining -= keep
	}
	return n, nil
}

func command(root string, args ...string) ([]byte, error) {
	out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return out, nil
}

func isUnborn(root string) bool {
	return exec.Command("git", "-C", root, "rev-parse", "--verify", "HEAD").Run() != nil
}
func lineCount(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	n := bytes.Count(data, []byte{'\n'})
	if !bytes.HasSuffix(data, []byte{'\n'}) {
		n++
	}
	return n
}

func countTextFile(path string) (int, bool) {
	file, err := os.Open(path)
	if err != nil {
		return 0, false
	}
	defer file.Close()
	buf := make([]byte, 64*1024)
	lines, total := 0, 0
	last := byte('\n')
	for {
		n, readErr := file.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			if bytes.Contains(chunk, []byte{0}) {
				return 0, false
			}
			lines += bytes.Count(chunk, []byte{'\n'})
			last, total = chunk[n-1], total+n
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return 0, false
		}
	}
	if total > 0 && last != '\n' {
		lines++
	}
	return lines, true
}
