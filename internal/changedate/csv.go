package changedate

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func CollectFromCSV(csvPath string, loc *time.Location) ([]FileEntry, error) {
	if loc == nil {
		loc = time.Local
	}

	f, err := os.Open(csvPath)
	if err != nil {
		return nil, fmt.Errorf("open csv %s: %w", csvPath, err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1

	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read csv %s: %w", csvPath, err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("csv %s is empty", csvPath)
	}

	start := 0
	if isCSVHeader(rows[0]) {
		start = 1
	}

	baseDir := filepath.Dir(csvPath)
	entries := make([]FileEntry, 0, len(rows)-start)
	for i := start; i < len(rows); i++ {
		row := rows[i]
		if len(row) < 2 {
			return nil, fmt.Errorf("csv %s row %d: expected 2 columns, got %d", csvPath, i+1, len(row))
		}

		rawPath := strings.TrimSpace(row[0])
		rawTime := strings.TrimSpace(row[1])
		if rawPath == "" {
			return nil, fmt.Errorf("csv %s row %d: empty file path", csvPath, i+1)
		}
		if rawTime == "" {
			return nil, fmt.Errorf("csv %s row %d: empty datetime", csvPath, i+1)
		}

		resolvedPath := rawPath
		if !filepath.IsAbs(rawPath) {
			resolvedPath = filepath.Clean(filepath.Join(baseDir, rawPath))
		}

		info, err := os.Stat(resolvedPath)
		if err != nil {
			if os.IsNotExist(err) {
				entries = append(entries, FileEntry{
					Path:      resolvedPath,
					Base:      strings.ToLower(filepath.Base(resolvedPath)),
					Skipped:   true,
					SkipCause: "file does not exist",
				})
				continue
			}
			return nil, fmt.Errorf("csv %s row %d: stat %s: %w", csvPath, i+1, resolvedPath, err)
		}
		if info.IsDir() {
			return nil, fmt.Errorf("csv %s row %d: %s is a directory", csvPath, i+1, resolvedPath)
		}

		ext := strings.ToLower(filepath.Ext(resolvedPath))
		if _, ok := allowedExt[ext]; !ok {
			return nil, fmt.Errorf("csv %s row %d: %s has unsupported extension %s", csvPath, i+1, resolvedPath, ext)
		}

		ts, err := ParseInputTime(rawTime, loc)
		if err != nil {
			return nil, fmt.Errorf("csv %s row %d: parse datetime %q: %w", csvPath, i+1, rawTime, err)
		}

		parsed := ts
		entries = append(entries, FileEntry{
			Path:       resolvedPath,
			Base:       strings.ToLower(filepath.Base(resolvedPath)),
			Ext:        ext,
			ModTime:    info.ModTime(),
			TargetTime: &parsed,
		})
	}

	return entries, nil
}

func isCSVHeader(row []string) bool {
	if len(row) < 2 {
		return false
	}
	return isPathHeader(row[0]) && isTimeHeader(row[1])
}

func isPathHeader(value string) bool {
	switch normalizeHeader(value) {
	case "path", "filepath", "file":
		return true
	default:
		return false
	}
}

func isTimeHeader(value string) bool {
	switch normalizeHeader(value) {
	case "datetime", "timestamp", "mtime", "time", "updatedat", "updatedtime":
		return true
	default:
		return false
	}
}

func normalizeHeader(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "_", "")
	value = strings.ReplaceAll(value, "-", "")
	return value
}
