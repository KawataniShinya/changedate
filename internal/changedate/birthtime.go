package changedate

import (
	"fmt"
	"os/exec"
	"time"
)

var setFileLookPath = exec.LookPath
var execCommand = exec.Command

func setBirthTime(path string, t time.Time) error {
	switch currentGOOS {
	case "darwin":
		formatted := t.Format("01/02/2006 15:04:05")
		return runSetFile(path, formatted)
	case "windows":
		return setBirthTimeWindows(path, t)
	default:
		return nil
	}
}

var runSetFile = func(path, formatted string) error {
	cmd := execCommand("SetFile", "-d", formatted, path)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run SetFile: %w", err)
	}
	return nil
}
