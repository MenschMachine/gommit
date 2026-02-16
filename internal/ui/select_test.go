package ui

import (
	"testing"
)

func TestSelectOption_ValidatesInputs(t *testing.T) {
	// Test with empty options list
	_, err := SelectOption("Choose", []string{})
	if err == nil {
		t.Error("expected error with empty options, got nil")
	}

	// Note: Full interactive testing requires mocked stdin/tty
	// which is beyond simple unit tests. Huh provides WithAccessible()
	// mode for automated environments but still needs input simulation.
}
