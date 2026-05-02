package lsp

import "testing"

func TestDecodeLocations(t *testing.T) {
	raw := []byte(`[{"uri":"file:///tmp/main.go","range":{"start":{"line":2,"character":4}}}]`)
	locations, err := decodeLocations(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(locations) != 1 {
		t.Fatalf("locations = %d", len(locations))
	}
	if locations[0].Path != "/tmp/main.go" || locations[0].Line != 3 || locations[0].Character != 5 {
		t.Fatalf("location = %#v", locations[0])
	}
}

func TestDecodeNullLocations(t *testing.T) {
	locations, err := decodeLocations([]byte(`null`))
	if err != nil {
		t.Fatal(err)
	}
	if locations != nil {
		t.Fatalf("locations = %#v", locations)
	}
}
