package jsx

import "testing"

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

func TestValidateSyntax_RuntimeReferenceAllowed(t *testing.T) {
	// Runtime errors are not caught by validation per spec.
	if err := ValidateSyntax(`undefined_func();`); err != nil {
		t.Fatalf("want nil (runtime not validated), got %v", err)
	}
}
