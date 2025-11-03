package calendar

import (
	"testing"
	"time"

	"github.com/calmcacil/CalmsToolkit/internal/config"
)

func TestNewDateRange(t *testing.T) {
	tests := []struct {
		name      string
		mode      ViewMode
		reference time.Time
		daysPast  int
		want      DateRange
	}{
		{
			name:      "Day view - today",
			mode:      DayView,
			reference: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
			daysPast:  0,
			want: DateRange{
				Start: time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2025, 1, 16, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			name:      "Day view - with past days",
			mode:      DayView,
			reference: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
			daysPast:  2,
			want: DateRange{
				Start: time.Date(2025, 1, 13, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2025, 1, 14, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			name:      "Week view",
			mode:      WeekView,
			reference: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
			daysPast:  0,
			want: DateRange{
				Start: time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2025, 1, 22, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			name:      "Month view",
			mode:      MonthView,
			reference: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
			daysPast:  0,
			want: DateRange{
				Start: time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2025, 2, 15, 0, 0, 0, 0, time.UTC),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewDateRange(tt.mode, tt.reference, tt.daysPast)
			if !got.Start.Equal(tt.want.Start) {
				t.Errorf("NewDateRange() Start = %v, want %v", got.Start, tt.want.Start)
			}
			if !got.End.Equal(tt.want.End) {
				t.Errorf("NewDateRange() End = %v, want %v", got.End, tt.want.End)
			}
		})
	}
}

func TestDateRange_Navigate(t *testing.T) {
	baseDate := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	baseRange := DateRange{
		Start: baseDate,
		End:   baseDate.AddDate(0, 0, 1),
	}

	tests := []struct {
		name      string
		direction int
		mode      ViewMode
		want      DateRange
	}{
		{
			name:      "Day view - forward 1 day",
			direction: 1,
			mode:      DayView,
			want: DateRange{
				Start: time.Date(2025, 1, 16, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2025, 1, 17, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			name:      "Day view - backward 1 day",
			direction: -1,
			mode:      DayView,
			want: DateRange{
				Start: time.Date(2025, 1, 14, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			name:      "Week view - forward 1 week",
			direction: 1,
			mode:      WeekView,
			want: DateRange{
				Start: time.Date(2025, 1, 22, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2025, 1, 23, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			name:      "Month view - forward 1 month",
			direction: 1,
			mode:      MonthView,
			want: DateRange{
				Start: time.Date(2025, 2, 15, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2025, 2, 16, 0, 0, 0, 0, time.UTC),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := baseRange.Navigate(tt.direction, tt.mode)
			if !got.Start.Equal(tt.want.Start) {
				t.Errorf("DateRange.Navigate() Start = %v, want %v", got.Start, tt.want.Start)
			}
			if !got.End.Equal(tt.want.End) {
				t.Errorf("DateRange.Navigate() End = %v, want %v", got.End, tt.want.End)
			}
		})
	}
}

func TestViewMode_String(t *testing.T) {
	tests := []struct {
		name string
		mode ViewMode
		want string
	}{
		{"Day view", DayView, "Day"},
		{"Week view", WeekView, "Week"},
		{"Month view", MonthView, "Month"},
		{"Unknown view", ViewMode(999), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.mode.String(); got != tt.want {
				t.Errorf("ViewMode.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewModel(t *testing.T) {
	cfg := &config.Config{
		SonarrURLs:   []string{"http://localhost:8989"},
		SonarrTokens: []string{"test-token"},
		RadarrURLs:   []string{"http://localhost:7878"},
		RadarrTokens: []string{"test-token"},
		Timeout:      10 * time.Second,
		NoColor:      false,
		Debug:        false,
	}

	model := NewModel(cfg)

	if model.config != cfg {
		t.Error("NewModel() config not set correctly")
	}

	if model.viewMode != DayView {
		t.Errorf("NewModel() viewMode = %v, want %v", model.viewMode, DayView)
	}

	if !model.loading {
		t.Error("NewModel() loading should be true initially")
	}

	if model.apiClient == nil {
		t.Error("NewModel() apiClient should not be nil")
	}
}
