package colors

import (
	"testing"
)

func TestClrFunc(t *testing.T) {
	f := ClrFunc(false)
	if f("test") != "test" {
		t.Errorf("ClrFunc(false)(%q) = %q, want %q", "test", f("test"), "test")
	}

	f = ClrFunc(true)
	if f("test") != "" {
		t.Errorf("ClrFunc(true)(%q) = %q, want %q", "test", f("test"), "")
	}
}

func TestVisibleLen(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"hello", 5},
		{"", 0},
		{"\033[31mhello\033[0m", 5},
	}
	for _, tt := range tests {
		got := VisibleLen(tt.input)
		if got != tt.want {
			t.Errorf("VisibleLen(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestPadRight(t *testing.T) {
	got := PadRight("hi", 5)
	if got != "hi   " {
		t.Errorf("PadRight(%q, 5) = %q, want %q", "hi", got, "hi   ")
	}

	got = PadRight("hello", 3)
	if got != "hello" {
		t.Errorf("PadRight(%q, 3) = %q, want %q", "hello", got, "hello")
	}
}

func TestColorizer(t *testing.T) {
	c := New(true)
	if c.Apply("red") != "" {
		t.Error("Expected empty string with NoColor=true")
	}

	c = New(false)
	if c.Apply("red") != "red" {
		t.Error("Expected 'red' with NoColor=false")
	}
}

func TestValidThemes(t *testing.T) {
	themes := ValidThemes()
	if len(themes) < 3 {
		t.Errorf("Expected at least 3 themes, got %d", len(themes))
	}
}

func TestValidateTheme(t *testing.T) {
	if !ValidateTheme("default") {
		t.Error("ValidateTheme('default') should be true")
	}
	if !ValidateTheme("catppuccin-mocha") {
		t.Error("ValidateTheme('catppuccin-mocha') should be true")
	}
	if !ValidateTheme("catppuccin-latte") {
		t.Error("ValidateTheme('catppuccin-latte') should be true")
	}
	if ValidateTheme("invalid-theme") {
		t.Error("ValidateTheme('invalid-theme') should be false")
	}
}

func TestGetPalette(t *testing.T) {
	p := GetPalette("default")
	if p == nil {
		t.Fatal("GetPalette('default') returned nil")
	}
	if p.Success == "" {
		t.Error("Palette.Success should not be empty")
	}

	p = GetPalette("catppuccin-mocha")
	if p == nil {
		t.Fatal("GetPalette('catppuccin-mocha') returned nil")
	}

	p = GetPalette("catppuccin-latte")
	if p == nil {
		t.Fatal("GetPalette('catppuccin-latte') returned nil")
	}

	p = GetPalette("nonexistent")
	if p == nil {
		t.Fatal("GetPalette('nonexistent') returned nil")
	}
}
