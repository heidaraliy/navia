package app

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/heidaraliy/navia/internal/config"
)

func BenchmarkNavigatorViewLargeTree(b *testing.B) {
	root := b.TempDir()
	for i := 0; i < 2000; i++ {
		benchWrite(b, filepath.Join(root, fmt.Sprintf("file-%04d.go", i)), "package demo\n")
	}
	m, err := New(root, config.Default(), false)
	if err != nil {
		b.Fatal(err)
	}
	m.width, m.height = 180, 55
	m.clampLayout()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = m.View()
	}
}

func benchWrite(b *testing.B, path, value string) {
	b.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		b.Fatal(err)
	}
}
