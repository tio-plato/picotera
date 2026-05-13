package jsx

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"picotera/pkg/kv"
	"picotera/pkg/logx"

	"github.com/fastschema/qjs"
)

// fetchClient is the shared HTTP client used by picotera.fetch. The 5s
// timeout matches the original spec.
var fetchClient = &http.Client{Timeout: 5 * time.Second}

// registerHelpers wires fetch / setTimeout / console / kv into the runtime.
func registerHelpers(s *Session) {
	c := s.rt.Context()
	registerFetch(c)
	registerSetTimeout(c)
	registerConsole(s)
	registerKV(s)
}

// registerFetch exposes picotera.fetch via __picotera_fetch (async).
// JS side wraps it: picotera.fetch(url, init?) → Promise<{status, headers, body}>.
func registerFetch(c *qjs.Context) {
	c.SetAsyncFunc("__picotera_fetch", func(this *qjs.This) {
		args := this.Args()
		var url, initJSON string
		if len(args) > 0 {
			url = args[0].String()
		}
		if len(args) > 1 {
			initJSON = args[1].String()
		}
		go func() {
			resp, ferr := doFetch(url, initJSON)
			ctx := this.Context()
			if ferr != nil {
				this.Promise().Reject(ctx.NewString(ferr.Error()))
				return
			}
			b, _ := json.Marshal(resp)
			this.Promise().Resolve(ctx.NewString(string(b)))
		}()
	})
}

type fetchResponse struct {
	Status  int                 `json:"status"`
	Headers map[string][]string `json:"headers"`
	Body    string              `json:"body"`
}

func doFetch(url, initJSON string) (*fetchResponse, error) {
	var init struct {
		Method  string            `json:"method"`
		Headers map[string]string `json:"headers"`
		Body    string            `json:"body"`
	}
	if initJSON != "" {
		_ = json.Unmarshal([]byte(initJSON), &init)
	}
	method := init.Method
	if method == "" {
		method = "GET"
	}
	req, err := http.NewRequest(method, url, strings.NewReader(init.Body))
	if err != nil {
		return nil, err
	}
	for k, v := range init.Headers {
		req.Header.Set(k, v)
	}
	resp, err := fetchClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return &fetchResponse{
		Status:  resp.StatusCode,
		Headers: resp.Header,
		Body:    string(body),
	}, nil
}

// registerSetTimeout exposes a Promise-based setTimeout via __picotera_setTimeout.
// JS side: globalThis.setTimeout(fn, ms) calls __picotera_setTimeout(ms).then(fn).
func registerSetTimeout(c *qjs.Context) {
	c.SetAsyncFunc("__picotera_setTimeout", func(this *qjs.This) {
		args := this.Args()
		var ms int32
		if len(args) > 0 {
			ms = args[0].Int32()
		}
		go func() {
			time.Sleep(time.Duration(ms) * time.Millisecond)
			this.Promise().Resolve(this.Context().NewUndefined())
		}()
	})
}

// registerKV exposes picotera.kv via __picotera_kv_* (synchronous).
func registerKV(s *Session) {
	c := s.rt.Context()
	store := s.engine.kvStore

	c.SetFunc("__picotera_kv_get", func(this *qjs.This) (*qjs.Value, error) {
		args := this.Args()
		var key string
		if len(args) > 0 {
			key = args[0].String()
		}
		val, err := store.Get(this.Context(), key)
		if err == kv.ErrKeyNotFound {
			return this.Context().NewNull(), nil
		}
		if err != nil {
			return nil, err
		}
		return this.Context().NewString(val), nil
	})

	c.SetFunc("__picotera_kv_set", func(this *qjs.This) (*qjs.Value, error) {
		args := this.Args()
		var key, value string
		if len(args) > 0 {
			key = args[0].String()
		}
		if len(args) > 1 {
			value = args[1].String()
		}
		err := store.Set(this.Context(), key, value)
		if err != nil {
			return nil, err
		}
		return this.Context().NewUndefined(), nil
	})

	c.SetFunc("__picotera_kv_setex", func(this *qjs.This) (*qjs.Value, error) {
		args := this.Args()
		var key string
		var seconds int32
		var value string
		if len(args) > 0 {
			key = args[0].String()
		}
		if len(args) > 1 {
			seconds = args[1].Int32()
		}
		if len(args) > 2 {
			value = args[2].String()
		}
		err := store.SetEx(this.Context(), key, value, time.Duration(seconds)*time.Second)
		if err != nil {
			return nil, err
		}
		return this.Context().NewUndefined(), nil
	})

	c.SetFunc("__picotera_kv_ttl", func(this *qjs.This) (*qjs.Value, error) {
		args := this.Args()
		var key string
		if len(args) > 0 {
			key = args[0].String()
		}
		ttl, err := store.TTL(this.Context(), key)
		if err != nil {
			return nil, err
		}
		return this.Context().NewInt32(int32(ttl)), nil
	})

	c.SetFunc("__picotera_kv_del", func(this *qjs.This) (*qjs.Value, error) {
		args := this.Args()
		var key string
		if len(args) > 0 {
			key = args[0].String()
		}
		err := store.Del(this.Context(), key)
		if err != nil {
			return nil, err
		}
		return this.Context().NewUndefined(), nil
	})
}

// registerConsole wires console.{log,info,warn,error} through __picotera_console
// to logx (tagged with the session's requestID) and appends a structured
// entry to the session's log buffer for inclusion in the meta response artifact.
func registerConsole(s *Session) {
	c := s.rt.Context()
	c.SetFunc("__picotera_console", func(this *qjs.This) (*qjs.Value, error) {
		args := this.Args()
		var level, msg string
		if len(args) > 0 {
			level = args[0].String()
		}
		if len(args) > 1 {
			msg = args[1].String()
		}
		entry := logx.New().WithField("source", "jsx").WithField("request_id", s.requestID)
		switch level {
		case "error":
			entry.Error(msg)
		case "warn":
			entry.Warn(msg)
		case "info":
			entry.Info(msg)
		default:
			entry.Info(msg)
		}
		s.appendLog(level, msg)
		return this.Context().NewUndefined(), nil
	})
}
