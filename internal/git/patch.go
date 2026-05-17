package git

import (
	"bytes"
	"fmt"
	"strings"
)

type PatchReview struct {
	Changes []Change
	Summary Summary
	Patches map[string]string
}

func ParsePatchReview(data []byte) (PatchReview, error) {
	var review PatchReview
	review.Patches = make(map[string]string)

	sections := splitPatchSections(data)
	for _, section := range sections {
		change, summary, ok := parsePatchSection(section)
		if !ok {
			continue
		}
		review.Changes = append(review.Changes, change)
		review.Summary.FilesAdded += summary.FilesAdded
		review.Summary.FilesChanged += summary.FilesChanged
		review.Summary.FilesRemoved += summary.FilesRemoved
		review.Summary.LinesAdded += summary.LinesAdded
		review.Summary.LinesRemoved += summary.LinesRemoved
		review.Summary.TotalLineFiles += summary.TotalLineFiles
		review.Patches[change.Path] = string(section)
	}
	review.Summary.LinesChanged = review.Summary.LinesAdded + review.Summary.LinesRemoved
	if len(review.Changes) == 0 && len(bytes.TrimSpace(data)) > 0 {
		return PatchReview{}, fmt.Errorf("patch contains no file diffs")
	}
	return review, nil
}

func splitPatchSections(data []byte) [][]byte {
	var sections [][]byte
	start := -1
	offset := 0
	lines := bytes.SplitAfter(data, []byte("\n"))
	for _, line := range lines {
		if bytes.HasPrefix(line, []byte("diff --git ")) {
			if start >= 0 {
				sections = append(sections, data[start:offset])
			}
			start = offset
		}
		offset += len(line)
	}
	if start >= 0 {
		sections = append(sections, data[start:])
	}
	return sections
}

func parsePatchSection(section []byte) (Change, Summary, bool) {
	var change Change
	var s Summary
	var oldPath, newPath, renameFrom, renameTo string
	var inHunk bool
	var sawLineChange bool

	for _, rawLine := range bytes.Split(section, []byte("\n")) {
		line := strings.TrimSuffix(string(rawLine), "\r")
		switch {
		case strings.HasPrefix(line, "diff --git "):
			oldPath, newPath = parseDiffGitPaths(line)
		case strings.HasPrefix(line, "new file mode "):
			change.Kind = ChangeAdded
			change.IndexStatus = 'A'
		case strings.HasPrefix(line, "deleted file mode "):
			change.Kind = ChangeDeleted
			change.IndexStatus = 'D'
		case strings.HasPrefix(line, "rename from "):
			renameFrom = strings.TrimPrefix(line, "rename from ")
			change.Kind = ChangeRenamed
			change.IndexStatus = 'R'
		case strings.HasPrefix(line, "rename to "):
			renameTo = strings.TrimPrefix(line, "rename to ")
			change.Kind = ChangeRenamed
			change.IndexStatus = 'R'
		case strings.HasPrefix(line, "--- "):
			oldPath = cleanPatchPath(strings.TrimPrefix(line, "--- "))
		case strings.HasPrefix(line, "+++ "):
			newPath = cleanPatchPath(strings.TrimPrefix(line, "+++ "))
		case strings.HasPrefix(line, "@@"):
			inHunk = true
		case inHunk && strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			s.LinesAdded++
			sawLineChange = true
		case inHunk && strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			s.LinesRemoved++
			sawLineChange = true
		}
	}

	if renameFrom != "" {
		change.OldPath = renameFrom
	}
	if renameTo != "" {
		newPath = renameTo
	}
	if newPath == "/dev/null" || newPath == "" {
		change.Path = oldPath
	} else {
		change.Path = newPath
	}
	if change.Path == "" || change.Path == "/dev/null" {
		return Change{}, Summary{}, false
	}
	if change.OldPath == "" && oldPath != "" && oldPath != "/dev/null" && oldPath != change.Path {
		change.OldPath = oldPath
	}
	if change.Kind == 0 {
		switch {
		case oldPath == "/dev/null":
			change.Kind = ChangeAdded
			change.IndexStatus = 'A'
		case newPath == "/dev/null":
			change.Kind = ChangeDeleted
			change.IndexStatus = 'D'
		default:
			change.Kind = ChangeModified
			change.IndexStatus = 'M'
		}
	}
	switch change.Kind {
	case ChangeAdded:
		s.FilesAdded = 1
	case ChangeDeleted:
		s.FilesRemoved = 1
	default:
		s.FilesChanged = 1
	}
	if sawLineChange {
		s.TotalLineFiles = 1
	}
	return change, s, true
}

func parseDiffGitPaths(line string) (string, string) {
	rest := strings.TrimPrefix(line, "diff --git ")
	if before, after, ok := strings.Cut(rest, " b/"); ok {
		return cleanPatchPath(before), cleanPatchPath("b/" + after)
	}
	return "", ""
}

func cleanPatchPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "/dev/null" {
		return path
	}
	path = strings.TrimPrefix(path, "a/")
	path = strings.TrimPrefix(path, "b/")
	return path
}
