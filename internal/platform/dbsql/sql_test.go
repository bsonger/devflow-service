package dbsql

import (
	"database/sql"
	"testing"
	"time"
)

func TestEmptyToNull(t *testing.T) {
	if got := EmptyToNull("   "); got != nil {
		t.Fatalf("EmptyToNull(blank) = %#v, want nil", got)
	}
	if got := EmptyToNull("prod"); got != "prod" {
		t.Fatalf("EmptyToNull(non-empty) = %#v, want %q", got, "prod")
	}
}

func TestNullableTimePtr(t *testing.T) {
	if got := NullableTimePtr(nil); got != nil {
		t.Fatalf("NullableTimePtr(nil) = %#v, want nil", got)
	}
	now := time.Now().UTC().Truncate(time.Second)
	got, ok := NullableTimePtr(&now).(time.Time)
	if !ok {
		t.Fatalf("NullableTimePtr(non-nil) type = %T, want time.Time", NullableTimePtr(&now))
	}
	if !got.Equal(now) {
		t.Fatalf("NullableTimePtr(non-nil) = %v, want %v", got, now)
	}
}

func TestTimePtrFromNull(t *testing.T) {
	if got := TimePtrFromNull(sql.NullTime{}); got != nil {
		t.Fatalf("TimePtrFromNull(invalid) = %#v, want nil", got)
	}
	now := time.Now().UTC().Truncate(time.Second)
	got := TimePtrFromNull(sql.NullTime{Valid: true, Time: now})
	if got == nil || !got.Equal(now) {
		t.Fatalf("TimePtrFromNull(valid) = %#v, want %v", got, now)
	}
}

func TestParseNullUUID(t *testing.T) {
	if got, err := ParseNullUUID(sql.NullString{}); err != nil || got != nil {
		t.Fatalf("ParseNullUUID(invalid) = (%#v, %v), want (nil, nil)", got, err)
	}
	id := "5c1e6fae-7d17-4dc3-a1c4-9cc3eb32f7c2"
	got, err := ParseNullUUID(sql.NullString{Valid: true, String: id})
	if err != nil {
		t.Fatalf("ParseNullUUID(valid) error = %v", err)
	}
	if got == nil || got.String() != id {
		t.Fatalf("ParseNullUUID(valid) = %#v, want %s", got, id)
	}
	if _, err := ParseNullUUID(sql.NullString{Valid: true, String: "bad-uuid"}); err == nil {
		t.Fatal("expected ParseNullUUID to reject malformed UUID")
	}
}
