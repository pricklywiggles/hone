package debuglog

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
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
	path := filepath.Join(logDir(), "debug.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	logger = log.New(f, "", 0)
})

func init() {
	initOnce()
}

func logDir() string {
	if runtime.GOOS == "windows" {
		if dir, err := os.UserConfigDir(); err == nil {
			return filepath.Join(dir, "hone")
		}
	}
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".local", "share", "hone")
}

func Log(format string, args ...any) {
	if !enabled || logger == nil {
		return
	}
	ts := time.Now().Format("2006-01-02 15:04:05")
	logger.Printf("%s  %s", ts, fmt.Sprintf(format, args...))
}
