// Package server — project_extractor.go
//
// Pulls candidate workspace paths out of a gateway request body using a fixed
// set of regexes, JSON-unescapes each capture, and asks projectRouter to match
// them to a project id. Hooked from handle_gateway.go and
// handle_unified_gateway.go between body read and meta-row insert.
package server

import (
	"context"
	"encoding/json"
	"regexp"

	"picotera/pkg/logx"
)

// projectExtractRegexps are the fixed patterns scanned over each request body.
// New patterns must be appended here — there is no runtime configuration.
//
// `(?:\\n|\n)` matches either the JSON-escape sequence `\n` (two bytes, the
// usual case since LLM gateway bodies are JSON) or an actual newline byte
// (defensive — for the rare non-JSON body).
var projectExtractRegexps = []*regexp.Regexp{
	regexp.MustCompile(`Workspace root folder: (.*?)(?:\\n|\n|$|")`),
	regexp.MustCompile(`Primary working directory: (.*?)(?:\\n|\n|$|")`),
	regexp.MustCompile(`Current working directory: (.*?)(?:\\n|\n|$|")`),
	regexp.MustCompile(`<cwd>(.*?)</cwd>`),
	regexp.MustCompile(`<env>(?:\\n|\n)Working directory: (.*?)(?:\\n|\n|$|")`),
}

type projectExtractor struct {
	router *projectRouter
}

func newProjectExtractor(router *projectRouter) *projectExtractor {
	return &projectExtractor{router: router}
}

// Extract scans body, decodes capture groups, dedupes them, and asks the
// router for a project id. Returns (0, false, nil) when no candidates parse
// or no path matches.
func (e *projectExtractor) Extract(ctx context.Context, body []byte) (int32, bool, error) {
	if len(body) == 0 {
		return 0, false, nil
	}
	candidates := extractProjectCandidates(ctx, body)
	if len(candidates) == 0 {
		return 0, false, nil
	}
	logx.WithContext(ctx).WithField("candidates", candidates).Debug("project extractor: candidates")
	return e.router.Match(ctx, candidates)
}

func extractProjectCandidates(ctx context.Context, body []byte) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, re := range projectExtractRegexps {
		matches := re.FindAllSubmatch(body, -1)
		for _, m := range matches {
			if len(m) < 2 {
				continue
			}
			decoded, ok := decodeJSONString(ctx, m[1])
			if !ok || decoded == "" {
				continue
			}
			if _, dup := seen[decoded]; dup {
				continue
			}
			seen[decoded] = struct{}{}
			out = append(out, decoded)
		}
	}
	return out
}

func decodeJSONString(ctx context.Context, raw []byte) (string, bool) {
	wrapped := make([]byte, 0, len(raw)+2)
	wrapped = append(wrapped, '"')
	wrapped = append(wrapped, raw...)
	wrapped = append(wrapped, '"')
	var s string
	if err := json.Unmarshal(wrapped, &s); err != nil {
		logx.WithContext(ctx).WithError(err).WithField("raw", string(raw)).Debug("project extractor: json unmarshal failed")
		return "", false
	}
	return s, true
}
