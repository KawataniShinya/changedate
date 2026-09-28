package changedate

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

var setFileLookPath = exec.LookPath
var execCommand = exec.Command

func validateBirthTimeOptions(opts Options) error {
	if !opts.SetBirthTime {
		return nil
	}
	switch currentGOOS {
	case "darwin", "windows":
		if !opts.DryRun {
			return ensureSetFileAvailable()
		}
	case "linux":
		if opts.SMBShare == "" || opts.SMBRoot == "" || opts.SMBAuthFile == "" {
			return fmt.Errorf("Linux --set-birthtime requires --smb-share, --smb-root, and --smb-auth-file")
		}
		if !strings.HasPrefix(opts.SMBShare, "//") || strings.Count(strings.TrimPrefix(opts.SMBShare, "//"), "/") != 1 {
			return fmt.Errorf("--smb-share must have the form //server/share")
		}
		if _, err := setFileLookPath("smbclient"); err != nil {
			return fmt.Errorf("smbclient not found: %w", err)
		}
		if info, err := os.Stat(opts.SMBAuthFile); err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("invalid --smb-auth-file %s", opts.SMBAuthFile)
		}
	default:
		return fmt.Errorf("--set-birthtime is unsupported on %s", currentGOOS)
	}
	return nil
}

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
