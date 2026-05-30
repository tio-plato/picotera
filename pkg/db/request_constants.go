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

const (
	FinishReasonInternal       = 1
	FinishReasonCancelled      = 2
	FinishReasonEOF            = 3
	FinishReasonHeadersTimeout = 4
	FinishReasonReadTimeout    = 5
	FinishReasonStreamError    = 6
	// FinishReasonDashboardCancelled marks a request row deliberately
	// interrupted from the dashboard, distinct from FinishReasonCancelled
	// (client disconnect / context cancellation).
	FinishReasonDashboardCancelled = 7
)
