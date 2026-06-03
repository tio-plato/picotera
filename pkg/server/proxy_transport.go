package server

import (
	"net/http"
	"net/url"
	"sync"
)

// transportKey identifies a cached *http.Transport by its proxy configuration
// and whether it carries the streaming header timeout.
type transportKey struct {
	proxy     string
	streaming bool
}

// proxyTransportCache lazily creates and caches *http.Transport instances
// keyed by (proxy configuration, streaming) — streaming and non-streaming
// requests use bases with different ResponseHeaderTimeout.
type proxyTransportCache struct {
	streamBase    *http.Transport
	nonStreamBase *http.Transport
	mu            sync.RWMutex
	cache         map[transportKey]*http.Transport
}

func newProxyTransportCache(streamBase, nonStreamBase *http.Transport) *proxyTransportCache {
	return &proxyTransportCache{
		streamBase:    streamBase,
		nonStreamBase: nonStreamBase,
		cache:         make(map[transportKey]*http.Transport),
	}
}

func (c *proxyTransportCache) base(streaming bool) *http.Transport {
	if streaming {
		return c.streamBase
	}
	return c.nonStreamBase
}

// get returns an http.Transport configured for the given proxy URL and
// streaming flag.
//   - "" (empty) → ProxyFromEnvironment (default behavior, uses base transport)
//   - "direct"   → no proxy; connect directly
//   - URL string → use that URL as the proxy (e.g. "http://proxy:8080")
//
// The streaming flag selects between the streaming and non-streaming bases,
// which differ only in ResponseHeaderTimeout.
func (c *proxyTransportCache) get(proxyURL string, streaming bool) *http.Transport {
	base := c.base(streaming)
	if proxyURL == "" {
		return base
	}

	key := transportKey{proxy: proxyURL, streaming: streaming}
	c.mu.RLock()
	t, ok := c.cache[key]
	c.mu.RUnlock()
	if ok {
		return t
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	// Double-check after acquiring write lock.
	if t, ok = c.cache[key]; ok {
		return t
	}

	cloned := base.Clone()
	if proxyURL == "direct" {
		cloned.Proxy = nil // no proxy at all
	} else {
		parsed, err := url.Parse(proxyURL)
		if err != nil {
			// Invalid URL — fall back to environment proxy.
			// Shouldn't happen because API validation catches it.
			return base
		}
		cloned.Proxy = http.ProxyURL(parsed)
	}
	c.cache[key] = cloned
	return cloned
}
