package jsx

import (
	"context"

	"picotera/pkg/db"
)

type ScriptStore interface {
	ListEnabledScripts(ctx context.Context) ([]db.Script, error)
}
