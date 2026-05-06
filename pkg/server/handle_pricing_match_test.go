package server

import (
	"context"
	"testing"

	"picotera/pkg/contract"

	"github.com/danielgtaylor/huma/v2"
)

func TestHandleMatchPricingRejectsEmptyTarget(t *testing.T) {
	s := &Server{}
	input := &contract.MatchPricingRequest{}

	_, err := s.handleMatchPricing(context.Background(), input)
	if err == nil {
		t.Fatal("expected error")
	}

	status, ok := err.(huma.StatusError)
	if !ok {
		t.Fatalf("expected huma status error, got %T", err)
	}
	if status.GetStatus() != 400 {
		t.Fatalf("status = %d, want 400", status.GetStatus())
	}
}
