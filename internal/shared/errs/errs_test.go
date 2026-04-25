package errs

import (
	"errors"
	"testing"
)

func TestRequiredReturnsInvalidArgumentCode(t *testing.T) {
	err := Required("application_id")
	if Code(err) != CodeInvalidArgument {
		t.Fatalf("code = %q, want %q", Code(err), CodeInvalidArgument)
	}
	if err.Error() != "application_id is required" {
		t.Fatalf("message = %q", err.Error())
	}
}

func TestJoinInvalidSkipsEmptyMessages(t *testing.T) {
	err := JoinInvalid([]string{"name is required", "", "strategy is invalid"})
	if Code(err) != CodeInvalidArgument {
		t.Fatalf("code = %q, want %q", Code(err), CodeInvalidArgument)
	}
	if err.Error() != "name is required; strategy is invalid" {
		t.Fatalf("message = %q", err.Error())
	}
}

func TestWrapPreservesCodeAndCause(t *testing.T) {
	cause := errors.New("boom")
	err := Wrap(CodeFailedPrecondition, "config repository sync failed", cause)
	if Code(err) != CodeFailedPrecondition {
		t.Fatalf("code = %q, want %q", Code(err), CodeFailedPrecondition)
	}
	if !errors.Is(err, cause) {
		t.Fatal("expected wrapped error to match cause")
	}
}
