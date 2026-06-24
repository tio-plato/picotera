// Package auth resolves the user identity for internal management API requests.
//
// The first phase ships two identity providers — "single-user-mode" and
// "http-header" — and a chi middleware that authenticates only the
// /api/picotera prefix. Identity resolution is decoupled from the gateway data
// plane: the gateway catch-all and /api/unified routes authenticate via API
// key and never reach this package.
package auth

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"picotera/pkg/configx"
	"picotera/pkg/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ImpersonationHeader carries the target user id when an admin impersonates
// another user. It is read in the chi middleware (below Huma) and only honored
// when the real, server-resolved identity is an admin.
const ImpersonationHeader = "X-PicoTera-Impersonation-User-Id"

// Identity provider names persisted in user_identity.provider.
const (
	ProviderSingleUserMode = "single-user-mode"
	ProviderHTTPHeader     = "http-header"

	singleUserIdentity = "root"
)

type contextKey struct{}

var userContextKey = contextKey{}

// WithUser stores the resolved user on the request context.
func WithUser(ctx context.Context, u *db.AppUser) context.Context {
	return context.WithValue(ctx, userContextKey, u)
}

// UserFromContext returns the resolved user, or nil if the request was not
// authenticated.
func UserFromContext(ctx context.Context) *db.AppUser {
	u, _ := ctx.Value(userContextKey).(*db.AppUser)
	return u
}

// ErrUnauthorized is returned by Resolve when no user could be identified.
var ErrUnauthorized = errors.New("unauthorized")

// Impersonation errors returned by ResolveWithImpersonation.
var (
	// ErrImpersonationForbidden is returned when a non-admin real user sends
	// the impersonation header.
	ErrImpersonationForbidden = errors.New("impersonation forbidden")
	// ErrImpersonationBadID is returned when the impersonation header value is
	// not a valid integer.
	ErrImpersonationBadID = errors.New("invalid impersonation user id")
	// ErrImpersonationTargetNotFound is returned when the impersonation target
	// user does not exist.
	ErrImpersonationTargetNotFound = errors.New("impersonation target not found")
)

// Resolver maps an incoming request to an application user.
type Resolver struct {
	db      *pgxpool.Pool
	queries *db.Queries
	config  configx.AuthConfig
}

// NewResolver builds a Resolver. It needs both the pool (for transactional
// auto-create) and the pool-bound queries (for plain reads).
func NewResolver(pool *pgxpool.Pool, queries *db.Queries, config configx.AuthConfig) *Resolver {
	return &Resolver{db: pool, queries: queries, config: config}
}

// Resolve identifies the user for a request following the fixed precedence:
// single-user-mode, then http-header, otherwise unauthorized.
func (r *Resolver) Resolve(ctx context.Context, req *http.Request) (*db.AppUser, error) {
	if r.config.SingleUserMode {
		// Single-user mode ignores all headers and is bootstrapped
		// unconditionally as an admin, independent of AutoCreateUser.
		return r.resolveOrCreate(ctx, ProviderSingleUserMode, singleUserIdentity, singleUserIdentity, true, true)
	}

	if r.config.HeaderEnabled {
		value := req.Header.Get(r.config.HeaderName)
		if value == "" {
			return nil, ErrUnauthorized
		}
		return r.resolveOrCreate(ctx, ProviderHTTPHeader, value, value, false, r.config.AutoCreateUser)
	}

	// No identity provider configured: never implicitly authenticate.
	return nil, ErrUnauthorized
}

// ResolveWithImpersonation resolves the real user via Resolve, then applies
// impersonation when the request carries ImpersonationHeader. Impersonation is
// honored only for a real admin; the target user is returned as-is (a disabled
// target is allowed so admins can inspect disabled users' data).
func (r *Resolver) ResolveWithImpersonation(ctx context.Context, req *http.Request) (*db.AppUser, error) {
	user, err := r.Resolve(ctx, req)
	if err != nil {
		return nil, err
	}
	raw := req.Header.Get(ImpersonationHeader)
	if raw == "" {
		return user, nil
	}
	if !user.IsAdmin {
		return nil, ErrImpersonationForbidden
	}
	id, perr := strconv.ParseInt(raw, 10, 64)
	if perr != nil {
		return nil, ErrImpersonationBadID
	}
	target, gerr := r.queries.GetUserByID(ctx, id)
	if gerr != nil {
		if errors.Is(gerr, pgx.ErrNoRows) {
			return nil, ErrImpersonationTargetNotFound
		}
		return nil, gerr
	}
	return &target, nil
}

// resolveOrCreate looks up the user by identity, creating it when autoCreate is
// set. admin is the is_admin flag applied to a newly created user.
func (r *Resolver) resolveOrCreate(ctx context.Context, provider, identity, displayName string, admin, autoCreate bool) (*db.AppUser, error) {
	u, err := r.queries.GetUserByIdentity(ctx, db.GetUserByIdentityParams{
		Provider: provider,
		Identity: identity,
	})
	if err == nil {
		if u.Disabled {
			return nil, ErrUnauthorized
		}
		return &u, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if !autoCreate {
		return nil, ErrUnauthorized
	}
	return r.createUserWithIdentity(ctx, provider, identity, displayName, admin)
}

// createUserWithIdentity inserts an app_user and its identity in a single
// transaction. A concurrent creator winning the (provider, identity) unique
// constraint makes InsertUserIdentity return zero rows; we roll back and reread
// the now-existing user to stay idempotent.
func (r *Resolver) createUserWithIdentity(ctx context.Context, provider, identity, displayName string, admin bool) (*db.AppUser, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	q := r.queries.WithTx(tx)

	user, err := q.InsertUser(ctx, db.InsertUserParams{
		DisplayName: displayName,
		IsAdmin:     admin,
		Annotations: []byte("{}"),
	})
	if err != nil {
		return nil, err
	}

	_, err = q.InsertUserIdentity(ctx, db.InsertUserIdentityParams{
		UserID:   user.ID,
		Provider: provider,
		Identity: identity,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Lost the race: another request already created this identity.
			_ = tx.Rollback(ctx)
			existing, rerr := r.queries.GetUserByIdentity(ctx, db.GetUserByIdentityParams{
				Provider: provider,
				Identity: identity,
			})
			if rerr != nil {
				return nil, rerr
			}
			return &existing, nil
		}
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &user, nil
}
