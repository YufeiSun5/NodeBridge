package agentlog

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

type rotatingWriter struct {
	mu       sync.Mutex
	path     string
	maxBytes int64
	backups  int
	file     *os.File
	size     int64
	fallback io.Writer
	failed   bool
	closed   bool
}

func openRotating(path string, maxBytes int64, backups int, fallback io.Writer) (*rotatingWriter, error) {
	if maxBytes <= 0 || backups < 1 {
		return nil, fmt.Errorf("invalid runtime log limits")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create runtime log directory: %w", err)
	}
	w := &rotatingWriter{path: path, maxBytes: maxBytes, backups: backups, fallback: fallback}
	if err := w.open(); err != nil {
		return nil, err
	}
	return w, nil
}

func regularOrMissing(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("runtime log path is not a regular file: %s", path)
	}
	return nil
}

func (w *rotatingWriter) open() error {
	if err := regularOrMissing(w.path); err != nil {
		return err
	}
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("open runtime log: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	w.file, w.size = f, info.Size()
	return nil
}

func (w *rotatingWriter) rotate() error {
	for i := 1; i <= w.backups; i++ {
		if err := regularOrMissing(fmt.Sprintf("%s.%d", w.path, i)); err != nil {
			return err
		}
	}
	if w.file != nil {
		if err := w.file.Close(); err != nil {
			w.file = nil
			return err
		}
		w.file = nil
	}
	last := fmt.Sprintf("%s.%d", w.path, w.backups)
	if err := os.Remove(last); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for i := w.backups - 1; i >= 0; i-- {
		source := w.path
		if i > 0 {
			source = fmt.Sprintf("%s.%d", w.path, i)
		}
		if err := os.Rename(source, fmt.Sprintf("%s.%d", w.path, i+1)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return w.open()
}

func (w *rotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return 0, os.ErrClosed
	}
	var err error
	if w.file == nil {
		err = w.open()
	}
	if err == nil && int64(len(p)) > w.maxBytes {
		err = fmt.Errorf("runtime log record exceeds size limit")
	}
	if err == nil && w.size+int64(len(p)) > w.maxBytes {
		err = w.rotate()
	}
	var n int
	if err == nil {
		n, err = w.file.Write(p)
		w.size += int64(n)
		if err == nil && n != len(p) {
			err = io.ErrShortWrite
		}
	}
	if err != nil {
		if w.file != nil {
			if n > 0 {
				_ = w.file.Truncate(w.size - int64(n))
			}
			_ = w.file.Close()
			w.file = nil
		}
		if w.fallback != nil {
			if !w.failed {
				_, _ = fmt.Fprintf(w.fallback, "runtime diagnostic file unavailable; subsequent records use stderr: %v\n", err)
			}
			if int64(len(p)) <= w.maxBytes {
				_, _ = w.fallback.Write(p)
			}
		}
		w.failed = true
		return n, err
	}
	if w.failed && w.fallback != nil {
		_, _ = fmt.Fprintln(w.fallback, "runtime diagnostic file recovered")
	}
	w.failed = false
	return n, nil
}

func (w *rotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}
