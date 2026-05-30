package server

import (
	"context"
	"slices"
	"testing"
)

func TestExtractProjectCandidatesEnvWorkingDirectoryEscapedNewlines(t *testing.T) {
	body := []byte(`report/summary/findings/analysis .md files. Return findings directly as your final assistant message — the parent agent reads your text output, not files you create.\n\nHere is useful information about the environment you are running in:\n<env>\nWorking directory: /home/oott123/Work/Projects/picotera\nIs directory a git repo: Yes\nPlatform: linux\nShell: bash\nOS Version: Linux 7.0.2-zen1-1-zen\n</env>\nYou are powered by the model named Haiku 4.5. The exact model ID is claude-haiku-4-5-20251001.\n\nAssistant knowledge cutoff is February 2025.","cache_control":{"typ`)

	candidates := extractProjectCandidates(context.Background(), body)
	if !slices.Contains(candidates, "/home/oott123/Work/Projects/picotera") {
		t.Fatalf("expected env working directory candidate, got %#v", candidates)
	}
}
