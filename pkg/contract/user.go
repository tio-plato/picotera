package contract

import (
	"net/http"

	"picotera/pkg/db"

	"github.com/danielgtaylor/huma/v2"
)

// MeView is the current authenticated user's public information.
type MeView struct {
	ID          int64  `json:"id"`
	DisplayName string `json:"displayName"`
	IsAdmin     bool   `json:"isAdmin"`
}

// ToMeView converts a db.AppUser to the API view.
func ToMeView(u *db.AppUser) MeView {
	return MeView{
		ID:          u.ID,
		DisplayName: u.DisplayName,
		IsAdmin:     u.IsAdmin,
	}
}

// GetMeResponse is the response for the current user.
type GetMeResponse struct {
	Body MeView
}

var OperationGetMe = huma.Operation{
	OperationID: "getMe",
	Method:      http.MethodGet,
	Path:        "/me",
	Summary:     "Get current user",
	Tags:        []string{"User"},
}
