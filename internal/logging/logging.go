// Package logging configures the server logger with a rotating file sink.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/kanomahoro/bf-cn0261-go-server/internal/config"
)

// rotator writes to one file and renames it when the size limit is reached.
type rotator struct {
	mu      sync.Mutex
	dir     string
	maxSize int64
	maxFile int
	file    *os.File
	size    int64
}

func newRotator(dir string, maxSize int64, maxFiles int) (*rotator, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	r := &rotator{dir: dir, maxSize: maxSize, maxFile: maxFiles}
	if err := r.open(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *rotator) open() error {
	path := filepath.Join(r.dir, "Server.log")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("cannot open log file: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return err
	}
	r.file = file
	r.size = info.Size()
	return nil
}

func (r *rotator) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.size+int64(len(p)) > r.maxSize {
		r.file.Close()
		for i := r.maxFile - 1; i >= 1; i-- {
			from := filepath.Join(r.dir, fmt.Sprintf("Server.%d.log", i))
			to := filepath.Join(r.dir, fmt.Sprintf("Server.%d.log", i+1))
			_ = os.Rename(from, to)
		}
		_ = os.Rename(filepath.Join(r.dir, "Server.log"), filepath.Join(r.dir, "Server.1.log"))
		if err := r.open(); err != nil {
			return 0, err
		}
	}
	n, err := r.file.Write(p)
	r.size += int64(n)
	return n, err
}

func (r *rotator) Sync() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.file.Sync()
}

// Initialize builds the process logger.
func Initialize(cfg config.Logging) (*slog.Logger, error) {
	file, err := newRotator(cfg.Directory, cfg.MaxFileSizeBytes, cfg.MaxFiles)
	if err != nil {
		return nil, err
	}
	writer := io.MultiWriter(os.Stdout, file)
	level := slog.LevelInfo
	switch cfg.Level {
	case "trace", "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error", "critical":
		level = slog.LevelError
	}
	handler := slog.NewTextHandler(writer, &slog.HandlerOptions{Level: level})
	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger, nil
}

// ParseLevel maps a configuration string to a slog level.
func ParseLevel(name string) slog.Level {
	switch name {
	case "trace", "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error", "critical":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
