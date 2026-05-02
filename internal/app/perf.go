package app

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

var perfMu sync.Mutex

func perfNow() time.Time {
	if !perfEnabled() {
		return time.Time{}
	}
	return time.Now()
}

func perfLogDuration(name string, start time.Time, fields ...string) {
	if start.IsZero() || !perfEnabled() {
		return
	}
	perfLog(name, time.Since(start), fields...)
}

func perfEnabled() bool {
	return os.Getenv("NAVIA_PERF") != ""
}

func perfLog(name string, duration time.Duration, fields ...string) {
	out := perfWriter()
	if out == nil {
		return
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s duration_ms=%.3f", name, float64(duration.Microseconds())/1000)
	for i := 0; i+1 < len(fields); i += 2 {
		fmt.Fprintf(&b, " %s=%q", fields[i], fields[i+1])
	}
	b.WriteByte('\n')
	perfMu.Lock()
	defer perfMu.Unlock()
	_, _ = io.WriteString(out, b.String())
}

func perfWriter() io.Writer {
	if !perfEnabled() {
		return nil
	}
	if path := os.Getenv("NAVIA_PERF_LOG"); path != "" {
		file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return os.Stderr
		}
		return closeAfterWrite{file}
	}
	return os.Stderr
}

type closeAfterWrite struct {
	file *os.File
}

func (w closeAfterWrite) Write(p []byte) (int, error) {
	defer w.file.Close()
	return w.file.Write(p)
}
