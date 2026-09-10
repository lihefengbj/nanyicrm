// Package logger routes the standard library log and gin's writer into both
// stdout and a daily-rotated file under the configured log directory.
// File names look like server_20260910.log and roll over at midnight.
// Files older than the configured retention are removed automatically.
package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lihefengbj/nanyicrm/backend/internal/config"
)

// dailyWriter appends to dir/<name>_yyyymmdd.log, reopening the file when the
// date changes so a long-running process rolls over at midnight.
type dailyWriter struct {
	dir        string
	name       string
	retainDays int

	mu   sync.Mutex
	day  string
	file *os.File
}

func newDailyWriter(dir, name string, retainDays int) (*dailyWriter, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}
	name = strings.TrimSuffix(name, filepath.Ext(name))
	w := &dailyWriter{dir: dir, name: name, retainDays: retainDays}
	if err := w.rotateLocked(time.Now()); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *dailyWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if today := time.Now().Format("20060102"); today != w.day {
		if err := w.rotateLocked(time.Now()); err != nil {
			return 0, err
		}
	}
	return w.file.Write(p)
}

func (w *dailyWriter) rotateLocked(now time.Time) error {
	if w.file != nil {
		w.file.Close()
	}
	w.day = now.Format("20060102")
	path := filepath.Join(w.dir, fmt.Sprintf("%s_%s.log", w.name, w.day))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	w.file = f
	w.purgeOldLocked(now)
	return nil
}

// purgeOldLocked deletes <name>_yyyymmdd.log files past the retention window.
// Only files matching our exact naming pattern are candidates.
func (w *dailyWriter) purgeOldLocked(now time.Time) {
	if w.retainDays <= 0 {
		return
	}
	cutoff := now.AddDate(0, 0, -w.retainDays)
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return
	}
	prefix := w.name + "_"
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".log") {
			continue
		}
		dayStr := strings.TrimSuffix(strings.TrimPrefix(name, prefix), ".log")
		day, err := time.Parse("20060102", dayStr)
		if err != nil || !day.Before(cutoff) {
			continue
		}
		if err := os.Remove(filepath.Join(w.dir, name)); err == nil {
			log.Printf("removed expired log file %s", name)
		}
	}
}

func (w *dailyWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

// Setup creates the log directory and tees log/gin output into stdout plus the
// daily log file. The returned closer should be deferred by main.
func Setup(cfg config.LogConfig) (io.Closer, error) {
	w, err := newDailyWriter(cfg.Dir, cfg.File, cfg.RetainDays)
	if err != nil {
		return nil, err
	}
	out := io.MultiWriter(os.Stdout, w)
	log.SetOutput(out)
	gin.DefaultWriter = out
	gin.DefaultErrorWriter = out
	log.Printf("logging to %s", filepath.Join(cfg.Dir, fmt.Sprintf("%s_%s.log", w.name, w.day)))
	return w, nil
}
