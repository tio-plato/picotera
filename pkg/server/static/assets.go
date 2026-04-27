package static

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"mime"
	"path"
	"strings"
	"sync"

	"github.com/andybalholm/brotli"
)

type asset struct {
	contentType string
	raw         []byte
	gzip        []byte
	brotli      []byte
	etag        string
}

const (
	compressMinBytes  = 1024
	compressMinSaving = 0.10 // require at least 10% smaller
)

var (
	assetsOnce sync.Once
	assets     map[string]*asset
)

var contentTypeOverrides = map[string]string{
	".js":   "text/javascript; charset=utf-8",
	".mjs":  "text/javascript; charset=utf-8",
	".css":  "text/css; charset=utf-8",
	".svg":  "image/svg+xml",
	".json": "application/json",
	".html": "text/html; charset=utf-8",
}

var compressibleTypes = map[string]bool{
	"application/javascript": true,
	"application/json":       true,
	"application/wasm":       true,
	"image/svg+xml":          true,
}

func ensureAssets() {
	assetsOnce.Do(func() {
		built := map[string]*asset{}
		sub, err := fs.Sub(distFS, "dist")
		if err != nil {
			assets = built
			return
		}
		fs.WalkDir(sub, ".", func(p string, d fs.DirEntry, werr error) error {
			if werr != nil || d.IsDir() {
				return nil
			}
			raw, err := fs.ReadFile(sub, p)
			if err != nil {
				return nil
			}
			ext := strings.ToLower(path.Ext(p))
			ct := contentTypeOverrides[ext]
			if ct == "" {
				ct = mime.TypeByExtension(ext)
			}
			if ct == "" {
				ct = "application/octet-stream"
			}
			a := &asset{contentType: ct, raw: raw, etag: makeETag(raw)}
			if shouldCompress(ct, len(raw)) {
				if gz, ok := tryGzip(raw); ok {
					a.gzip = gz
				}
				if br, ok := tryBrotli(raw); ok {
					a.brotli = br
				}
			}
			built["/"+p] = a
			return nil
		})
		assets = built
	})
}

func shouldCompress(contentType string, size int) bool {
	if size < compressMinBytes {
		return false
	}
	mt := contentType
	if i := strings.Index(mt, ";"); i >= 0 {
		mt = strings.TrimSpace(mt[:i])
	}
	if strings.HasPrefix(mt, "text/") {
		return true
	}
	return compressibleTypes[mt]
}

func tryGzip(raw []byte) ([]byte, bool) {
	var buf bytes.Buffer
	w, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, false
	}
	if _, err := w.Write(raw); err != nil {
		return nil, false
	}
	if err := w.Close(); err != nil {
		return nil, false
	}
	return acceptCompressed(raw, buf.Bytes())
}

func tryBrotli(raw []byte) ([]byte, bool) {
	var buf bytes.Buffer
	w := brotli.NewWriterLevel(&buf, brotli.BestCompression)
	if _, err := w.Write(raw); err != nil {
		return nil, false
	}
	if err := w.Close(); err != nil {
		return nil, false
	}
	return acceptCompressed(raw, buf.Bytes())
}

func acceptCompressed(raw, compressed []byte) ([]byte, bool) {
	if len(compressed) >= len(raw) {
		return nil, false
	}
	if float64(len(compressed)) > float64(len(raw))*(1-compressMinSaving) {
		return nil, false
	}
	out := make([]byte, len(compressed))
	copy(out, compressed)
	return out, true
}

func makeETag(raw []byte) string {
	sum := sha256.Sum256(raw)
	return `"` + hex.EncodeToString(sum[:8]) + `"`
}

func lookup(target string) *asset {
	ensureAssets()
	return assets[target]
}

func indexAsset() *asset {
	ensureAssets()
	return assets["/index.html"]
}
