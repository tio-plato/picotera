package jsx

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"picotera/pkg/db"
	"picotera/pkg/kv"
)

// mockMessagesBody builds an Anthropic-Messages-shaped request body of roughly
// targetBytes, modelling a real multimodal request: the bulk is base64 image
// data (pure ASCII, i.e. a high character count per byte) and the conversation
// text carries some non-ASCII (CJK) content.
//
// That mix is what reproduces the production failure. QuickJS stores any string
// containing a non-ASCII character as a wide (UTF-16, 2 bytes/char) string, so
// the multi-MiB serialized body becomes a wide string; converting it back to a
// UTF-8 C buffer in Value.MarshalJSON allocates the worst case (up to 3 bytes
// per UTF-16 unit), and that large contiguous allocation tips the round-trip
// over the memory limit. An all-ASCII body of the same size does NOT trigger it.
func mockMessagesBody(targetBytes int) []byte {
	imageBytes := targetBytes * 85 / 100
	image := strings.Repeat("iVBORw0KGgoAAAANSUhEUgAB", imageBytes/24+1)[:imageBytes]

	textReps := (targetBytes - imageBytes) / len("这是详细的分析说明文字。")
	msgs := []any{
		map[string]any{"role": "user", "content": []any{
			map[string]any{"type": "text", "text": "请分析这张图片的内容并总结要点。"},
			map[string]any{"type": "image", "source": map[string]any{
				"type": "base64", "media_type": "image/png", "data": image,
			}},
		}},
		map[string]any{"role": "assistant", "content": "好的，我来分析。" + strings.Repeat("这是详细的分析说明文字。", textReps)},
	}
	out, _ := json.Marshal(map[string]any{
		"model":      "claude-opus-4-6",
		"max_tokens": 4096,
		"messages":   msgs,
	})
	return out
}

// TestSession_RewriteRequest_LargeBody guards the requirement that the
// rewriteRequest waterfall handles large request bodies (up to 30 MiB) under
// the production default JS memory limit (64 MiB; see configx js_memory_limit).
//
// The hooks here do NOT touch the body — they only inspect/replace url/headers,
// which is the common case. A large body should pass through untouched without
// being re-parsed and re-serialized.
//
// Before the fix this fails. RunRewriteRequest unconditionally embeds the whole
// body into the eval source, parses it to a JS object, re-stringifies it, then
// marshals the entire shape back; the round-trip peak reaches several multiples
// of the body size. Around ~10 MiB it surfaces as
//
//	jsx: rewriteRequest decode: unexpected end of JSON input
//
// (the outer JSON.stringify succeeds but the final C-string allocation for the
// wide result string fails, and the QuickJS binding silently returns empty
// bytes); larger bodies fail earlier as "InternalError: out of memory". Both
// are the same root cause.
func TestSession_RewriteRequest_LargeBody(t *testing.T) {
	const memLimit = 64 * 1024 * 1024 // production default

	hooks := []struct {
		name   string
		source string
	}{
		{"noop", `picotera.hooks.rewriteRequest.tap("a", function () {});`},
		{"rewrite-headers", `picotera.hooks.rewriteRequest.tap("a", function (ctx, p) {
			p.headers["x-extra"] = ["1"];
			p.url = p.url + "?routed=1";
			return p;
		});`},
	}

	for _, mb := range []int{1, 10, 20, 30} {
		for _, h := range hooks {
			t.Run(fmt.Sprintf("%dMiB/%s", mb, h.name), func(t *testing.T) {
				body := mockMessagesBody(mb * 1024 * 1024)
				eng := NewEngine(
					Config{HookTimeout: 30 * time.Second, MemoryLimit: memLimit},
					&fakeStore{scripts: []db.Script{{ID: "a", Source: h.source}}},
					kv.NewMemoryStore(),
				)
				s, err := eng.NewSession(context.Background(), "t")
				if err != nil {
					t.Fatalf("NewSession: %v", err)
				}
				defer s.Close()

				in := PendingRequestShape{
					URL:     "https://upstream/v1/messages",
					Method:  "POST",
					Headers: map[string][]string{"content-type": {"application/json"}},
					Body:    json.RawMessage(body),
				}
				out, err := s.RunRewriteRequest(in)
				if err != nil {
					t.Fatalf("RunRewriteRequest (%d MiB, %s): %v", mb, h.name, err)
				}

				// Body is returned as a JSON string token; the inner string must
				// equal the original body bytes byte-for-byte (untouched body).
				var inner string
				if err := json.Unmarshal(out.Body, &inner); err != nil {
					t.Fatalf("decode body token (%d MiB, %s): %v (out.Body len=%d)", mb, h.name, err, len(out.Body))
				}
				if inner != string(body) {
					t.Fatalf("body round-trip mismatch (%d MiB, %s): got len=%d want len=%d", mb, h.name, len(inner), len(body))
				}
			})
		}
	}
}
