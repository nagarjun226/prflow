package gh

import (
	"os/exec"
	"strings"
)

// Runner executes gh commands. Interface allows test mocking.
type Runner interface {
	Run(args ...string) (string, error)
}

// CLIRunner shells out to the real gh CLI
type CLIRunner struct{}

func (r CLIRunner) Run(args ...string) (string, error) {
	cmd := exec.Command("gh", args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// defaultRunner is the package-level runner
var defaultRunner Runner = CLIRunner{}

// SetRunner allows overriding the runner (for tests)
func SetRunner(r Runner) {
	defaultRunner = r
}

// run uses the package-level runner
func run(args ...string) (string, error) {
	return defaultRunner.Run(args...)
}
