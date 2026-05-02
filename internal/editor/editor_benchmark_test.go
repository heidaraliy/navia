package editor

import (
	"strings"
	"testing"
)

func BenchmarkVisibleHugeSingleLine(b *testing.B) {
	buf := NewScratch("huge.txt")
	buf.Lines = []string{strings.Repeat("x", 1<<20)}
	buf.Col = 900_000
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		lines := buf.Visible(80, 24)
		if len(lines) != 24 {
			b.Fatalf("lines = %d", len(lines))
		}
	}
}

func BenchmarkWrapDisplayLimitHugeLine(b *testing.B) {
	line := strings.Repeat("x", 1<<20)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		lines := wrapDisplayLimit(line, "  ", 80, 24)
		if len(lines) != 24 {
			b.Fatalf("lines = %d", len(lines))
		}
	}
}
