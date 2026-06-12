package jsx

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"picotera/pkg/kv"
	"picotera/pkg/logx"

	"modernc.org/quickjs"
)

// fetchClient is the shared HTTP client used by picotera.fetch. The 5s
// timeout is the only backstop for a hook blocked in a host fetch call —
// SetEvalTimeout cannot interrupt a blocking host function.
var fetchClient = &http.Client{Timeout: 5 * time.Second}

// registerHelpers wires fetch / console / kv into the VM as synchronous host
// functions.
func registerHelpers(s *qjsSession) {
	registerFetch(s.vm)
	registerConsole(s)
	registerKV(s)
	registerRewriteBody(s)
}

// registerRewriteBody exposes the current rewriteRequest input body to JS as a
// raw JSON string via __picotera_rr_body(). The rewriteRequest hook defines
// pending.body as a lazy accessor that calls this and JSON.parses the result
// only when a hook actually reads or writes the body — so large untouched
// bodies are never parsed or re-serialized inside QuickJS.
func registerRewriteBody(s *qjsSession) {
	_ = s.vm.RegisterFunc("__picotera_rr_body", func() string {
		return s.rrBody
	}, false)
}

type fetchResponse struct {
	Status  int                 `json:"status"`
	Headers map[string][]string `json:"headers"`
	Body    string              `json:"body"`
}

// registerFetch exposes a synchronous __picotera_fetch(url, initJSON) host
// function. It returns (jsonBody, error); the multi-return surfaces in JS as
// the array [jsonBody, errOrNull], which the SDK turns into a parsed object or
// a thrown error.
func registerFetch(vm *quickjs.VM) {
	_ = vm.RegisterFunc("__picotera_fetch", func(url, initJSON string) (string, error) {
		resp, err := doFetch(url, initJSON)
		if err != nil {
			return "", err
		}
		b, _ := json.Marshal(resp)
		return string(b), nil
	}, false)
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

// registerKV exposes picotera.kv via __picotera_kv_* host functions. Functions
// that can fail return (value, error) so the SDK can throw on the error
// element; void operations return error (null on success). The session's
// request context bounds each operation.
func registerKV(s *qjsSession) {
	vm := s.vm
	store := s.engine.kvStore

	_ = vm.RegisterFunc("__picotera_kv_get", func(key string) (string, error) {
		val, err := store.Get(s.ctx, key)
		if err == kv.ErrKeyNotFound {
			return "", nil
		}
		if err != nil {
			return "", err
		}
		return val, nil
	}, false)

	_ = vm.RegisterFunc("__picotera_kv_set", func(key, value string) error {
		return store.Set(s.ctx, key, value)
	}, false)

	_ = vm.RegisterFunc("__picotera_kv_setex", func(key string, seconds int, value string) error {
		return store.SetEx(s.ctx, key, value, time.Duration(seconds)*time.Second)
	}, false)

	_ = vm.RegisterFunc("__picotera_kv_ttl", func(key string) (int, error) {
		ttl, err := store.TTL(s.ctx, key)
		if err != nil {
			return 0, err
		}
		return int(ttl), nil
	}, false)

	_ = vm.RegisterFunc("__picotera_kv_del", func(key string) error {
		return store.Del(s.ctx, key)
	}, false)
}

// registerConsole wires console.{log,info,warn,error} through __picotera_console
// to logx (tagged with the session's requestID) and appends a structured entry
// to the session's log buffer for inclusion in the meta response artifact.
func registerConsole(s *qjsSession) {
	_ = s.vm.RegisterFunc("__picotera_console", func(level, msg string) {
		entry := logx.New().WithField("source", "jsx").WithField("request_id", s.requestID)
		switch level {
		case "error":
			entry.Error(msg)
		case "warn":
			entry.Warn(msg)
		default:
			entry.Info(msg)
		}
		s.appendLog(level, msg)
	}, false)
}
