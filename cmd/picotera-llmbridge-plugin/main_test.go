package main

import (
	"context"
	"io"
	"strings"
	"testing"

	"picotera/pkg/llmbridge"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRecoverUnaryConvertsPanicToInternal(t *testing.T) {
	resp, err := recoverUnary(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/llmbridge.LLMBridge/BridgeRequest"},
		func(context.Context, any) (any, error) { panic("boom") })
	if resp != nil {
		t.Fatalf("resp = %v, want nil", resp)
	}
	if status.Code(err) != codes.Internal || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v, want Internal containing panic value", err)
	}
}

func TestRecoverStreamConvertsPanicToInternal(t *testing.T) {
	err := recoverStream(nil, &fakeBridgeStream{ctx: context.Background()}, &grpc.StreamServerInfo{FullMethod: "/llmbridge.LLMBridge/BridgeStream"},
		func(any, grpc.ServerStream) error { panic("kaboom") })
	if status.Code(err) != codes.Internal || !strings.Contains(err.Error(), "kaboom") {
		t.Fatalf("err = %v, want Internal containing panic value", err)
	}
}

func TestRecoverUnaryPassesThroughNormalError(t *testing.T) {
	want := status.Error(codes.InvalidArgument, "nope")
	resp, err := recoverUnary(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/llmbridge.LLMBridge/BridgeRequest"},
		func(context.Context, any) (any, error) { return nil, want })
	if resp != nil || err != want {
		t.Fatalf("resp=%v err=%v, want nil and the original error", resp, err)
	}
}

func TestBridgeStreamFirstFrameMustBeStart(t *testing.T) {
	stream := &fakeBridgeStream{
		ctx: context.Background(),
		recv: []*llmbridge.BridgeStreamChunk{
			{Payload: &llmbridge.BridgeStreamChunk_Data{Data: []byte("bad")}},
		},
	}
	err := (&server{}).BridgeStream(stream)
	if status.Code(err) != codes.InvalidArgument || !strings.Contains(err.Error(), "first frame must be start") {
		t.Fatalf("err = %v", err)
	}
}

func TestStreamReaderRejectsDuplicateStart(t *testing.T) {
	stream := &fakeBridgeStream{
		ctx: context.Background(),
		recv: []*llmbridge.BridgeStreamChunk{
			{Payload: &llmbridge.BridgeStreamChunk_Start{Start: &llmbridge.BridgeStreamStart{}}},
		},
	}
	reader := newStreamReader(stream)
	_, err := reader.Read(make([]byte, 8))
	if err == nil || !strings.Contains(err.Error(), "duplicate start") {
		t.Fatalf("err = %v", err)
	}
}

func TestStreamReaderRejectsEmptyErrorFrame(t *testing.T) {
	stream := &fakeBridgeStream{
		ctx: context.Background(),
		recv: []*llmbridge.BridgeStreamChunk{
			{Payload: &llmbridge.BridgeStreamChunk_Error{Error: &llmbridge.BridgeStreamError{}}},
		},
	}
	reader := newStreamReader(stream)
	_, err := reader.Read(make([]byte, 8))
	if err == nil || !strings.Contains(err.Error(), "empty error frame") {
		t.Fatalf("err = %v", err)
	}
}

func TestStreamReaderDataThenEnd(t *testing.T) {
	stream := &fakeBridgeStream{
		ctx: context.Background(),
		recv: []*llmbridge.BridgeStreamChunk{
			{Payload: &llmbridge.BridgeStreamChunk_Data{Data: []byte("hello")}},
			{Payload: &llmbridge.BridgeStreamChunk_End{End: &llmbridge.BridgeStreamEnd{}}},
		},
	}
	reader := newStreamReader(stream)
	buf := make([]byte, 8)
	n, err := reader.Read(buf)
	if err != nil {
		t.Fatalf("Read data: %v", err)
	}
	if string(buf[:n]) != "hello" {
		t.Fatalf("data = %q", buf[:n])
	}
	_, err = reader.Read(buf)
	if err != io.EOF {
		t.Fatalf("second Read err = %v, want EOF", err)
	}
}

type fakeBridgeStream struct {
	llmbridge.LLMBridge_BridgeStreamServer
	ctx  context.Context
	recv []*llmbridge.BridgeStreamChunk
	sent []*llmbridge.BridgeStreamChunk
}

func (s *fakeBridgeStream) Context() context.Context {
	return s.ctx
}

func (s *fakeBridgeStream) Recv() (*llmbridge.BridgeStreamChunk, error) {
	if len(s.recv) == 0 {
		return nil, io.EOF
	}
	chunk := s.recv[0]
	s.recv = s.recv[1:]
	return chunk, nil
}

func (s *fakeBridgeStream) Send(chunk *llmbridge.BridgeStreamChunk) error {
	s.sent = append(s.sent, chunk)
	return nil
}
