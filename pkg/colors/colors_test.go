package colors

import (
	"os"
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		noColor bool
		env     string
		want    string
	}{
		{
			name:    "colors enabled",
			noColor: false,
			want:    "\033[0;31m",
		},
		{
			name:    "colors disabled by flag",
			noColor: true,
			want:    "",
		},
		{
			name:    "colors disabled by env",
			noColor: false,
			env:     "1",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.env != "" {
				os.Setenv("NO_COLOR", tt.env)
				defer os.Unsetenv("NO_COLOR")
			}
			c := New(tt.noColor)
			if c.Red != tt.want {
				t.Errorf("New().Red = %q, want %q", c.Red, tt.want)
			}
		})
	}
}

func TestColorize(t *testing.T) {
	c := Colors{Red: "\033[0;31m", Reset: "\033[0m"}

	tests := []struct {
		name  string
		text  string
		color string
		want  string
	}{
		{
			name:  "simple colorize",
			text:  "test",
			color: c.Red,
			want:  "\033[0;31mtest\033[0m",
		},
		{
			name:  "empty colors",
			text:  "test",
			color: "",
			want:  "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.Colorize(tt.text, tt.color)
			if got != tt.want {
				t.Errorf("Colorize() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestColorHelpers(t *testing.T) {
	c := New(false)

	tests := []struct {
		name     string
		function func(string) string
		text     string
		want     string
	}{
		{
			name:     "RedText",
			function: c.RedText,
			text:     "error",
			want:     "\033[0;31merror\033[0m",
		},
		{
			name:     "GreenText",
			function: c.GreenText,
			text:     "success",
			want:     "\033[0;32msuccess\033[0m",
		},
		{
			name:     "BoldText",
			function: c.BoldText,
			text:     "bold",
			want:     "\033[1mbold\033[0m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.function(tt.text)
			if got != tt.want {
				t.Errorf("%s() = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}
