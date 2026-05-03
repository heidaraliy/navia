package lsp

import (
	"bufio"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type bufferWriteCloser struct {
	bytes.Buffer
	err error
}

func (w *bufferWriteCloser) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	return w.Buffer.Write(p)
}

func (w *bufferWriteCloser) Close() error {
	return nil
}

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

func TestRequestNotifyAndNavigationMethods(t *testing.T) {
	responses := strings.Join([]string{
		frame(`{"id":999,"result":null}`),
		frame(`{"id":1,"result":{"uri":"file:///tmp/def.go","range":{"start":{"line":0,"character":1}}}}`),
		frame(`{"id":2,"result":[{"uri":"file:///tmp/ref.go","range":{"start":{"line":2,"character":3}}}]}`),
	}, "")
	writer := &bufferWriteCloser{}
	client := &Client{in: writer, out: bufio.NewReader(strings.NewReader(responses)), nextID: 1}

	if err := client.DidOpen("/tmp/main.go", "package main"); err != nil {
		t.Fatal(err)
	}
	if err := client.DidSave("/tmp/main.go", "package main"); err != nil {
		t.Fatal(err)
	}
	defs, err := client.Definition("/tmp/main.go", 0, -1)
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) != 1 || defs[0].Path != "/tmp/def.go" || defs[0].Line != 1 || defs[0].Character != 2 {
		t.Fatalf("defs = %#v", defs)
	}
	refs, err := client.References("/tmp/main.go", 3, 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 || refs[0].Path != "/tmp/ref.go" || refs[0].Line != 3 || refs[0].Character != 4 {
		t.Fatalf("refs = %#v", refs)
	}
	out := writer.String()
	for _, want := range []string{"textDocument/didOpen", "textDocument/didSave", "textDocument/definition", "textDocument/references"} {
		if !strings.Contains(out, want) {
			t.Fatalf("request stream missing %q:\n%s", want, out)
		}
	}
}

func TestRequestAndReadErrors(t *testing.T) {
	client := &Client{in: &bufferWriteCloser{err: errors.New("write failed")}, out: bufio.NewReader(strings.NewReader("")), nextID: 1}
	if _, err := client.request("method", nil); err == nil {
		t.Fatal("request write error returned nil")
	}

	client = &Client{in: &bufferWriteCloser{}, out: bufio.NewReader(strings.NewReader(frame(`{"id":1,"error":{"message":"bad request"}}`))), nextID: 1}
	if _, err := client.request("method", nil); err == nil || err.Error() != "bad request" {
		t.Fatalf("request error = %v", err)
	}

	client = &Client{in: &bufferWriteCloser{}, out: bufio.NewReader(strings.NewReader("Content-Type: json\r\n\r\n{}")), nextID: 1}
	if _, err := client.request("method", nil); err == nil || !strings.Contains(err.Error(), "content length") {
		t.Fatalf("missing length error = %v", err)
	}

	client = &Client{out: bufio.NewReader(strings.NewReader("Content-Length: nope\r\n\r\n{}"))}
	if _, err := client.read(); err == nil {
		t.Fatal("bad content length returned nil")
	}

	client = &Client{in: &bufferWriteCloser{}, out: bufio.NewReader(strings.NewReader(frame(`not json`))), nextID: 1}
	if _, err := client.request("method", nil); err == nil {
		t.Fatal("invalid JSON response returned nil")
	}

	client = &Client{in: &bufferWriteCloser{}}
	if err := client.write(make(chan int)); err == nil {
		t.Fatal("unmarshalable write returned nil")
	}
}

func TestDecodeLocationsVariantsAndURIHelpers(t *testing.T) {
	one := []byte(`{"uri":"file:///tmp/one.go","range":{"start":{"line":1,"character":2}}}`)
	locations, err := decodeLocations(one)
	if err != nil {
		t.Fatal(err)
	}
	if len(locations) != 1 || locations[0].Path != "/tmp/one.go" {
		t.Fatalf("single location = %#v", locations)
	}
	if _, err := decodeLocations([]byte(`{`)); err == nil {
		t.Fatal("invalid locations returned nil")
	}
	if _, err := locationsFromWire([]wireLocation{{URI: "https://example.invalid/main.go"}}); err == nil {
		t.Fatal("unsupported URI returned nil")
	}
	uri := fileURI("/tmp/main.go")
	if !strings.HasPrefix(uri, "file:///") {
		t.Fatalf("fileURI = %q", uri)
	}
	if path, err := pathFromURI(uri); err != nil || path != "/tmp/main.go" {
		t.Fatalf("pathFromURI = %q %v", path, err)
	}
	if _, err := pathFromURI("file://%zz"); err == nil {
		t.Fatal("bad URI parse returned nil")
	}
	if max(1, 2) != 2 || max(3, 2) != 3 {
		t.Fatal("max returned wrong value")
	}
}

func TestDefinitionAndReferencesErrorBranches(t *testing.T) {
	client := &Client{in: &bufferWriteCloser{err: errors.New("write failed")}, out: bufio.NewReader(strings.NewReader("")), nextID: 1}
	if _, err := client.Definition("/tmp/main.go", 1, 1); err == nil {
		t.Fatal("Definition write error returned nil")
	}
	client = &Client{in: &bufferWriteCloser{err: errors.New("write failed")}, out: bufio.NewReader(strings.NewReader("")), nextID: 1}
	if _, err := client.References("/tmp/main.go", 1, 1); err == nil {
		t.Fatal("References write error returned nil")
	}
	client = &Client{in: &bufferWriteCloser{}, out: bufio.NewReader(strings.NewReader(frame(`{"id":1,"result":"bad"}`))), nextID: 1}
	if _, err := client.Definition("/tmp/main.go", 1, 1); err == nil {
		t.Fatal("Definition decode error returned nil")
	}
}

func TestStartAndCloseWithFakeServer(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "fake-lsp")
	body := "#!/bin/sh\nprintf 'Content-Length: 20\\r\\n\\r\\n{\"id\":1,\"result\":{}}'\nsleep 5\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	client, err := Start(script, dir)
	if err != nil {
		t.Fatal(err)
	}
	if client.nextID != 2 {
		t.Fatalf("nextID = %d, want 2", client.nextID)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	if err := (*Client)(nil).Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Start(filepath.Join(dir, "missing"), dir); err == nil {
		t.Fatal("Start with missing command returned nil")
	}
}

func TestStartTimesOutWhenServerDoesNotRespond(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "hanging-lsp")
	body := "#!/bin/sh\nsleep 5\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	if _, err := StartWithTimeout(script, dir, 50*time.Millisecond); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("StartWithTimeout error = %v, want timeout", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("StartWithTimeout took too long: %s", elapsed)
	}
}

func frame(json string) string {
	return "Content-Length: " + strconvItoa(len(json)) + "\r\n\r\n" + json
}

func strconvItoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits [20]byte
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	return string(digits[i:])
}
