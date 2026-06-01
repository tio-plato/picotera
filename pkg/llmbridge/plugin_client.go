package llmbridge

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"sync"
	"time"

	plugin "github.com/hashicorp/go-plugin"
)

const defaultPluginStartTimeout = 10 * time.Second

type pluginBridge struct {
	client *plugin.Client
	grpc   LLMBridgeClient
}

func newPluginBridge(ctx context.Context, cfg Config) (Bridge, error) {
	timeout := cfg.PluginStartTimeout
	if timeout == 0 {
		timeout = defaultPluginStartTimeout
	}
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig:  pluginHandshake,
		Plugins:          pluginMap(nil),
		Cmd:              exec.Command(cfg.PluginPath),
		StartTimeout:     timeout,
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
	})
	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("llmbridge: start plugin: %w", err)
	}
	raw, err := rpcClient.Dispense(pluginName)
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("llmbridge: dispense plugin: %w", err)
	}
	grpcClient, ok := raw.(LLMBridgeClient)
	if !ok {
		client.Kill()
		return nil, fmt.Errorf("llmbridge: plugin returned unexpected client %T", raw)
	}
	infoCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	info, err := grpcClient.GetInfo(infoCtx, &GetInfoRequest{})
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("llmbridge: validate plugin: %w", err)
	}
	if err := validatePluginABI(info.GetAbiVersion()); err != nil {
		client.Kill()
		return nil, err
	}
	return &pluginBridge{client: client, grpc: grpcClient}, nil
}

func (b *pluginBridge) Enabled() bool {
	return true
}

func (b *pluginBridge) Close(context.Context) error {
	b.client.Kill()
	return nil
}

func (b *pluginBridge) BridgeRequest(ctx context.Context, src, dst Format, body []byte, headers http.Header, pendingURL string, profile OutboundProfile) ([]byte, string, error) {
	if src == FormatUnknown || dst == FormatUnknown {
		return nil, "", fmt.Errorf("llmbridge: bridge with unknown format (src=%s dst=%s)", src, dst)
	}
	if src == dst {
		return body, contentTypeOrDefault(headers), nil
	}
	profileMsg, err := profileToProto(profile)
	if err != nil {
		return nil, "", err
	}
	resp, err := b.grpc.BridgeRequest(ctx, &BridgeRequestRequest{
		Src:        formatToProto(src),
		Dst:        formatToProto(dst),
		Body:       body,
		Headers:    headerToProto(headers),
		PendingUrl: pendingURL,
		Profile:    profileMsg,
	})
	if err != nil {
		return nil, "", fmt.Errorf("llmbridge: bridge request plugin call: %w", err)
	}
	return resp.GetBody(), resp.GetContentType(), nil
}

func (b *pluginBridge) BridgeNonStream(ctx context.Context, src, upstream Format, upstreamBody []byte, upstreamHeaders http.Header, profile OutboundProfile) ([]byte, string, error) {
	if src == FormatUnknown || upstream == FormatUnknown {
		return nil, "", fmt.Errorf("llmbridge: bridge non-stream with unknown format (src=%s upstream=%s)", src, upstream)
	}
	if src == upstream {
		return upstreamBody, contentTypeOrDefault(upstreamHeaders), nil
	}
	profileMsg, err := profileToProto(profile)
	if err != nil {
		return nil, "", err
	}
	resp, err := b.grpc.BridgeNonStream(ctx, &BridgeNonStreamRequest{
		Src:      formatToProto(src),
		Upstream: formatToProto(upstream),
		Body:     upstreamBody,
		Headers:  headerToProto(upstreamHeaders),
		Profile:  profileMsg,
	})
	if err != nil {
		return nil, "", fmt.Errorf("llmbridge: bridge non-stream plugin call: %w", err)
	}
	return resp.GetBody(), resp.GetContentType(), nil
}

func (b *pluginBridge) AggregateStream(ctx context.Context, format Format, contentType string, body []byte, profile OutboundProfile) ([]byte, error) {
	if format == FormatUnknown {
		return nil, fmt.Errorf("llmbridge: aggregate stream with unknown format")
	}
	profileMsg, err := profileToProto(profile)
	if err != nil {
		return nil, err
	}
	resp, err := b.grpc.AggregateStream(ctx, &AggregateStreamRequest{
		Format:      formatToProto(format),
		ContentType: contentType,
		Body:        body,
		Profile:     profileMsg,
	})
	if err != nil {
		return nil, fmt.Errorf("llmbridge: aggregate stream plugin call: %w", err)
	}
	return resp.GetBody(), nil
}

func (b *pluginBridge) BridgeStream(ctx context.Context, src, upstream Format, upstreamBody io.ReadCloser, upstreamCT string, profile OutboundProfile) (io.ReadCloser, error) {
	if src == FormatUnknown || upstream == FormatUnknown {
		_ = upstreamBody.Close()
		return nil, fmt.Errorf("llmbridge: bridge stream with unknown format (src=%s upstream=%s)", src, upstream)
	}
	if src == upstream {
		return upstreamBody, nil
	}
	profileMsg, err := profileToProto(profile)
	if err != nil {
		_ = upstreamBody.Close()
		return nil, err
	}
	stream, err := b.grpc.BridgeStream(ctx)
	if err != nil {
		_ = upstreamBody.Close()
		return nil, fmt.Errorf("llmbridge: open bridge stream plugin call: %w", err)
	}
	if err := stream.Send(&BridgeStreamChunk{Payload: &BridgeStreamChunk_Start{Start: &BridgeStreamStart{
		Src:         formatToProto(src),
		Upstream:    formatToProto(upstream),
		ContentType: upstreamCT,
		Profile:     profileMsg,
	}}}); err != nil {
		_ = upstreamBody.Close()
		_ = stream.CloseSend()
		return nil, fmt.Errorf("llmbridge: send bridge stream start: %w", err)
	}

	pr, pw := io.Pipe()
	done := make(chan struct{})
	var closeOnce sync.Once
	closeAll := func() {
		closeOnce.Do(func() {
			_ = upstreamBody.Close()
			_ = stream.CloseSend()
			close(done)
		})
	}

	go func() {
		defer closeAll()
		buf := make([]byte, 32*1024)
		for {
			n, readErr := upstreamBody.Read(buf)
			if n > 0 {
				data := make([]byte, n)
				copy(data, buf[:n])
				if err := stream.Send(&BridgeStreamChunk{Payload: &BridgeStreamChunk_Data{Data: data}}); err != nil {
					_ = pw.CloseWithError(fmt.Errorf("llmbridge: send bridge stream data: %w", err))
					return
				}
			}
			if readErr == nil {
				continue
			}
			if readErr == io.EOF {
				if err := stream.Send(&BridgeStreamChunk{Payload: &BridgeStreamChunk_End{End: &BridgeStreamEnd{}}}); err != nil {
					_ = pw.CloseWithError(fmt.Errorf("llmbridge: send bridge stream end: %w", err))
				}
				return
			}
			_ = stream.Send(&BridgeStreamChunk{Payload: &BridgeStreamChunk_Error{Error: &BridgeStreamError{Message: readErr.Error()}}})
			_ = pw.CloseWithError(readErr)
			return
		}
	}()

	go func() {
		defer closeAll()
		for {
			chunk, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					_ = pw.Close()
				} else {
					_ = pw.CloseWithError(fmt.Errorf("llmbridge: receive bridge stream data: %w", err))
				}
				return
			}
			switch payload := chunk.GetPayload().(type) {
			case *BridgeStreamChunk_Data:
				if _, err := pw.Write(payload.Data); err != nil {
					return
				}
			case *BridgeStreamChunk_Error:
				msg := payload.Error.GetMessage()
				if msg == "" {
					msg = "llmbridge: bridge stream plugin returned empty error"
				}
				_ = pw.CloseWithError(fmt.Errorf("%s", msg))
				return
			default:
				_ = pw.CloseWithError(fmt.Errorf("llmbridge: bridge stream plugin returned unexpected frame"))
				return
			}
		}
	}()

	return &pluginStreamReadCloser{ReadCloser: pr, close: closeAll, done: done}, nil
}

type pluginStreamReadCloser struct {
	io.ReadCloser
	close func()
	done  <-chan struct{}
}

func (r *pluginStreamReadCloser) Close() error {
	err := r.ReadCloser.Close()
	r.close()
	<-r.done
	return err
}
