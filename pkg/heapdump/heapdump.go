// Package heapdump writes pprof profiles on demand and installs a SIGUSR1
// handler that triggers them. It is shared by the gateway host process and the
// llmbridge plugin subprocess so both dump into the same directory.
package heapdump

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"syscall"
	"time"

	"picotera/pkg/logx"
)

// Write emits heap, allocs and goroutine pprof profiles for the current process
// into dir, tagging each file with role ("host" or "plugin") and a UTC
// timestamp. It returns the paths written so far; on the first failure it
// returns immediately, leaving any already-written files in place.
func Write(dir, role string) ([]string, error) {
	ts := time.Now().UTC().Format("20060102T150405")
	var written []string

	// heap: GC first so the profile reflects the live set after collection,
	// which is the standard practice for chasing leaks.
	runtime.GC()
	heapPath := filepath.Join(dir, fmt.Sprintf("picotera-%s-%s-heap.pprof", role, ts))
	if err := writeProfile(heapPath, "heap"); err != nil {
		return written, err
	}
	written = append(written, heapPath)

	allocsPath := filepath.Join(dir, fmt.Sprintf("picotera-%s-%s-allocs.pprof", role, ts))
	if err := writeProfile(allocsPath, "allocs"); err != nil {
		return written, err
	}
	written = append(written, allocsPath)

	goroutinePath := filepath.Join(dir, fmt.Sprintf("picotera-%s-%s-goroutine.pprof", role, ts))
	if err := writeProfile(goroutinePath, "goroutine"); err != nil {
		return written, err
	}
	written = append(written, goroutinePath)

	return written, nil
}

func writeProfile(path, name string) error {
	p := pprof.Lookup(name)
	if p == nil {
		return fmt.Errorf("heapdump: unknown profile %q", name)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("heapdump: create %s: %w", path, err)
	}
	if err := p.WriteTo(f, 0); err != nil {
		_ = f.Close()
		return fmt.Errorf("heapdump: write %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("heapdump: close %s: %w", path, err)
	}
	return nil
}

// Install registers a SIGUSR1 handler that writes profiles into dir under the
// given role. After each successful or failed dump it invokes onDump (when
// non-nil), letting the host forward the signal to the plugin. The signal
// channel is buffered to 1, so bursts of SIGUSR1 collapse into sequential dumps
// rather than queueing without bound.
func Install(dir, role string, onDump func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGUSR1)
	go func() {
		for range ch {
			log := logx.New().WithField("source", "heapdump").WithField("role", role)
			log.WithField("dir", dir).Info("SIGUSR1 received, writing pprof profiles")
			files, err := Write(dir, role)
			if err != nil {
				log.WithError(err).WithField("written", files).Error("failed to write pprof profiles")
			} else {
				log.WithField("files", files).Info("wrote pprof profiles")
			}
			if onDump != nil {
				onDump()
			}
		}
	}()
}
