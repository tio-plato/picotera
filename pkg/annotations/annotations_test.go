package annotations

import "testing"

func TestDecode_EmptyInputs(t *testing.T) {
	cases := [][]byte{nil, []byte(""), []byte("null"), []byte("{}")}
	for _, raw := range cases {
		got, err := Decode(raw)
		if err != nil {
			t.Fatalf("Decode(%q) err: %v", string(raw), err)
		}
		if got == nil {
			t.Fatalf("Decode(%q) returned nil map", string(raw))
		}
		if len(got) != 0 {
			t.Fatalf("Decode(%q) expected empty map, got %v", string(raw), got)
		}
	}
}

func TestDecode_NonObjectErrors(t *testing.T) {
	for _, raw := range []string{`[]`, `"x"`, `1`, `true`} {
		if _, err := Decode([]byte(raw)); err == nil {
			t.Fatalf("Decode(%q) expected error", raw)
		}
	}
}

func TestDecode_CoercesNonStringValues(t *testing.T) {
	got, err := Decode([]byte(`{"a":1,"b":true,"c":null,"d":1.5,"e":"x"}`))
	if err != nil {
		t.Fatalf("Decode err: %v", err)
	}
	want := map[string]string{
		"a": "1",
		"b": "true",
		"c": "",
		"d": "1.5",
		"e": "x",
	}
	if len(got) != len(want) {
		t.Fatalf("size mismatch: got %v want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("key %q: got %q want %q", k, got[k], v)
		}
	}
}

func TestMerge_LaterWins(t *testing.T) {
	model := map[string]string{"a": "1", "shared": "model"}
	provider := map[string]string{"b": "2", "shared": "provider"}
	entry := map[string]string{"c": "3", "shared": "entry"}
	apiKey := map[string]string{"d": "4", "shared": "apiKey"}

	got := Merge(model, provider, entry, apiKey)
	want := map[string]string{
		"a": "1", "b": "2", "c": "3", "d": "4", "shared": "apiKey",
	}
	if len(got) != len(want) {
		t.Fatalf("size mismatch: got %v want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("key %q: got %q want %q", k, got[k], v)
		}
	}
}

func TestMerge_EmptyAndNilProduceNonNil(t *testing.T) {
	if got := Merge(); got == nil {
		t.Fatal("Merge() returned nil")
	}
	if got := Merge(nil, nil, nil); got == nil || len(got) != 0 {
		t.Fatalf("Merge(nil...) want empty non-nil map, got %v", got)
	}
}
