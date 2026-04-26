package jsx

import (
	"fmt"

	"github.com/fastschema/qjs"
)

// ValidateSyntax checks JS source for parse-time errors. Runtime errors
// (undefined references, throws, etc.) are NOT caught — only syntax issues.
//
// Implementation: Compile() returns the bytecode for the source without
// executing it; a malformed source surfaces as a SyntaxError immediately.
func ValidateSyntax(source string) error {
	rt, err := qjs.New()
	if err != nil {
		return fmt.Errorf("jsx: qjs.New: %w", err)
	}
	defer rt.Close()
	if _, err := rt.Context().Compile("submitted.js", qjs.Code(source)); err != nil {
		return fmt.Errorf("jsx: invalid syntax: %w", err)
	}
	return nil
}
