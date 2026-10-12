package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	logMu   sync.Mutex
	logPath string
)

func initLog() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	logPath = filepath.Join(dir, "tail-split", "tail-split.log")
	_ = os.MkdirAll(filepath.Dir(logPath), 0o755)
	// keep the log small: truncate over 256 KB
	if fi, err := os.Stat(logPath); err == nil && fi.Size() > 256*1024 {
		_ = os.Truncate(logPath, 0)
	}
	return logPath
}

func logf(format string, args ...any) {
	logMu.Lock()
	defer logMu.Unlock()
	if logPath == "" {
		return
	}
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	line := fmt.Sprintf(format, args...)
	fmt.Fprintf(f, "%s %s\n", time.Now().Format("2006-01-02 15:04:05"), line)
}
