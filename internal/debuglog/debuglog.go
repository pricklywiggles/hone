package debuglog

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	logger  *log.Logger
	enabled bool
)

var initOnce = sync.OnceFunc(func() {
	if os.Getenv("HONE_DEBUG") == "" {
		return
	}
	enabled = true
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}
	path := filepath.Join(homeDir, ".local", "share", "hone", "debug.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	logger = log.New(f, "", 0)
})

func init() {
	initOnce()
}

func Log(format string, args ...any) {
	if !enabled || logger == nil {
		return
	}
	ts := time.Now().Format("2006-01-02 15:04:05")
	logger.Printf("%s  %s", ts, fmt.Sprintf(format, args...))
}
