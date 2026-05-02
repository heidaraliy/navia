package app

import (
	"fmt"
	"strings"
	"testing"

	"github.com/heidaraliy/navia/internal/config"
	navfs "github.com/heidaraliy/navia/internal/fs"
	"github.com/heidaraliy/navia/internal/syntax"
	"github.com/heidaraliy/navia/internal/ui"
)

func BenchmarkRenderPreviewContentHugeLine(b *testing.B) {
	m := Model{
		previewViewport: viewportForTest(80, 24),
		preview: navfs.Preview{
			Kind:    navfs.PreviewText,
			Path:    "huge.txt",
			Content: strings.Repeat("x", 1<<20),
		},
		styles: ui.NewStyles(),
		syntax: syntax.New(config.Default().Theme),
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if got := m.renderPreviewContent(); !strings.Contains(got, " ...") {
			b.Fatalf("preview was not clipped")
		}
	}
}

func BenchmarkRenderListLargeTree(b *testing.B) {
	const rows = 100_000
	m := Model{
		width:         120,
		height:        40,
		selectedIndex: rows - 1,
		focus:         FocusTree,
		styles:        ui.NewStyles(),
	}
	m.treeRows = make([]TreeRow, rows)
	m.rows = make([]ResultRow, rows)
	for i := 0; i < rows; i++ {
		entry := navfs.FileEntry{
			Name: fmt.Sprintf("file-%06d.go", i),
			Path: fmt.Sprintf("/tmp/navia/file-%06d.go", i),
		}
		row := TreeRow{Entry: entry, Depth: i % 6}
		m.treeRows[i] = row
		m.rows[i] = ResultRow{Entry: entry, Depth: row.Depth}
	}
	m.rebuildTreeRowIndex()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if got := m.renderList(48); !strings.Contains(got, "file-099999.go") {
			b.Fatalf("renderList missing selected row")
		}
	}
}
