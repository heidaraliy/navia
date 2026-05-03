package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Client struct {
	cmd     *exec.Cmd
	in      io.WriteCloser
	out     *bufio.Reader
	nextID  int
	mu      sync.Mutex
	timeout time.Duration
}

type Location struct {
	Path      string
	Line      int
	Character int
}

const defaultRequestTimeout = 5 * time.Second

func Start(command, root string) (*Client, error) {
	return StartWithTimeout(command, root, defaultRequestTimeout)
}

func StartWithTimeout(command, root string, timeout time.Duration) (*Client, error) {
	resolved, err := exec.LookPath(command)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(resolved)
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	outPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	c := &Client{cmd: cmd, in: in, out: bufio.NewReader(outPipe), nextID: 1, timeout: timeout}
	params := map[string]any{
		"processId": nil,
		"rootUri":   fileURI(root),
		"capabilities": map[string]any{
			"textDocument": map[string]any{
				"definition": map[string]any{"linkSupport": false},
				"references": map[string]any{},
			},
		},
	}
	if _, err := c.request("initialize", params); err != nil {
		_ = c.Close()
		return nil, err
	}
	_ = c.notify("initialized", map[string]any{})
	return c, nil
}

func (c *Client) Close() error {
	if c == nil || c.cmd == nil || c.cmd.Process == nil {
		return nil
	}
	_ = c.notify("exit", nil)
	return c.cmd.Process.Kill()
}

func (c *Client) DidOpen(path, text string) error {
	return c.notify("textDocument/didOpen", map[string]any{
		"textDocument": map[string]any{
			"uri":        fileURI(path),
			"languageId": "go",
			"version":    1,
			"text":       text,
		},
	})
}

func (c *Client) DidSave(path, text string) error {
	return c.notify("textDocument/didSave", map[string]any{
		"textDocument": map[string]any{"uri": fileURI(path)},
		"text":         text,
	})
}

func (c *Client) Definition(path string, line, character int) ([]Location, error) {
	raw, err := c.request("textDocument/definition", textPositionParams(path, line, character))
	if err != nil {
		return nil, err
	}
	return decodeLocations(raw)
}

func (c *Client) References(path string, line, character int) ([]Location, error) {
	params := textPositionParams(path, line, character)
	params["context"] = map[string]any{"includeDeclaration": true}
	raw, err := c.request("textDocument/references", params)
	if err != nil {
		return nil, err
	}
	return decodeLocations(raw)
}

func textPositionParams(path string, line, character int) map[string]any {
	return map[string]any{
		"textDocument": map[string]any{"uri": fileURI(path)},
		"position": map[string]any{
			"line":      max(0, line-1),
			"character": max(0, character-1),
		},
	}
}

func (c *Client) request(method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	id := c.nextID
	c.nextID++
	if err := c.write(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
		return nil, err
	}
	for {
		msg, err := c.readWithTimeout()
		if err != nil {
			return nil, err
		}
		var envelope struct {
			ID     int             `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(msg, &envelope); err != nil {
			return nil, err
		}
		if envelope.ID != id {
			continue
		}
		if envelope.Error != nil {
			return nil, errors.New(envelope.Error.Message)
		}
		return envelope.Result, nil
	}
}

func (c *Client) readWithTimeout() ([]byte, error) {
	timeout := c.timeout
	if timeout <= 0 {
		timeout = defaultRequestTimeout
	}
	type result struct {
		msg []byte
		err error
	}
	done := make(chan result, 1)
	go func() {
		msg, err := c.read()
		done <- result{msg: msg, err: err}
	}()
	select {
	case res := <-done:
		return res.msg, res.err
	case <-time.After(timeout):
		return nil, fmt.Errorf("lsp response timed out after %s", timeout)
	}
}

func (c *Client) notify(method string, params any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.write(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}

func (c *Client) write(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(c.in, "Content-Length: %d\r\n\r\n%s", len(data), data)
	return err
}

func (c *Client) read() ([]byte, error) {
	length := -1
	for {
		line, err := c.out.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		key, value, ok := strings.Cut(line, ":")
		if ok && strings.EqualFold(strings.TrimSpace(key), "Content-Length") {
			n, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return nil, err
			}
			length = n
		}
	}
	if length < 0 {
		return nil, errors.New("lsp response missing content length")
	}
	buf := make([]byte, length)
	_, err := io.ReadFull(c.out, buf)
	return buf, err
}

func decodeLocations(raw json.RawMessage) ([]Location, error) {
	if bytes.Equal(raw, []byte("null")) || len(raw) == 0 {
		return nil, nil
	}
	var many []wireLocation
	if err := json.Unmarshal(raw, &many); err == nil {
		return locationsFromWire(many)
	}
	var one wireLocation
	if err := json.Unmarshal(raw, &one); err != nil {
		return nil, err
	}
	return locationsFromWire([]wireLocation{one})
}

type wireLocation struct {
	URI   string `json:"uri"`
	Range struct {
		Start struct {
			Line      int `json:"line"`
			Character int `json:"character"`
		} `json:"start"`
	} `json:"range"`
}

func locationsFromWire(items []wireLocation) ([]Location, error) {
	locations := make([]Location, 0, len(items))
	for _, item := range items {
		path, err := pathFromURI(item.URI)
		if err != nil {
			return nil, err
		}
		locations = append(locations, Location{
			Path:      path,
			Line:      item.Range.Start.Line + 1,
			Character: item.Range.Start.Character + 1,
		})
	}
	return locations, nil
}

func fileURI(path string) string {
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
	return u.String()
}

func pathFromURI(uri string) (string, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", err
	}
	if u.Scheme != "file" {
		return "", fmt.Errorf("unsupported uri: %s", uri)
	}
	return filepath.FromSlash(u.Path), nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
