package notify

import (
	"testing"
)

func TestAvailable(t *testing.T) {
	// Available should return a bool without panicking on any platform.
	got := Available()
	t.Logf("notify.Available() = %v", got)
}

func TestSendDoesNotPanic(t *testing.T) {
	// Send must never panic regardless of platform or missing tools.
	err := Send("prflow test", "This is a test notification")
	if err != nil {
		t.Errorf("Send should never return an error (best-effort), got: %v", err)
	}
}

func TestSendEmptyStrings(t *testing.T) {
	// Edge case: empty title and body should not panic.
	err := Send("", "")
	if err != nil {
		t.Errorf("Send with empty strings should not error: %v", err)
	}
}
