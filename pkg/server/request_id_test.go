package server

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/rs/xid"
)

func TestNewRequestIDReturnsXIDTimestamp(t *testing.T) {
	id, createdAt := newRequestID()
	parsed, err := xid.FromString(id)
	if err != nil {
		t.Fatalf("parse generated id: %v", err)
	}
	if !createdAt.Equal(parsed.Time().UTC()) {
		t.Fatalf("createdAt = %s, want %s", createdAt, parsed.Time().UTC())
	}
	if createdAt.Location() != time.UTC {
		t.Fatalf("createdAt location = %s, want UTC", createdAt.Location())
	}
}

func TestRequestIDCreatedAtParsesStrictLowercaseXID(t *testing.T) {
	id := xid.New()
	got, err := requestIDCreatedAt(id.String())
	if err != nil {
		t.Fatalf("requestIDCreatedAt returned error: %v", err)
	}
	if !got.Equal(id.Time().UTC()) {
		t.Fatalf("createdAt = %s, want %s", got, id.Time().UTC())
	}
	if got.Location() != time.UTC {
		t.Fatalf("createdAt location = %s, want UTC", got.Location())
	}
}

func TestRequestIDCreatedAtRejectsInvalidIDs(t *testing.T) {
	valid := xid.New().String()
	tests := []string{
		"",
		" " + valid,
		valid + " ",
		strings.ToUpper(valid),
		valid[:19],
		valid + "0",
		strings.Repeat("w", 20),
	}
	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			_, err := requestIDCreatedAt(tt)
			if err == nil {
				t.Fatal("expected error")
			}
			var statusErr huma.StatusError
			if !errors.As(err, &statusErr) {
				t.Fatalf("error type = %T, want huma.StatusError", err)
			}
			if statusErr.GetStatus() != 400 {
				t.Fatalf("status = %d, want 400", statusErr.GetStatus())
			}
		})
	}
}
