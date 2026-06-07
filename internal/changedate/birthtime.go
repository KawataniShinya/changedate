package changedate

import (
	"fmt"
	"os/exec"
	"time"
)

var setFileLookPath = exec.LookPath
var execCommand = exec.Command

func setBirthTime(path string, t time.Time) error {
	if currentGOOS != "darwin" {
		return nil
	}
	formatted := t.Format("01/02/2006 15:04:05")
	return runSetFile(path, formatted)
}

var runSetFile = func(path, formatted string) error {
	cmd := execCommand("SetFile", "-d", formatted, path)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run SetFile: %w", err)
	}
	return nil
}
