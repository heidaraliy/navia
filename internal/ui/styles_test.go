package ui

import "testing"

func TestNewStylesReturnsUsableStyles(t *testing.T) {
	styles := NewStyles()
	if got := styles.Brand.Render("Navia"); got == "" {
		t.Fatalf("brand style rendered empty output")
	}
	if got := styles.Panel.Render("content"); got == "" {
		t.Fatalf("panel style rendered empty output")
	}
	if got := styles.Selected.Render("row"); got == "" {
		t.Fatalf("selected style rendered empty output")
	}
}
