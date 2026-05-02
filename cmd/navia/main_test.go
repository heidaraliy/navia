package main

import "testing"

func TestDisplayVersionUsesInjectedVersion(t *testing.T) {
	old := version
	t.Cleanup(func() {
		version = old
	})

	version = "1.2.3"
	if got := displayVersion(); got != "1.2.3" {
		t.Fatalf("displayVersion() = %q, want %q", got, "1.2.3")
	}
}

func TestDisplayVersionTrimsLeadingV(t *testing.T) {
	old := version
	t.Cleanup(func() {
		version = old
	})

	version = "v1.2.3"
	if got := displayVersion(); got != "1.2.3" {
		t.Fatalf("displayVersion() = %q, want %q", got, "1.2.3")
	}
}
