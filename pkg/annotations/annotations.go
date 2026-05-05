// Package annotations provides the merge/decode helpers that produce the
// per-request annotations map exposed to JS hooks. Annotations originate from
// four layers — model, provider, provider_models[] entry, and api_key — and
// are flattened into a single map[string]string with later layers winning on
// key conflict.
package annotations

import (
	"encoding/json"
	"fmt"
)

// Merge returns a new map produced by overlaying each layer in order. Later
// layers win on key conflict. Nil layers are skipped. The result is never
// nil; an empty merge yields an empty (allocated) map so JSON encoding
// produces "{}" instead of "null".
func Merge(layers ...map[string]string) map[string]string {
	out := map[string]string{}
	for _, layer := range layers {
		for k, v := range layer {
			out[k] = v
		}
	}
	return out
}

// Decode parses JSONB bytes into map[string]string. nil/empty/"null"/"{}"
// all yield an empty map. Non-object JSON returns an error. Non-string
// values are coerced via fmt.Sprint to keep the surface flat (matches how
// the dashboard's AnnotationsEditor stores everything as strings).
func Decode(raw []byte) (map[string]string, error) {
	if len(raw) == 0 {
		return map[string]string{}, nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("annotations: decode: %w", err)
	}
	if v == nil {
		return map[string]string{}, nil
	}
	obj, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("annotations: expected object, got %T", v)
	}
	out := make(map[string]string, len(obj))
	for k, val := range obj {
		switch s := val.(type) {
		case string:
			out[k] = s
		case nil:
			out[k] = ""
		default:
			out[k] = fmt.Sprint(val)
		}
	}
	return out, nil
}
