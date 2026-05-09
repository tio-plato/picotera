// Package server — project_router.go
//
// In-memory cache of every (project_id, path) pair derived from the project
// table's `paths` JSONB array. Used by project_extractor.go to map a candidate
// path string to a project id via longest-prefix wins.
//
// Mirrors endpoint_router.go: lazy load on first Match, explicit Invalidate()
// on every project mutation. Any future writer of the project table MUST call
// Server.projectRouter.Invalidate() at the same site.
package server

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"picotera/pkg/db"
)

type projectEntry struct {
	path      string
	projectID int32
}

type projectRouter struct {
	queries *db.Queries

	mu      sync.RWMutex
	entries []projectEntry // sorted: len(path) desc, then projectID asc
	loaded  bool
}

func newProjectRouter(q *db.Queries) *projectRouter {
	return &projectRouter{queries: q}
}

// Match walks the cached entries (longest-path first) and returns the project
// id of the first entry whose path is a prefix of any candidate. Returns
// (0, false) when no entry matches or candidates is empty.
func (r *projectRouter) Match(ctx context.Context, candidates []string) (int32, bool, error) {
	if len(candidates) == 0 {
		return 0, false, nil
	}

	r.mu.RLock()
	if r.loaded {
		id, ok := r.matchLocked(candidates)
		r.mu.RUnlock()
		return id, ok, nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	if !r.loaded {
		if err := r.load(ctx); err != nil {
			r.mu.Unlock()
			return 0, false, err
		}
	}
	id, ok := r.matchLocked(candidates)
	r.mu.Unlock()
	return id, ok, nil
}

func (r *projectRouter) matchLocked(candidates []string) (int32, bool) {
	for _, e := range r.entries {
		for _, c := range candidates {
			if strings.HasPrefix(c, e.path) {
				return e.projectID, true
			}
		}
	}
	return 0, false
}

// Invalidate drops the cached entries. The next Match call will reload from
// the database.
func (r *projectRouter) Invalidate() {
	r.mu.Lock()
	r.entries = nil
	r.loaded = false
	r.mu.Unlock()
}

func (r *projectRouter) load(ctx context.Context) error {
	rows, err := r.queries.ListProjectPaths(ctx)
	if err != nil {
		return fmt.Errorf("project router: load: %w", err)
	}

	entries := make([]projectEntry, 0, len(rows))
	for _, row := range rows {
		if row.Path == "" {
			continue
		}
		entries = append(entries, projectEntry{
			path:      row.Path,
			projectID: row.ProjectID,
		})
	}

	sortProjectEntries(entries)
	r.entries = entries
	r.loaded = true
	return nil
}

func sortProjectEntries(entries []projectEntry) {
	// len(path) desc, ties broken by projectID asc.
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0; j-- {
			a, b := entries[j-1], entries[j]
			if len(a.path) > len(b.path) {
				break
			}
			if len(a.path) == len(b.path) && a.projectID <= b.projectID {
				break
			}
			entries[j-1], entries[j] = b, a
		}
	}
}
