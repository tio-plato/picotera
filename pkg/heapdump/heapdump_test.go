package heapdump

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestWrite(t *testing.T) {
	dir := t.TempDir()

	files, err := Write(dir, "host")
	if err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if len(files) != 3 {
		t.Fatalf("expected 3 files, got %d: %v", len(files), files)
	}

	namePattern := regexp.MustCompile(`^picotera-host-\d{8}T\d{6}-(heap|allocs|goroutine)\.pprof$`)
	kinds := map[string]bool{}
	for _, path := range files {
		base := filepath.Base(path)
		m := namePattern.FindStringSubmatch(base)
		if m == nil {
			t.Errorf("file name %q does not match expected pattern", base)
			continue
		}
		kinds[m[1]] = true

		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("stat %s: %v", path, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("profile %s is empty", path)
		}
	}

	for _, kind := range []string{"heap", "allocs", "goroutine"} {
		if !kinds[kind] {
			t.Errorf("missing %s profile", kind)
		}
	}
}
