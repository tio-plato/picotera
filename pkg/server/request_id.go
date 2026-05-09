package server

import (
	"regexp"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/rs/xid"
)

var strictXIDPattern = regexp.MustCompile(`^[0-9a-v]{20}$`)

func newRequestID() (string, time.Time) {
	id := xid.New()
	return id.String(), id.Time().UTC()
}

func requestIDCreatedAt(id string) (time.Time, error) {
	if !strictXIDPattern.MatchString(id) {
		return time.Time{}, huma.Error400BadRequest("invalid request id")
	}
	parsed, err := xid.FromString(id)
	if err != nil {
		return time.Time{}, huma.Error400BadRequest("invalid request id", err)
	}
	return parsed.Time().UTC(), nil
}

func validateTraceID(id string) error {
	if !strictXIDPattern.MatchString(id) {
		return huma.Error400BadRequest("invalid trace id")
	}
	if _, err := xid.FromString(id); err != nil {
		return huma.Error400BadRequest("invalid trace id", err)
	}
	return nil
}
