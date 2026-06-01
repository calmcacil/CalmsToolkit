package datex

import (
	"encoding/json"
	"testing"
	"time"
)

func TestUnmarshalJSON_RFC3339(t *testing.T) {
	var d DateTime
	data := `"2026-06-01T12:00:00Z"`
	if err := json.Unmarshal([]byte(data), &d); err != nil {
		t.Fatal(err)
	}
	if d.Year() != 2026 || d.Month() != 6 || d.Day() != 1 {
		t.Errorf("got %v, want 2026-06-01", d.Time)
	}
}

func TestUnmarshalJSON_DateOnly(t *testing.T) {
	var d DateTime
	data := `"2026-06-01"`
	if err := json.Unmarshal([]byte(data), &d); err != nil {
		t.Fatal(err)
	}
}

func TestUnmarshalJSON_EmptyString(t *testing.T) {
	var d DateTime
	if err := json.Unmarshal([]byte(`""`), &d); err != nil {
		t.Fatal(err)
	}
	if !d.IsZero() {
		t.Error("empty string should produce zero time")
	}
}

func TestUnmarshalJSON_Null(t *testing.T) {
	var d DateTime
	if err := json.Unmarshal([]byte(`null`), &d); err != nil {
		t.Fatal(err)
	}
	if !d.IsZero() {
		t.Error("null should produce zero time")
	}
}

func TestUnmarshalJSON_Invalid(t *testing.T) {
	var d DateTime
	if err := json.Unmarshal([]byte(`"not-a-date"`), &d); err == nil {
		t.Fatal("expected error for invalid date")
	}
}

func TestMarshalJSON_ZeroIsNull(t *testing.T) {
	var d DateTime
	data, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "null" {
		t.Errorf("zero time should marshal as null, got %s", data)
	}
}

func TestMarshalJSON_RoundTrip(t *testing.T) {
	orig := DateTime{Time: time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)}
	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatal(err)
	}
	var round DateTime
	if err := json.Unmarshal(data, &round); err != nil {
		t.Fatal(err)
	}
	if !round.Time.Equal(orig.Time) {
		t.Errorf("round trip: got %v, want %v", round.Time, orig.Time)
	}
}
