package service

import (
	"context"
	"testing"

	sharederrs "github.com/bsonger/devflow-service/internal/shared/errs"
)

func TestResolveEnvironmentResolverReturnsNilOnEmptyBaseURL(t *testing.T) {
	if got := ResolveEnvironmentResolver("   "); got != nil {
		t.Fatalf("resolver = %#v, want nil", got)
	}
}

func TestHTTPEnvironmentResolverResolveNameRejectsEmptyID(t *testing.T) {
	resolver := ResolveEnvironmentResolver("http://example.com")
	if resolver == nil {
		t.Fatal("expected resolver")
	}

	_, err := resolver.ResolveName(context.Background(), "   ")
	if !sharederrs.HasCode(err, sharederrs.CodeInvalidArgument) {
		t.Fatalf("code = %q, want %q", sharederrs.Code(err), sharederrs.CodeInvalidArgument)
	}
}
