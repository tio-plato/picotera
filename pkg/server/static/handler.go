package static

import (
	"net/http"
	"path"
	"strconv"
	"strings"
)

func Handler() http.Handler {
	ensureAssets()
	return http.HandlerFunc(serve)
}

func serve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	target := resolveTarget(r.URL.Path)
	if target == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	a := lookup(target)
	isFallback := false
	if a == nil {
		a = indexAsset()
		isFallback = true
	}
	if a == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	if match := r.Header.Get("If-None-Match"); match != "" && etagMatches(match, a.etag) {
		w.Header().Set("ETag", a.etag)
		w.Header().Set("Cache-Control", cacheControl(target, isFallback))
		w.Header().Set("Vary", "Accept-Encoding")
		w.WriteHeader(http.StatusNotModified)
		return
	}

	encoding, body := negotiate(r.Header.Get("Accept-Encoding"), a)

	h := w.Header()
	h.Set("Content-Type", a.contentType)
	h.Set("Content-Length", strconv.Itoa(len(body)))
	h.Set("ETag", a.etag)
	h.Set("Vary", "Accept-Encoding")
	h.Set("Cache-Control", cacheControl(target, isFallback))
	if encoding != "" {
		h.Set("Content-Encoding", encoding)
	}

	status := http.StatusOK
	if isFallback {
		// SPA fallback: status stays 200 so the browser renders index.html
		// and Vue Router resolves the path client-side.
		status = http.StatusOK
	}
	w.WriteHeader(status)

	if r.Method == http.MethodHead {
		return
	}
	w.Write(body)
}

func resolveTarget(rawPath string) string {
	cleaned := path.Clean(rawPath)
	if strings.Contains(cleaned, "..") {
		return ""
	}
	if cleaned == "/" || cleaned == "." || cleaned == "" {
		return "/index.html"
	}
	if !strings.HasPrefix(cleaned, "/") {
		cleaned = "/" + cleaned
	}
	return cleaned
}

func cacheControl(target string, isFallback bool) string {
	if isFallback {
		return "no-cache"
	}
	if strings.HasPrefix(target, "/assets/") {
		return "public, max-age=31536000, immutable"
	}
	return "no-cache"
}

func negotiate(acceptEncoding string, a *asset) (encoding string, body []byte) {
	allowsBr, allowsGzip := parseAcceptEncoding(acceptEncoding)
	if allowsBr && a.brotli != nil {
		return "br", a.brotli
	}
	if allowsGzip && a.gzip != nil {
		return "gzip", a.gzip
	}
	return "", a.raw
}

func parseAcceptEncoding(header string) (br, gzip bool) {
	if header == "" {
		return false, false
	}
	for _, part := range strings.Split(header, ",") {
		token, q := parseEncodingToken(part)
		if q == 0 {
			continue
		}
		switch token {
		case "br":
			br = true
		case "gzip":
			gzip = true
		case "*":
			br = true
			gzip = true
		}
	}
	return
}

// parseEncodingToken returns the token and its q-value (default 1.0).
// q=0 disables the encoding.
func parseEncodingToken(part string) (token string, q float64) {
	q = 1.0
	part = strings.TrimSpace(part)
	if part == "" {
		return "", 0
	}
	if i := strings.Index(part, ";"); i >= 0 {
		token = strings.TrimSpace(part[:i])
		params := part[i+1:]
		for _, p := range strings.Split(params, ";") {
			p = strings.TrimSpace(p)
			if strings.HasPrefix(p, "q=") {
				if v, err := strconv.ParseFloat(p[2:], 64); err == nil {
					q = v
				}
			}
		}
	} else {
		token = part
	}
	token = strings.ToLower(token)
	return
}

func etagMatches(ifNoneMatch, etag string) bool {
	if ifNoneMatch == "*" {
		return true
	}
	for _, candidate := range strings.Split(ifNoneMatch, ",") {
		candidate = strings.TrimSpace(candidate)
		candidate = strings.TrimPrefix(candidate, "W/")
		if candidate == etag {
			return true
		}
	}
	return false
}
