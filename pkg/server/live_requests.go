package server

import (
	"bytes"
	"context"
	"sync"
	"sync/atomic"
	"time"

	cmap "github.com/orcaman/concurrent-map/v2"
)

// liveRequestRegistry tracks in-flight request rows (meta + each upstream
// attempt) for the lifetime of a single gateway call. It serves two purposes:
//
//  1. Interrupt: each entry holds the cancel func for its context, so the
//     management API can cancel an in-flight row. Control-flow differences
//     (continue to the next provider vs. abort the stream) fall out naturally
//     from whether cancellation happens before or after headers arrive.
//  2. Live status: each upstream entry carries an in-memory progress snapshot
//     (headers received, byte count, body-so-far). The meta entry mirrors the
//     progress of the upstream that is currently streaming.
//
// Everything lives in process memory only — single instance, released when the
// request finishes. The concurrent map protects map structure; liveProgress
// keeps its own mutex because its bytes.Buffer is written by the streaming
// goroutine and read by the API goroutine concurrently.
type liveRequestRegistry struct {
	entries cmap.ConcurrentMap[string, *liveEntry]
}

func newLiveRequestRegistry() *liveRequestRegistry {
	return &liveRequestRegistry{entries: cmap.New[*liveEntry]()}
}

type liveEntryKind int

const (
	liveKindMeta liveEntryKind = iota
	liveKindUpstream
)

type liveEntry struct {
	kind   liveEntryKind
	cancel context.CancelFunc

	// progress is this row's own progress. Upstream rows have a non-nil
	// progress; meta rows leave it nil and mirror the active upstream instead.
	progress *liveProgress

	// active points at the progress of the upstream attempt currently
	// streaming back to the client. Meta rows only; nil until an upstream
	// reaches the header-received stage.
	active atomic.Pointer[liveProgress]

	// stopReason is the finish reason written when the dashboard interrupts
	// this row (0 means not interrupted).
	stopReason atomic.Int32
}

type liveProgress struct {
	mu              sync.Mutex
	headersReceived bool
	statusCode      int
	bytes           int64
	body            bytes.Buffer
	startedAt       time.Time
	lastChunkAt     time.Time
}

func newLiveProgress() *liveProgress {
	return &liveProgress{startedAt: time.Now()}
}

func (p *liveProgress) markHeaders(statusCode int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.headersReceived = true
	p.statusCode = statusCode
}

func (p *liveProgress) recordChunk(b []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.body.Write(b)
	p.bytes += int64(len(b))
	p.lastChunkAt = time.Now()
}

// RegisterMeta registers the meta row keyed by id with its flow cancel func.
func (r *liveRequestRegistry) RegisterMeta(id string, cancel context.CancelFunc) *liveEntry {
	e := &liveEntry{kind: liveKindMeta, cancel: cancel}
	r.entries.Set(id, e)
	return e
}

// RegisterUpstream registers an upstream attempt row keyed by id with its
// attempt cancel func and a fresh progress tracker.
func (r *liveRequestRegistry) RegisterUpstream(id string, cancel context.CancelFunc) *liveEntry {
	e := &liveEntry{kind: liveKindUpstream, cancel: cancel, progress: newLiveProgress()}
	r.entries.Set(id, e)
	return e
}

func (r *liveRequestRegistry) Remove(id string) {
	r.entries.Remove(id)
}

// get returns the entry for id, if registered. Used by streaming success
// paths to wire the meta row's active pointer to a streaming upstream.
func (r *liveRequestRegistry) get(id string) (*liveEntry, bool) {
	return r.entries.Get(id)
}

// Interrupt cancels the entry's context and records the stop reason. Returns
// false when no in-flight entry exists for id (a race-condition no-op).
func (r *liveRequestRegistry) Interrupt(id string, reason int32) bool {
	e, ok := r.entries.Get(id)
	if !ok {
		return false
	}
	e.stopReason.Store(reason)
	e.cancel()
	return true
}

// StopReason returns the stop reason recorded for id, or 0 if absent.
func (r *liveRequestRegistry) StopReason(id string) int32 {
	e, ok := r.entries.Get(id)
	if !ok {
		return 0
	}
	return e.stopReason.Load()
}

type liveSnapshot struct {
	InFlight        bool
	Kind            liveEntryKind
	HeadersReceived bool
	StatusCode      int
	Bytes           int64
	Body            string
	StartedAt       time.Time
	LastChunkAt     time.Time
}

// Snapshot returns the in-memory progress of id. For meta rows it follows the
// active pointer to the streaming upstream's progress (pending stage when no
// upstream is active yet). Returns inFlight=false when id is not registered.
func (r *liveRequestRegistry) Snapshot(id string) (liveSnapshot, bool) {
	e, ok := r.entries.Get(id)
	if !ok {
		return liveSnapshot{}, false
	}
	snap := liveSnapshot{InFlight: true, Kind: e.kind}
	prog := e.progress
	if e.kind == liveKindMeta {
		prog = e.active.Load()
	}
	if prog == nil {
		// Meta row with no streaming upstream yet: pending stage.
		return snap, true
	}
	prog.mu.Lock()
	snap.HeadersReceived = prog.headersReceived
	snap.StatusCode = prog.statusCode
	snap.Bytes = prog.bytes
	snap.Body = prog.body.String()
	snap.StartedAt = prog.startedAt
	snap.LastChunkAt = prog.lastChunkAt
	prog.mu.Unlock()
	return snap, true
}
