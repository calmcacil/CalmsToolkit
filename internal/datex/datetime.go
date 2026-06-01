package datex

import (
	"fmt"
	"strings"
	"time"
)

var formats = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05Z",
	"2006-01-02T15:04:05.000Z",
	"2006-01-02T15:04:05",
	"2006-01-02",
}

type DateTime struct {
	time.Time
}

func (d *DateTime) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	if s == "" || s == "null" {
		return nil
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			d.Time = t
			return nil
		}
	}
	return fmt.Errorf("unrecognized date: %s", s)
}

func (d DateTime) MarshalJSON() ([]byte, error) {
	if d.Time.IsZero() {
		return []byte("null"), nil
	}
	return []byte(`"` + d.Time.UTC().Format(time.RFC3339) + `"`), nil
}
