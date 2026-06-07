package changedate

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	exactFilenameTimePattern = regexp.MustCompile(`(?i)(?:^|[^0-9])(\d{8})[_-](\d{6})(\d{3})?(?:[^0-9]|$)`)
	dateOnlyFilenamePattern  = regexp.MustCompile(`(?i)(?:^|[^0-9])(\d{8})(?:[^0-9]|$)`)
)

type csvAutoRow struct {
	index      int
	path       string
	datetime   string
	recognized bool
	exactTime  *time.Time
	dateOnly   *time.Time
}

func AutoFillCSV(opts Options, out io.Writer) error {
	if opts.Location == nil {
		opts.Location = time.Local
	}
	if opts.CSVPath == "" {
		return fmt.Errorf("mode csv-autofill requires --csv")
	}

	rows, pathIdx, datetimeIdx, hasHeader, err := readCSVRows(opts.CSVPath)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return fmt.Errorf("csv %s is empty", opts.CSVPath)
	}

	start := 0
	if hasHeader {
		start = 1
	}
	var excludePattern *regexp.Regexp
	if opts.AutofillExcludeRegex != "" {
		excludePattern, err = regexp.Compile(opts.AutofillExcludeRegex)
		if err != nil {
			return fmt.Errorf("compile --autofill-exclude-regex: %w", err)
		}
	}
	fmt.Fprintf(logWriter(opts), "csv-autofill: %d input rows\n", len(rows)-start)
	progress := newProgressReporter(logWriter(opts), "csv-autofill", len(rows)-start)
	progress.Start("scanning rows")
	skippedByRegex := 0
	autoRows := make([]csvAutoRow, 0, len(rows)-start)
	for i := start; i < len(rows); i++ {
		row := rows[i]
		detail := ""
		if pathIdx < len(row) {
			detail = strings.TrimSpace(row[pathIdx])
		}
		progress.Step(i-start+1, detail)
		if pathIdx >= len(row) {
			continue
		}
		fullPath := strings.TrimSpace(row[pathIdx])
		baseName := filepath.Base(fullPath)
		if excludePattern != nil && excludePattern.MatchString(baseName) {
			skippedByRegex++
			continue
		}
		inferred, exact, ok := inferTimeFromFilename(baseName, opts.Location)
		if !ok {
			continue
		}
		autoRows = append(autoRows, csvAutoRow{
			index:      i,
			path:       fullPath,
			recognized: true,
			exactTime:  exact,
			dateOnly:   inferred,
		})
	}
	progress.Done("scan complete")
	if skippedByRegex > 0 {
		fmt.Fprintf(logWriter(opts), "csv-autofill: skipped %d rows by exclude regex\n", skippedByRegex)
	}

	// Assign sequential times for date-only rows within each date bucket.
	type bucketRow struct {
		index int
		path  string
		date  time.Time
	}
	buckets := make(map[string][]bucketRow)
	for _, row := range autoRows {
		if row.dateOnly == nil || row.exactTime != nil {
			continue
		}
		key := row.dateOnly.Format("2006-01-02")
		buckets[key] = append(buckets[key], bucketRow{index: row.index, path: row.path, date: *row.dateOnly})
	}

	for _, bucket := range buckets {
		sort.SliceStable(bucket, func(i, j int) bool {
			if bucket[i].path == bucket[j].path {
				return bucket[i].index < bucket[j].index
			}
			return bucket[i].path < bucket[j].path
		})
		for seq, item := range bucket {
			updated := item.date.Add(time.Duration(seq) * time.Second)
			rows[item.index] = ensureCSVRowLen(rows[item.index], datetimeIdx+1)
			rows[item.index][datetimeIdx] = formatCSVTime(updated)
		}
	}

	for _, row := range autoRows {
		if row.exactTime != nil {
			rows[row.index] = ensureCSVRowLen(rows[row.index], datetimeIdx+1)
			rows[row.index][datetimeIdx] = formatCSVTime(*row.exactTime)
		}
	}

	if opts.DryRun {
		return writeCSVRows(out, rows)
	}

	outputPath := opts.CSVOutPath
	if outputPath == "" {
		outputPath = opts.CSVPath
	}
	fmt.Fprintf(logWriter(opts), "csv-autofill: writing %s\n", outputPath)
	return writeCSVFileAtomic(outputPath, rows)
}

func readCSVRows(path string) ([][]string, int, int, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, -1, -1, false, fmt.Errorf("open csv %s: %w", path, err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1

	rows, err := reader.ReadAll()
	if err != nil {
		return nil, -1, -1, false, fmt.Errorf("read csv %s: %w", path, err)
	}
	if len(rows) == 0 {
		return nil, -1, -1, false, nil
	}

	hasHeader := isCSVHeader(rows[0])
	pathIdx := 0
	if idx := findPathColumn(rows[0]); idx >= 0 {
		pathIdx = idx
	}
	datetimeIdx := 1
	if idx := findDatetimeColumn(rows[0]); idx >= 0 {
		datetimeIdx = idx
	}
	return rows, pathIdx, datetimeIdx, hasHeader, nil
}

func findPathColumn(header []string) int {
	for i, cell := range header {
		switch normalizeHeader(cell) {
		case "path", "filepath", "file":
			return i
		}
	}
	return -1
}

func findDatetimeColumn(header []string) int {
	for i, cell := range header {
		switch normalizeHeader(cell) {
		case "datetime", "timestamp", "mtime", "time", "updatedat", "updatedtime":
			return i
		}
	}
	return -1
}

func inferTimeFromFilename(name string, loc *time.Location) (*time.Time, *time.Time, bool) {
	if name == "" {
		return nil, nil, false
	}
	if m := exactFilenameTimePattern.FindStringSubmatch(name); len(m) == 4 {
		value := m[1] + "_" + m[2]
		layout := "20060102_150405"
		if m[3] != "" {
			value += "." + m[3]
			layout = "20060102_150405.000"
		}
		ts, err := time.ParseInLocation(layout, value, loc)
		if err == nil {
			return nil, &ts, true
		}
	}
	if m := dateOnlyFilenamePattern.FindStringSubmatch(name); len(m) == 2 {
		ts, err := time.ParseInLocation("20060102", m[1], loc)
		if err == nil {
			return &ts, nil, true
		}
	}
	if ts, ok := inferUnixTimestampFromFilename(name, loc); ok {
		return nil, &ts, true
	}
	return nil, nil, false
}

func inferUnixTimestampFromFilename(name string, loc *time.Location) (time.Time, bool) {
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	if stem == "" || !isDigits(stem) {
		return time.Time{}, false
	}

	switch len(stem) {
	case 10:
		sec, err := strconv.ParseInt(stem, 10, 64)
		if err != nil {
			return time.Time{}, false
		}
		return time.Unix(sec, 0).In(loc), true
	case 13:
		ms, err := strconv.ParseInt(stem, 10, 64)
		if err != nil {
			return time.Time{}, false
		}
		sec := ms / 1e3
		nsec := (ms % 1e3) * 1e6
		return time.Unix(sec, nsec).In(loc), true
	case 16:
		us, err := strconv.ParseInt(stem, 10, 64)
		if err != nil {
			return time.Time{}, false
		}
		sec := us / 1e6
		nsec := (us % 1e6) * 1e3
		return time.Unix(sec, nsec).In(loc), true
	case 19:
		ns, err := strconv.ParseInt(stem, 10, 64)
		if err != nil {
			return time.Time{}, false
		}
		sec := ns / 1e9
		nsec := ns % 1e9
		return time.Unix(sec, nsec).In(loc), true
	default:
		return time.Time{}, false
	}
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func formatCSVTime(ts time.Time) string {
	if ts.Nanosecond() == 0 {
		return formatTime(ts)
	}
	switch {
	case ts.Nanosecond()%1_000_000 == 0:
		return ts.Format("2006-01-02 15:04:05.000")
	case ts.Nanosecond()%1_000 == 0:
		return ts.Format("2006-01-02 15:04:05.000000")
	default:
		return ts.Format("2006-01-02 15:04:05.000000000")
	}
}

func writeCSVFileAtomic(path string, rows [][]string) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".changedate-*.csv")
	if err != nil {
		return fmt.Errorf("create temp csv: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if err := writeCSVRows(tmp, rows); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace csv %s: %w", path, err)
	}
	return nil
}

func writeCSVRows(out io.Writer, rows [][]string) error {
	w := csv.NewWriter(out)
	for _, row := range rows {
		if err := w.Write(row); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func ensureCSVRowLen(row []string, minLen int) []string {
	if len(row) >= minLen {
		return row
	}
	padded := make([]string, minLen)
	copy(padded, row)
	return padded
}
