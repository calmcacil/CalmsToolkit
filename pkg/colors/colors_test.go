package colors

import (
	"os"
	"testing"
)

func TestNew(t *testing.T) {
	// Test with colors enabled
	colors := New()
	if colors.Reset == "" {
		t.Error("Expected Reset to be set when colors are enabled")
	}
	if colors.Red == "" {
		t.Error("Expected Red to be set when colors are enabled")
	}

	// Test with NO_COLOR set
	os.Setenv("NO_COLOR", "1")
	defer os.Unsetenv("NO_COLOR")

	noColors := New()
	if noColors.Reset != "" {
		t.Error("Expected Reset to be empty when NO_COLOR is set")
	}
	if noColors.Red != "" {
		t.Error("Expected Red to be empty when NO_COLOR is set")
	}
}

func TestNewWithOverride(t *testing.T) {
	// Test with noColor = true
	noColors := NewWithOverride(true)
	if noColors.Reset != "" {
		t.Error("Expected Reset to be empty when noColor is true")
	}

	// Test with noColor = false
	colors := NewWithOverride(false)
	if colors.Reset == "" {
		t.Error("Expected Reset to be set when noColor is false")
	}
}

func TestColorize(t *testing.T) {
	colors := New()

	// Test normal colorization
	result := colors.Colorize(colors.Red, "test")
	expected := colors.Red + "test" + colors.Reset
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}

	// Test with empty color
	result = colors.Colorize("", "test")
	if result != "test" {
		t.Errorf("Expected %q, got %q", "test", result)
	}

	// Test with empty text
	result = colors.Colorize(colors.Red, "")
	if result != "" {
		t.Errorf("Expected empty string, got %q", result)
	}

	// Test with disabled colors
	noColors := NewWithOverride(true)
	result = noColors.Colorize(noColors.Red, "test")
	if result != "test" {
		t.Errorf("Expected %q, got %q", "test", result)
	}
}

func TestColorMethods(t *testing.T) {
	colors := New()

	tests := []struct {
		name     string
		method   func(string) string
		text     string
		expected string
	}{
		{"RedText", colors.RedText, "test", colors.Red + "test" + colors.Reset},
		{"GreenText", colors.GreenText, "test", colors.Green + "test" + colors.Reset},
		{"YellowText", colors.YellowText, "test", colors.Yellow + "test" + colors.Reset},
		{"BlueText", colors.BlueText, "test", colors.Blue + "test" + colors.Reset},
		{"MagentaText", colors.MagentaText, "test", colors.Magenta + "test" + colors.Reset},
		{"CyanText", colors.CyanText, "test", colors.Cyan + "test" + colors.Reset},
		{"GrayText", colors.GrayText, "test", colors.Gray + "test" + colors.Reset},
		{"BoldText", colors.BoldText, "test", colors.Bold + "test" + colors.Reset},
		{"OrangeText", colors.OrangeText, "test", colors.Orange + "test" + colors.Reset},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.method(tt.text)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}

	// Test with disabled colors
	disabledColors := NewWithOverride(true)
	disabledTests := []struct {
		name     string
		method   func(string) string
		text     string
		expected string
	}{
		{"RedText", disabledColors.RedText, "test", "test"},
		{"GreenText", disabledColors.GreenText, "test", "test"},
		{"YellowText", disabledColors.YellowText, "test", "test"},
		{"BlueText", disabledColors.BlueText, "test", "test"},
		{"MagentaText", disabledColors.MagentaText, "test", "test"},
		{"CyanText", disabledColors.CyanText, "test", "test"},
		{"GrayText", disabledColors.GrayText, "test", "test"},
		{"BoldText", disabledColors.BoldText, "test", "test"},
		{"OrangeText", disabledColors.OrangeText, "test", "test"},
	}

	for _, tt := range disabledTests {
		t.Run(tt.name+"Disabled", func(t *testing.T) {
			result := tt.method(tt.text)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}

	// Also verify the disabled colors instance has empty values
	if disabledColors.Reset != "" {
		t.Errorf("Expected disabled colors to have empty Reset, got %q", disabledColors.Reset)
	}
}
