package server

import (
	"context"
	"time"

	"picotera/pkg/contract"
	"picotera/pkg/db"
)

// handleInterruptRequest interrupts the in-flight processing of a request row
// (meta or upstream). Returns interrupted=false when the row is no longer
// in-flight (a race-condition no-op), not an error.
func (s *Server) handleInterruptRequest(ctx context.Context, input *contract.InterruptRequestRequest) (*contract.InterruptRequestResponse, error) {
	if _, err := requestIDCreatedAt(input.ID); err != nil {
		return nil, err
	}
	interrupted := s.liveRequests.Interrupt(input.ID, db.FinishReasonDashboardCancelled)
	resp := &contract.InterruptRequestResponse{}
	resp.Body.Interrupted = interrupted
	return resp, nil
}

// handleGetRequestLive returns the in-memory live progress of a request row.
// Rows that have finished or were never in this process return inFlight=false.
func (s *Server) handleGetRequestLive(ctx context.Context, input *contract.GetRequestLiveRequest) (*contract.GetRequestLiveResponse, error) {
	if _, err := requestIDCreatedAt(input.ID); err != nil {
		return nil, err
	}
	snap, ok := s.liveRequests.Snapshot(input.ID)
	view := contract.RequestLiveView{}
	if !ok {
		return &contract.GetRequestLiveResponse{Body: view}, nil
	}
	view.InFlight = true
	switch snap.Kind {
	case liveKindMeta:
		view.Kind = "meta"
	case liveKindUpstream:
		view.Kind = "upstream"
	}
	view.HeadersReceived = snap.HeadersReceived
	view.StatusCode = snap.StatusCode
	view.BytesReceived = snap.Bytes
	view.Body = snap.Body
	view.Timings = snap.Timings
	switch {
	case !snap.HeadersReceived:
		view.Phase = "pending"
	case snap.Bytes > 0:
		view.Phase = "streaming"
	default:
		view.Phase = "headerReceived"
	}
	if !snap.StartedAt.IsZero() {
		view.StartedAt = snap.StartedAt.UTC().Format(time.RFC3339Nano)
	}
	if !snap.LastChunkAt.IsZero() {
		view.LastChunkAt = snap.LastChunkAt.UTC().Format(time.RFC3339Nano)
	}
	return &contract.GetRequestLiveResponse{Body: view}, nil
}
