//go:build wasip1

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sync"
	"unsafe"

	"picotera/pkg/llmbridge"
	"picotera/pkg/llmbridgeimpl"
)

const abiVersion = 1

const (
	streamStatusOK uint32 = iota
	streamStatusEOF
	streamStatusError
)

var (
	allocMu sync.Mutex
	allocs  = map[uint32][]byte{}

	streamMu     sync.Mutex
	nextStreamID uint32 = 1
	streams             = map[uint32]llmbridgeimpl.StreamBridge{}
)

//go:wasmimport picotera_llmbridge_host llmbridge_stream_read
func hostStreamRead(ptr, cap uint32) uint64

//go:wasmimport picotera_llmbridge_host llmbridge_stream_write
func hostStreamWrite(ptr, len uint32) uint32

func main() {}

//go:wasmexport llmbridge_abi_version
func llmbridgeABIVersion() uint32 {
	return abiVersion
}

//go:wasmexport llmbridge_alloc
func llmbridgeAlloc(n uint32) uint32 {
	if n == 0 {
		n = 1
	}
	b := make([]byte, n)
	ptr := uint32(uintptr(unsafe.Pointer(&b[0])))
	allocMu.Lock()
	allocs[ptr] = b
	allocMu.Unlock()
	return ptr
}

//go:wasmexport llmbridge_free
func llmbridgeFree(ptr uint32) {
	allocMu.Lock()
	delete(allocs, ptr)
	allocMu.Unlock()
}

//go:wasmexport llmbridge_bridge_request
func llmbridgeBridgeRequest(ptr, n uint32) uint64 {
	var req bridgeRequestEnvelope
	if err := decodeInput(ptr, n, &req); err != nil {
		return encodeOutput(operationResponse{OK: false, Error: err.Error()})
	}
	body, ct, err := llmbridgeimpl.BridgeRequest(context.Background(), req.Src, req.Dst, req.Body, http.Header(req.Headers), req.PendingURL, req.Profile)
	if err != nil {
		return encodeOutput(operationResponse{OK: false, Error: err.Error()})
	}
	return encodeOutput(operationResponse{OK: true, Body: body, ContentType: ct})
}

//go:wasmexport llmbridge_bridge_non_stream
func llmbridgeBridgeNonStream(ptr, n uint32) uint64 {
	var req bridgeNonStreamEnvelope
	if err := decodeInput(ptr, n, &req); err != nil {
		return encodeOutput(operationResponse{OK: false, Error: err.Error()})
	}
	body, ct, err := llmbridgeimpl.BridgeNonStream(context.Background(), req.Src, req.Upstream, req.Body, http.Header(req.Headers), req.Profile)
	if err != nil {
		return encodeOutput(operationResponse{OK: false, Error: err.Error()})
	}
	return encodeOutput(operationResponse{OK: true, Body: body, ContentType: ct})
}

//go:wasmexport llmbridge_bridge_stream_open
func llmbridgeBridgeStreamOpen(ptr, n uint32) uint64 {
	var req bridgeStreamOpenEnvelope
	if err := decodeInput(ptr, n, &req); err != nil {
		return encodeOutput(operationResponse{OK: false, Error: err.Error()})
	}
	stream, err := llmbridgeimpl.OpenStream(context.Background(), req.Src, req.Upstream, hostReader{}, req.ContentType, req.Profile)
	if err != nil {
		return encodeOutput(operationResponse{OK: false, Error: err.Error()})
	}
	id := storeStream(stream)
	return encodeOutput(operationResponse{OK: true, StreamID: id})
}

//go:wasmexport llmbridge_bridge_stream_pump
func llmbridgeBridgeStreamPump(streamID uint32) uint64 {
	stream := getStream(streamID)
	if stream == nil {
		return encodeOutput(operationResponse{OK: false, Error: fmt.Sprintf("llmbridge: unknown stream id %d", streamID)})
	}
	if err := stream.Pump(context.Background(), hostWriter{}); err != nil {
		return encodeOutput(operationResponse{OK: false, Error: err.Error()})
	}
	return encodeOutput(operationResponse{OK: true})
}

//go:wasmexport llmbridge_bridge_stream_close
func llmbridgeBridgeStreamClose(streamID uint32) uint64 {
	stream := deleteStream(streamID)
	if stream == nil {
		return encodeOutput(operationResponse{OK: false, Error: fmt.Sprintf("llmbridge: unknown stream id %d", streamID)})
	}
	if err := stream.Close(); err != nil {
		return encodeOutput(operationResponse{OK: false, Error: err.Error()})
	}
	return encodeOutput(operationResponse{OK: true})
}

//go:wasmexport llmbridge_aggregate_stream
func llmbridgeAggregateStream(ptr, n uint32) uint64 {
	var req aggregateStreamEnvelope
	if err := decodeInput(ptr, n, &req); err != nil {
		return encodeOutput(operationResponse{OK: false, Error: err.Error()})
	}
	body, err := llmbridgeimpl.AggregateStream(context.Background(), req.Format, req.ContentType, req.Body, req.Profile)
	if err != nil {
		return encodeOutput(operationResponse{OK: false, Error: err.Error()})
	}
	return encodeOutput(operationResponse{OK: true, Body: body})
}

type bridgeRequestEnvelope struct {
	Src        llmbridge.Format          `json:"src"`
	Dst        llmbridge.Format          `json:"dst"`
	Body       []byte                    `json:"body"`
	Headers    map[string][]string       `json:"headers"`
	PendingURL string                    `json:"pendingURL"`
	Profile    llmbridge.OutboundProfile `json:"profile"`
}

type bridgeNonStreamEnvelope struct {
	Src      llmbridge.Format          `json:"src"`
	Upstream llmbridge.Format          `json:"upstream"`
	Body     []byte                    `json:"body"`
	Headers  map[string][]string       `json:"headers"`
	Profile  llmbridge.OutboundProfile `json:"profile"`
}

type bridgeStreamOpenEnvelope struct {
	Src         llmbridge.Format          `json:"src"`
	Upstream    llmbridge.Format          `json:"upstream"`
	ContentType string                    `json:"contentType"`
	Profile     llmbridge.OutboundProfile `json:"profile"`
}

type aggregateStreamEnvelope struct {
	Format      llmbridge.Format          `json:"format"`
	ContentType string                    `json:"contentType"`
	Body        []byte                    `json:"body"`
	Profile     llmbridge.OutboundProfile `json:"profile"`
}

type operationResponse struct {
	OK          bool   `json:"ok"`
	Error       string `json:"error,omitempty"`
	Body        []byte `json:"body,omitempty"`
	ContentType string `json:"contentType,omitempty"`
	StreamID    uint32 `json:"streamID,omitempty"`
}

func decodeInput(ptr, n uint32, dst any) error {
	if n == 0 {
		return fmt.Errorf("llmbridge: empty wasm input")
	}
	raw := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), n)
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("llmbridge: decode wasm input: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("llmbridge: decode wasm input: multiple JSON values")
		}
		return fmt.Errorf("llmbridge: decode wasm input: %w", err)
	}
	return nil
}

func encodeOutput(v operationResponse) uint64 {
	raw, err := json.Marshal(v)
	if err != nil {
		raw, _ = json.Marshal(operationResponse{OK: false, Error: fmt.Sprintf("llmbridge: encode wasm output: %v", err)})
	}
	if uint64(len(raw)) > math.MaxUint32 {
		raw, _ = json.Marshal(operationResponse{OK: false, Error: "llmbridge: wasm output exceeds uint32 length"})
	}
	ptr := llmbridgeAlloc(uint32(len(raw)))
	copy(unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), len(raw)), raw)
	return pack(ptr, uint32(len(raw)))
}

func pack(ptr, n uint32) uint64 {
	return uint64(ptr)<<32 | uint64(n)
}

func storeStream(stream llmbridgeimpl.StreamBridge) uint32 {
	streamMu.Lock()
	defer streamMu.Unlock()
	id := nextStreamID
	nextStreamID++
	if nextStreamID == 0 {
		nextStreamID = 1
	}
	streams[id] = stream
	return id
}

func getStream(id uint32) llmbridgeimpl.StreamBridge {
	streamMu.Lock()
	defer streamMu.Unlock()
	return streams[id]
}

func deleteStream(id uint32) llmbridgeimpl.StreamBridge {
	streamMu.Lock()
	defer streamMu.Unlock()
	stream := streams[id]
	delete(streams, id)
	return stream
}

type hostReader struct{}

func (hostReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	statusAndN := hostStreamRead(uint32(uintptr(unsafe.Pointer(&p[0]))), uint32(len(p)))
	status := uint32(statusAndN >> 32)
	n := int(uint32(statusAndN))
	switch status {
	case streamStatusOK:
		return n, nil
	case streamStatusEOF:
		return n, io.EOF
	default:
		if n > 0 {
			return n, nil
		}
		return 0, fmt.Errorf("llmbridge: host stream read failed")
	}
}

func (hostReader) Close() error {
	return nil
}

type hostWriter struct{}

func (hostWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	status := hostStreamWrite(uint32(uintptr(unsafe.Pointer(&p[0]))), uint32(len(p)))
	if status != streamStatusOK {
		return 0, fmt.Errorf("llmbridge: host stream write failed")
	}
	return len(p), nil
}
