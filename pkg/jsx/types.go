package jsx

import (
	"encoding/json"
	"fmt"
)

// marshalToJSLiteral encodes v to JSON. The result is also a valid JS
// expression literal (because JSON is a subset of JS expression syntax).
func marshalToJSLiteral(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("jsx: marshal: %w", err)
	}
	return string(b), nil
}

// unmarshalJSON decodes a JSON string returned by JS into out. Empty/null
// strings are treated as no-op.
func unmarshalJSON(s string, out any) error {
	if s == "" || s == "null" {
		return nil
	}
	if err := json.Unmarshal([]byte(s), out); err != nil {
		return fmt.Errorf("jsx: unmarshal: %w", err)
	}
	return nil
}
