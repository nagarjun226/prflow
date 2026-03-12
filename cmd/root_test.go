package cmd

import (
	"testing"
)

func TestPrintUsage(t *testing.T) {
	// Should not panic
	printUsage()
}

func TestRunConfig(t *testing.T) {
	err := runConfig()
	if err != nil {
		t.Errorf("runConfig should not error: %v", err)
	}
}

func TestRunList(t *testing.T) {
	err := runList()
	if err != nil {
		t.Errorf("runList should not error: %v", err)
	}
}
