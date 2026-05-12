package server

import (
	"net/http"
	"net/url"
	"sync"
)

// proxyTransportCache lazily creates and caches *http.Transport instances
// keyed by proxy configuration string.
type proxyTransportCache struct {
	base  *http.Transport
	mu    sync.RWMutex
	cache map[string]*http.Transport
}

func newProxyTransportCache(base *http.Transport) *proxyTransportCache {
	return &proxyTransportCache{
		base:  base,
		cache: make(map[string]*http.Transport),
	}
}

// get returns an http.Transport configured for the given proxy URL.
//   - "" (empty) → ProxyFromEnvironment (default behavior, uses base transport)
//   - "direct"   → no proxy; connect directly
//   - URL string → use that URL as the proxy (e.g. "http://proxy:8080")
func (c *proxyTransportCache) get(proxyURL string) *http.Transport {
	if proxyURL == "" {
		return c.base
	}

	c.mu.RLock()
	t, ok := c.cache[proxyURL]
	c.mu.RUnlock()
	if ok {
		return t
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	// Double-check after acquiring write lock.
	if t, ok = c.cache[proxyURL]; ok {
		return t
	}

	cloned := c.base.Clone()
	if proxyURL == "direct" {
		cloned.Proxy = nil // no proxy at all
	} else {
		parsed, err := url.Parse(proxyURL)
		if err != nil {
			// Invalid URL — fall back to environment proxy.
			// Shouldn't happen because API validation catches it.
			return c.base
		}
		cloned.Proxy = http.ProxyURL(parsed)
	}
	c.cache[proxyURL] = cloned
	return cloned
}
