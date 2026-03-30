package notify

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// Send delivers a desktop notification. It is best-effort: if the notification
// system is unavailable it prints to stderr and returns nil.
func Send(title, body string) error {
	switch runtime.GOOS {
	case "linux":
		if _, err := exec.LookPath("notify-send"); err == nil {
			_ = exec.Command("notify-send", title, body).Run()
			return nil
		}
	case "darwin":
		script := fmt.Sprintf(`display notification %q with title %q`, body, title)
		_ = exec.Command("osascript", "-e", script).Run()
		return nil
	case "windows":
		ps := fmt.Sprintf(`New-BurntToastNotification -Text '%s','%s'`, title, body)
		_ = exec.Command("powershell", "-Command", ps).Run()
		return nil
	}
	// Fallback: print to stderr
	if title != "" || body != "" {
		fmt.Fprintf(os.Stderr, "[prflow] %s: %s\n", title, body)
	}
	return nil
}

// Available reports whether the current platform has a supported notification
// mechanism.
func Available() bool {
	switch runtime.GOOS {
	case "darwin":
		_, err := exec.LookPath("osascript")
		return err == nil
	case "linux":
		_, err := exec.LookPath("notify-send")
		return err == nil
	case "windows":
		_, err := exec.LookPath("powershell")
		return err == nil
	default:
		return false
	}
}
