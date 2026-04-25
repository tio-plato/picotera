package db

// Request type distinguishes meta (client) requests from upstream (provider) requests.
const (
	RequestTypeMeta     = 0
	RequestTypeUpstream = 1
)

// Request status tracks the lifecycle of a request record.
const (
	RequestStatusPending        = 0
	RequestStatusHeaderReceived = 1
	RequestStatusCompleted      = 2
	RequestStatusFailed         = 3
)
