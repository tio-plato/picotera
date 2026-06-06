package jsx

import (
	"fmt"

	"modernc.org/quickjs"
)

// ValidateSyntax checks JS source for parse-time errors. Runtime errors
// (undefined references, throws, etc.) are NOT caught — only syntax issues.
//
// Implementation: Compile() returns the bytecode for the source without
// executing it; a malformed source surfaces as a SyntaxError immediately.
func ValidateSyntax(source string) error {
	return validateSyntaxFile(source, "script:<validation>")
}

func validateSyntaxFile(source, filename string) error {
	vm, err := quickjs.NewVM()
	if err != nil {
		return fmt.Errorf("jsx: quickjs.NewVM: %w", err)
	}
	defer vm.Close()
	if _, err := vm.CompileFile(source, filename, quickjs.EvalGlobal); err != nil {
		return fmt.Errorf("jsx: invalid syntax: %w", err)
	}
	return nil
}
