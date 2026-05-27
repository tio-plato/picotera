package server

import (
	"errors"
	"net/http"
	"time"

	"picotera/pkg/artifacts"
	"picotera/pkg/db"
	"picotera/pkg/errorx"
	"picotera/pkg/jsx"

	"github.com/jackc/pgx/v5/pgtype"
)

type gatewayHookError struct {
	err error
}

func (e gatewayHookError) Error() string {
	return e.err.Error()
}

func (e gatewayHookError) Unwrap() error {
	return e.err
}

func gatewayHookStatus(err error) int {
	if errors.Is(err, jsx.ErrHookTimeout) {
		return http.StatusServiceUnavailable
	}
	return http.StatusBadGateway
}

func (f *gatewayFlow) collectLogs() []artifacts.LogEntry {
	if f.session == nil {
		return nil
	}
	raw := f.session.Logs()
	if len(raw) == 0 {
		return nil
	}
	out := make([]artifacts.LogEntry, len(raw))
	for i, l := range raw {
		out[i] = artifacts.LogEntry{Level: l.Level, Message: l.Message, Ts: l.Ts}
	}
	return out
}

func (f *gatewayFlow) failMeta(status int32, errMsg string) {
	if f.meta.ID == "" {
		return
	}
	f.h.updateRequestOnComplete(f.ctxs.Persist, db.UpdateRequestOnCompleteParams{
		ID:           f.meta.ID,
		StatusCode:   pgtype.Int4{Int32: status, Valid: true},
		ErrorMessage: pgtype.Text{String: errMsg, Valid: true},
		TimeSpentMs:  pgtype.Int4{Int32: int32(time.Since(f.startedAt).Milliseconds()), Valid: true},
		Status:       db.RequestStatusFailed,
		CreatedAt:    pgtype.Timestamp{Time: f.meta.CreatedAt, Valid: true},
	})
}

func (f *gatewayFlow) failGatewayError(err error) {
	status, body := handleGatewayErr(f.w, err)
	f.h.uploadMetaResponseArtifact(f.ctxs.Persist, f.meta.ID, f.meta.CreatedAt, status, f.w.Header().Clone(), body, f.collectLogs(), nil)
}

func (f *gatewayFlow) failGatewayErrorWithFallback(err error, fallbackStatus int32, fallbackMsg string) {
	var gwErr *gatewayError
	if errors.As(err, &gwErr) {
		f.failMeta(int32(gwErr.status), gwErr.message)
	} else {
		f.failMeta(fallbackStatus, fallbackMsg)
	}
	f.failGatewayError(err)
}

func (f *gatewayFlow) failHook(err error) {
	status := gatewayHookStatus(err)
	errMsg := err.Error()
	f.failMeta(int32(status), errMsg)
	body := writeGatewayError(f.w, status, errMsg, errorx.UpstreamError.Error())
	f.h.uploadMetaResponseArtifact(f.ctxs.Persist, f.meta.ID, f.meta.CreatedAt, status, f.w.Header().Clone(), body, f.collectLogs(), nil)
}

func (f *gatewayFlow) failInternal(status int, message string, code string) {
	f.failMeta(int32(status), message)
	body := writeGatewayError(f.w, status, message, code)
	f.h.uploadMetaResponseArtifact(f.ctxs.Persist, f.meta.ID, f.meta.CreatedAt, status, f.w.Header().Clone(), body, f.collectLogs(), nil)
}

func (f *gatewayFlow) failAllProviders(lastErr error) {
	errMsg := "all providers failed"
	if lastErr != nil {
		errMsg = lastErr.Error()
	}
	f.failMeta(http.StatusBadGateway, errMsg)
	body := writeGatewayError(f.w, http.StatusBadGateway, errMsg, errorx.UpstreamError.Error())
	f.h.uploadMetaResponseArtifact(f.ctxs.Persist, f.meta.ID, f.meta.CreatedAt, http.StatusBadGateway, f.w.Header().Clone(), body, f.collectLogs(), nil)
}

func (f *gatewayFlow) failSuccessPath(input successInput, errMsg string) {
	input.Cancel()
	f.h.completeFailedAttempt(f.ctxs.Persist, input.UpstreamID, input.UpstreamCreatedAt, input.AttemptStart, int32(input.Response.StatusCode), errMsg)
	body := writeGatewayError(f.w, http.StatusBadGateway, "bridge failed: "+errMsg, errorx.UpstreamError.Error())
	f.failMeta(http.StatusBadGateway, errMsg)
	f.h.uploadMetaResponseArtifact(f.ctxs.Persist, f.meta.ID, f.meta.CreatedAt, http.StatusBadGateway, f.w.Header().Clone(), body, f.collectLogs(), nil)
	_ = input.Response.Body.Close()
}
