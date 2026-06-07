package changedate

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

func writeBackupCSV(dir string, loc *time.Location, files []FileEntry) (string, error) {
	if dir == "" {
		return "", nil
	}
	if loc == nil {
		loc = time.Local
	}

	entries := make([]FileEntry, 0, len(files))
	for _, file := range files {
		if file.Skipped {
			continue
		}
		entries = append(entries, file)
	}
	if len(entries) == 0 {
		return "", fmt.Errorf("no existing target files to back up")
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].ModTime.Equal(entries[j].ModTime) {
			return entries[i].Path < entries[j].Path
		}
		return entries[i].ModTime.Before(entries[j].ModTime)
	})

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create backup dir %s: %w", dir, err)
	}

	name := fmt.Sprintf("changedate-backup-%s.csv", time.Now().In(loc).Format("20060102-150405.000000000"))
	path := filepath.Join(dir, name)
	rows := make([][]string, 0, len(entries)+1)
	rows = append(rows, []string{"path", "datetime"})
	for _, file := range entries {
		path := file.Path
		if !filepath.IsAbs(path) {
			absPath, err := filepath.Abs(path)
			if err != nil {
				return "", fmt.Errorf("resolve absolute path for %s: %w", path, err)
			}
			path = absPath
		}
		rows = append(rows, []string{path, formatTime(file.ModTime)})
	}
	if err := writeCSVFileAtomic(path, rows); err != nil {
		return "", err
	}
	return path, nil
}
