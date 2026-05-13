package contract

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

type KvEntryView struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	TTL   int64  `json:"ttl"` // -1 = no expiry, >= 0 = seconds remaining
}

type ListKvEntriesRequest struct {
	Pattern string `query:"pattern" default:"*"`
	Cursor  uint64 `query:"cursor"`
}

type ListKvEntriesResponse struct {
	Body PaginatedBody[KvEntryView]
}

type GetKvEntryRequest struct {
	Key string `path:"key"`
}

type GetKvEntryResponse struct {
	Body KvEntryView
}

type KvMutateBody struct {
	Value      string `json:"value"`
	TTLSeconds *int64 `json:"ttlSeconds,omitempty"` // nil = no expiry
}

type UpsertKvEntryRequest struct {
	Key  string     `path:"key"`
	Body KvMutateBody
}

type UpsertKvEntryResponse struct {
	Body KvEntryView
}

type DeleteKvEntryRequest struct {
	Body struct {
		Key string `json:"key"`
	}
}

var OperationListKvEntries = huma.Operation{
	OperationID: "listKvEntries",
	Method:      http.MethodGet,
	Path:        "/kv",
	Summary:     "List KV entries",
}

var OperationGetKvEntry = huma.Operation{
	OperationID: "getKvEntry",
	Method:      http.MethodGet,
	Path:        "/kv/{key}",
	Summary:     "Get a KV entry",
}

var OperationUpsertKvEntry = huma.Operation{
	OperationID: "upsertKvEntry",
	Method:      http.MethodPut,
	Path:        "/kv/{key}",
	Summary:     "Create or update a KV entry",
}

var OperationDeleteKvEntry = huma.Operation{
	OperationID: "deleteKvEntry",
	Method:      http.MethodPost,
	Path:        "/kv/delete",
	Summary:     "Delete a KV entry",
}
