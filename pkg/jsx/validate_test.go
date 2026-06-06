package jsx

import (
	"strings"
	"testing"
)

func TestValidateSyntax_Valid(t *testing.T) {
	if err := ValidateSyntax(`var x = 1; function f(a, b) { return a + b; }`); err != nil {
		t.Fatalf("want nil, got %v", err)
	}
}

func TestValidateSyntax_BadSyntax(t *testing.T) {
	if err := ValidateSyntax(`var x = ;`); err == nil {
		t.Fatalf("want error, got nil")
	}
}

func TestValidateSyntax_BadSyntaxIncludesValidationFilename(t *testing.T) {
	err := ValidateSyntax("var ok = 1;\nvar x = ;")
	if err == nil {
		t.Fatalf("want error, got nil")
	}
	got := err.Error()
	for _, want := range []string{"script:<validation>", ":2:"} {
		if !strings.Contains(got, want) {
			t.Fatalf("error missing %q: %v", want, got)
		}
	}
}

func TestValidateSyntax_RuntimeReferenceAllowed(t *testing.T) {
	// Runtime errors are not caught by validation per spec.
	if err := ValidateSyntax(`undefined_func();`); err != nil {
		t.Fatalf("want nil (runtime not validated), got %v", err)
	}
}
