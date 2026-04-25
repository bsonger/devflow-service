package domain

import (
	"reflect"
	"testing"
)

func TestIntentContract(t *testing.T) {
	typ := reflect.TypeOf(Intent{})
	for _, field := range []string{"ResourceID", "TraceID", "Message", "LastError", "ClaimedBy", "AttemptCount"} {
		if _, ok := typ.FieldByName(field); !ok {
			t.Fatalf("Intent missing field %s", field)
		}
	}
}
