package llmbridgeimpl

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/looplj/axonhub/llm/httpclient"
)

// TestDirtySSEFinalEventDoesNotPanic is a regression test for a production
// SIGSEGV: an SSE stream whose final event is not terminated by a blank line
// ("dirty" at EOF). go-sse's Stream.Recv used to yield the final event with a
// nil error while nil-ing its internal scanner, so the idiomatic
// `for decoder.Next()` loop called Recv once more and dereferenced nil. The
// vendored fork (third_party/go-sse) now latches the stream closed in that
// branch (and the parser guards against a nil scanner), so the loop must
// terminate cleanly and still observe the final dirty event.
func TestDirtySSEFinalEventDoesNotPanic(t *testing.T) {
	// No trailing "\n\n" after the last event.
	body := "data: {\"a\":1}\n\ndata: {\"b\":2}\n"
	dec := httpclient.NewDefaultSSEDecoder(context.Background(), io.NopCloser(strings.NewReader(body)))
	defer dec.Close()

	var events int
	for dec.Next() {
		events++
	}
	if err := dec.Err(); err != nil {
		t.Fatalf("decoder Err: %v", err)
	}
	if events != 2 {
		t.Fatalf("events = %d, want 2 (final dirty event should still be delivered)", events)
	}

	// Calling Next again after the loop ended must stay false, never panic.
	if dec.Next() {
		t.Fatalf("Next returned true after stream exhaustion")
	}
}
