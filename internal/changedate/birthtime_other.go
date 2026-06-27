//go:build !windows

package changedate

import "time"

func setBirthTimeWindows(path string, t time.Time) error {
	return nil
}

