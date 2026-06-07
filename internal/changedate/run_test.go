package changedate

import (
	"bytes"
	"encoding/csv"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCollectFilesSortsAndFilters(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "b.JPG"))
	mustWriteFile(t, filepath.Join(root, "a.jpg"))
	mustWriteFile(t, filepath.Join(root, "sub", "c.mp4"))
	mustWriteFile(t, filepath.Join(root, "sub", "note.txt"))

	files, err := CollectFiles(root)
	if err != nil {
		t.Fatalf("CollectFiles: %v", err)
	}
	if got, want := len(files), 3; got != want {
		t.Fatalf("len(files) = %d, want %d", got, want)
	}
	if files[0].Base != "a.jpg" || files[1].Base != "b.jpg" || files[2].Base != "c.mp4" {
		t.Fatalf("unexpected order: %#v", files)
	}
}

func TestCollectFilesFollowsSymlinkDir(t *testing.T) {
	root := t.TempDir()
	actual := filepath.Join(root, "actual")
	link := filepath.Join(root, "link")
	mustWriteFile(t, filepath.Join(actual, "photo.jpg"))
	if err := os.Symlink(actual, link); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	files, err := CollectFiles(link)
	if err != nil {
		t.Fatalf("CollectFiles: %v", err)
	}
	if got, want := len(files), 1; got != want {
		t.Fatalf("len(files) = %d, want %d", got, want)
	}
	if files[0].Path != filepath.Join(link, "photo.jpg") {
		t.Fatalf("path = %q, want %q", files[0].Path, filepath.Join(link, "photo.jpg"))
	}
}

func TestRunSequenceDryRunDoesNotChangeMTime(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "photo.jpg")
	mustWriteFile(t, path)

	before := time.Date(2024, 1, 1, 8, 0, 0, 0, time.Local)
	if err := os.Chtimes(path, before, before); err != nil {
		t.Fatalf("initial chtimes: %v", err)
	}

	opts := Options{
		Dir:       root,
		Mode:      ModeSequence,
		DryRun:    true,
		StartTime: time.Date(2024, 1, 1, 10, 0, 0, 0, time.Local),
		Step:      time.Minute,
	}
	if err := Run(opts, os.Stdout); err != nil {
		t.Fatalf("Run: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !info.ModTime().Equal(before) {
		t.Fatalf("mtime changed in dry-run: got %v want %v", info.ModTime(), before)
	}
}

func TestRunAllChangesMTime(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "photo.mp4")
	mustWriteFile(t, path)

	target := time.Date(2024, 2, 3, 4, 5, 6, 0, time.Local)
	opts := Options{
		Dir:       root,
		Mode:      ModeAll,
		FixedTime: target,
	}
	if err := Run(opts, os.Stdout); err != nil {
		t.Fatalf("Run: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !info.ModTime().Equal(target) {
		t.Fatalf("mtime = %v, want %v", info.ModTime(), target)
	}
}

func TestRunCreatesBackupCSVBeforeMutating(t *testing.T) {
	root := t.TempDir()
	backupDir := filepath.Join(root, "backup")
	path := filepath.Join(root, "photo.jpg")
	mustWriteFile(t, path)

	before := time.Date(2024, 1, 1, 8, 0, 0, 0, time.Local)
	if err := os.Chtimes(path, before, before); err != nil {
		t.Fatalf("initial chtimes: %v", err)
	}

	target := time.Date(2024, 2, 3, 4, 5, 6, 0, time.Local)
	var buf bytes.Buffer
	opts := Options{
		Dir:          root,
		Mode:         ModeAll,
		FixedTime:    target,
		BackupCSVDir: backupDir,
	}
	if err := Run(opts, &buf); err != nil {
		t.Fatalf("Run: %v", err)
	}

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("ReadDir backup: %v", err)
	}
	if got, want := len(entries), 1; got != want {
		t.Fatalf("len(backup entries) = %d, want %d", got, want)
	}
	if !strings.HasPrefix(entries[0].Name(), "changedate-backup-") || !strings.HasSuffix(entries[0].Name(), ".csv") {
		t.Fatalf("unexpected backup filename: %s", entries[0].Name())
	}

	backupPath := filepath.Join(backupDir, entries[0].Name())
	data, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("ReadFile backup: %v", err)
	}
	got := strings.TrimSpace(string(data))
	want := strings.Join([]string{
		"path,datetime",
		path + ",2024-01-01 08:00:00",
	}, "\n")
	if got != want {
		t.Fatalf("backup csv mismatch\n got:\n%s\nwant:\n%s", got, want)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !info.ModTime().Equal(target) {
		t.Fatalf("mtime = %v, want %v", info.ModTime(), target)
	}
}

func TestRunBatchWritesBackupAndAutofillCSV(t *testing.T) {
	root := t.TempDir()
	backupDir := filepath.Join(root, "backup")
	path := filepath.Join(root, "20260515_193217.JPG")
	mustWriteFile(t, path)

	before := time.Date(2024, 1, 1, 8, 0, 0, 0, time.Local)
	if err := os.Chtimes(path, before, before); err != nil {
		t.Fatalf("initial chtimes: %v", err)
	}

	var buf bytes.Buffer
	opts := Options{
		Dir:          root,
		Mode:         ModeBatch,
		BackupCSVDir: backupDir,
	}
	if err := Run(opts, &buf); err != nil {
		t.Fatalf("Run: %v", err)
	}

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("ReadDir backup: %v", err)
	}
	if got, want := len(entries), 2; got != want {
		t.Fatalf("len(backup entries) = %d, want %d", got, want)
	}

	var backupPath, autofillPath string
	for _, entry := range entries {
		switch {
		case strings.Contains(entry.Name(), ".autofill.csv"):
			autofillPath = filepath.Join(backupDir, entry.Name())
		case strings.HasPrefix(entry.Name(), "changedate-backup-") && strings.HasSuffix(entry.Name(), ".csv"):
			backupPath = filepath.Join(backupDir, entry.Name())
		}
	}
	if backupPath == "" || autofillPath == "" {
		t.Fatalf("missing backup/autofill csv: backup=%q autofill=%q", backupPath, autofillPath)
	}

	backupData, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("ReadFile backup: %v", err)
	}
	if !strings.Contains(string(backupData), path+",2024-01-01 08:00:00") {
		t.Fatalf("backup csv does not contain original time:\n%s", string(backupData))
	}

	autofillData, err := os.ReadFile(autofillPath)
	if err != nil {
		t.Fatalf("ReadFile autofill: %v", err)
	}
	if !strings.Contains(string(autofillData), path+",2026-05-15 19:32:17") {
		t.Fatalf("autofill csv does not contain inferred time:\n%s", string(autofillData))
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	want := time.Date(2026, 5, 15, 19, 32, 17, 0, time.Local)
	if !info.ModTime().Equal(want) {
		t.Fatalf("mtime = %v, want %v", info.ModTime(), want)
	}

	got := buf.String()
	if !strings.Contains(got, "path\tbefore\tafter") {
		t.Fatalf("missing dry-run output:\n%s", got)
	}
}

func TestRunBatchHonorsAutofillExcludeRegex(t *testing.T) {
	root := t.TempDir()
	backupDir := filepath.Join(root, "backup")
	pxl := filepath.Join(root, "PXL_20260529_041553956.jpg")
	img := filepath.Join(root, "IMG_20260529_041553956.jpg")
	mustWriteFile(t, pxl)
	mustWriteFile(t, img)

	pxlTime := time.Date(2024, 1, 1, 8, 0, 0, 0, time.Local)
	imgTime := time.Date(2024, 1, 1, 8, 1, 0, 0, time.Local)
	if err := os.Chtimes(pxl, pxlTime, pxlTime); err != nil {
		t.Fatalf("initial chtimes pxl: %v", err)
	}
	if err := os.Chtimes(img, imgTime, imgTime); err != nil {
		t.Fatalf("initial chtimes img: %v", err)
	}

	var buf bytes.Buffer
	opts := Options{
		Dir:                  root,
		Mode:                 ModeBatch,
		BackupCSVDir:         backupDir,
		AutofillExcludeRegex: "^PXL",
	}
	if err := Run(opts, &buf); err != nil {
		t.Fatalf("Run: %v", err)
	}

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("ReadDir backup: %v", err)
	}
	var autofillPath string
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".autofill.csv") {
			autofillPath = filepath.Join(backupDir, entry.Name())
			break
		}
	}
	if autofillPath == "" {
		t.Fatal("missing autofill csv")
	}
	data, err := os.ReadFile(autofillPath)
	if err != nil {
		t.Fatalf("ReadFile autofill: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, pxl+",2024-01-01 08:00:00") {
		t.Fatalf("excluded row should stay unchanged:\n%s", got)
	}
	if !strings.Contains(got, img+",2026-05-29 04:15:53.956") {
		t.Fatalf("autofilled row missing:\n%s", got)
	}
}

func TestRunBatchWorksWithRelativePaths(t *testing.T) {
	root := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldwd)
	})

	backupDir := filepath.Join(".", "backup")
	path := filepath.Join("photo", "20260515_193217.JPG")
	mustWriteFile(t, path)

	opts := Options{
		Dir:          ".",
		Mode:         ModeBatch,
		BackupCSVDir: backupDir,
	}
	if err := Run(opts, io.Discard); err != nil {
		t.Fatalf("Run: %v", err)
	}

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("ReadDir backup: %v", err)
	}
	if got, want := len(entries), 2; got != want {
		t.Fatalf("len(backup entries) = %d, want %d", got, want)
	}
}

func TestRunSequenceWithExplicitFilesPreservesOrder(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "b.jpg")
	second := filepath.Join(root, "a.jpg")
	mustWriteFile(t, first)
	mustWriteFile(t, second)

	opts := Options{
		Files:     []string{first, second},
		Mode:      ModeSequence,
		StartTime: time.Date(2024, 1, 1, 10, 0, 0, 0, time.Local),
		Step:      time.Minute,
	}
	if err := Run(opts, os.Stdout); err != nil {
		t.Fatalf("Run: %v", err)
	}

	info1, err := os.Stat(first)
	if err != nil {
		t.Fatalf("Stat first: %v", err)
	}
	info2, err := os.Stat(second)
	if err != nil {
		t.Fatalf("Stat second: %v", err)
	}

	want1 := time.Date(2024, 1, 1, 10, 0, 0, 0, time.Local)
	want2 := time.Date(2024, 1, 1, 10, 1, 0, 0, time.Local)
	if !info1.ModTime().Equal(want1) {
		t.Fatalf("first mtime = %v, want %v", info1.ModTime(), want1)
	}
	if !info2.ModTime().Equal(want2) {
		t.Fatalf("second mtime = %v, want %v", info2.ModTime(), want2)
	}
}

func TestDryRunOutputIsSortedByTargetTime(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "b.jpg")
	second := filepath.Join(root, "a.jpg")
	mustWriteFile(t, first)
	mustWriteFile(t, second)

	var buf bytes.Buffer
	opts := Options{
		Files:     []string{first, second},
		Mode:      ModeSequence,
		DryRun:    true,
		StartTime: time.Date(2024, 1, 1, 10, 0, 0, 0, time.Local),
		Step:      time.Minute,
	}
	if err := Run(opts, &buf); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := buf.String()
	if !strings.HasPrefix(got, "path\tbefore\tafter\n") {
		t.Fatalf("missing header:\n%s", got)
	}
	firstIdx := strings.Index(got, first+"\t")
	secondIdx := strings.Index(got, second+"\t")
	if firstIdx == -1 || secondIdx == -1 {
		t.Fatalf("missing output lines:\n%s", got)
	}
	if !(firstIdx < secondIdx) {
		t.Fatalf("dry-run output not sorted by time:\n%s", got)
	}
}

func TestCollectSpecificFilesRejectsUnsupportedExtension(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "note.txt")
	mustWriteFile(t, path)

	if _, err := CollectSpecificFiles([]string{path}); err == nil {
		t.Fatal("expected error")
	}
}

func TestCollectFromCSV(t *testing.T) {
	root := t.TempDir()
	target1 := filepath.Join(root, "a.jpg")
	target2 := filepath.Join(root, "b.mp4")
	mustWriteFile(t, target1)
	mustWriteFile(t, target2)

	csvPath := filepath.Join(root, "changes.csv")
	if err := writeCSVFile(csvPath, [][]string{
		{"path", "datetime"},
		{"./a.jpg", "2024-01-01 10:00:00"},
		{"./b.mp4", "2024-01-01 10:01:00"},
	}); err != nil {
		t.Fatalf("writeCSVFile: %v", err)
	}

	entries, err := CollectFromCSV(csvPath, time.Local)
	if err != nil {
		t.Fatalf("CollectFromCSV: %v", err)
	}
	if got, want := len(entries), 2; got != want {
		t.Fatalf("len(entries) = %d, want %d", got, want)
	}
	if entries[0].Path != target1 || entries[1].Path != target2 {
		t.Fatalf("unexpected paths: %#v", entries)
	}
	if entries[0].TargetTime == nil || entries[1].TargetTime == nil {
		t.Fatal("expected target times")
	}
	want1 := time.Date(2024, 1, 1, 10, 0, 0, 0, time.Local)
	want2 := time.Date(2024, 1, 1, 10, 1, 0, 0, time.Local)
	if !entries[0].TargetTime.Equal(want1) {
		t.Fatalf("entry 0 target = %v, want %v", entries[0].TargetTime, want1)
	}
	if !entries[1].TargetTime.Equal(want2) {
		t.Fatalf("entry 1 target = %v, want %v", entries[1].TargetTime, want2)
	}
}

func TestRunCSVSkipsMissingFiles(t *testing.T) {
	root := t.TempDir()
	existing := filepath.Join(root, "a.jpg")
	missing := filepath.Join(root, "missing.jpg")
	mustWriteFile(t, existing)

	csvPath := filepath.Join(root, "changes.csv")
	if err := writeCSVFile(csvPath, [][]string{
		{"path", "datetime"},
		{"./a.jpg", "2024-01-01 10:00:00"},
		{"./missing.jpg", "2024-01-01 10:01:00"},
	}); err != nil {
		t.Fatalf("writeCSVFile: %v", err)
	}

	var buf bytes.Buffer
	opts := Options{
		Mode:    ModeCSV,
		CSVPath: csvPath,
	}
	if err := Run(opts, &buf); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "SKIP\t"+missing+"\t-\tfile does not exist") {
		t.Fatalf("missing skip output:\n%s", got)
	}
	if !strings.Contains(got, existing+"\t") {
		t.Fatalf("missing existing file output:\n%s", got)
	}
}

func TestExportCSVSortsByModTime(t *testing.T) {
	root := t.TempDir()
	older := filepath.Join(root, "b.mp4")
	newer := filepath.Join(root, "a.jpg")
	mustWriteFile(t, older)
	mustWriteFile(t, newer)

	oldTime := time.Date(2024, 1, 1, 10, 0, 0, 0, time.Local)
	newTime := time.Date(2024, 1, 1, 10, 1, 0, 0, time.Local)
	if err := os.Chtimes(older, oldTime, oldTime); err != nil {
		t.Fatalf("Chtimes older: %v", err)
	}
	if err := os.Chtimes(newer, newTime, newTime); err != nil {
		t.Fatalf("Chtimes newer: %v", err)
	}

	var buf bytes.Buffer
	opts := Options{
		Dir:  root,
		Mode: ModeExport,
	}
	if err := Run(opts, &buf); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := strings.TrimSpace(buf.String())
	want := strings.Join([]string{
		"path,datetime",
		older + ",2024-01-01 10:00:00",
		newer + ",2024-01-01 10:01:00",
	}, "\n")
	if got != want {
		t.Fatalf("csv output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestParseInputTimeSupportsCompactCSVFormat(t *testing.T) {
	got, err := ParseInputTime("20240101_100000", time.Local)
	if err != nil {
		t.Fatalf("ParseInputTime: %v", err)
	}
	want := time.Date(2024, 1, 1, 10, 0, 0, 0, time.Local)
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestAutoFillCSVInfersFullAndFractionalTimes(t *testing.T) {
	root := t.TempDir()
	csvPath := filepath.Join(root, "changes.csv")
	if err := writeCSVFile(csvPath, [][]string{
		{"path", "datetime"},
		{"./20260515_193217.JPG", ""},
		{"./PXL_20260514_065248267.jpg", "2024-01-01 00:00:00"},
		{"./PXL_20260514_065100000.jpg", "2024-01-01 00:00:00"},
	}); err != nil {
		t.Fatalf("writeCSVFile: %v", err)
	}

	var buf bytes.Buffer
	opts := Options{
		CSVPath: csvPath,
		Mode:    ModeCSVAuto,
		DryRun:  true,
	}
	if err := Run(opts, &buf); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "./20260515_193217.JPG,2026-05-15 19:32:17") {
		t.Fatalf("missing full timestamp inference:\n%s", got)
	}
	if !strings.Contains(got, "./PXL_20260514_065248267.jpg,2026-05-14 06:52:48.267") {
		t.Fatalf("missing milliseconds inference:\n%s", got)
	}
	if !strings.Contains(got, "./PXL_20260514_065100000.jpg,2026-05-14 06:51:00") {
		t.Fatalf("missing zero-millisecond inference:\n%s", got)
	}
}

func TestAutoFillCSVDateOnlySequences(t *testing.T) {
	root := t.TempDir()
	csvPath := filepath.Join(root, "changes.csv")
	if err := writeCSVFile(csvPath, [][]string{
		{"path", "datetime"},
		{"./PXL_20260514_a.jpg", ""},
		{"./PXL_20260514_b.jpg", ""},
	}); err != nil {
		t.Fatalf("writeCSVFile: %v", err)
	}

	var buf bytes.Buffer
	opts := Options{
		CSVPath: csvPath,
		Mode:    ModeCSVAuto,
		DryRun:  true,
	}
	if err := Run(opts, &buf); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "./PXL_20260514_a.jpg,2026-05-14 00:00:00") {
		t.Fatalf("missing date-only first line:\n%s", got)
	}
	if !strings.Contains(got, "./PXL_20260514_b.jpg,2026-05-14 00:00:01") {
		t.Fatalf("missing date-only second line:\n%s", got)
	}
}

func TestAutoFillCSVExcludesByRegex(t *testing.T) {
	root := t.TempDir()
	csvPath := filepath.Join(root, "changes.csv")
	if err := writeCSVFile(csvPath, [][]string{
		{"path", "datetime"},
		{"./PXL_20260529_041553956.jpg", "2000-01-01 00:00:00"},
		{"./IMG_20260529_041553956.jpg", ""},
	}); err != nil {
		t.Fatalf("writeCSVFile: %v", err)
	}

	var buf bytes.Buffer
	opts := Options{
		CSVPath:              csvPath,
		Mode:                 ModeCSVAuto,
		DryRun:               true,
		AutofillExcludeRegex: "^PXL",
	}
	if err := Run(opts, &buf); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "./PXL_20260529_041553956.jpg,2000-01-01 00:00:00") {
		t.Fatalf("excluded row should remain unchanged:\n%s", got)
	}
	if !strings.Contains(got, "./IMG_20260529_041553956.jpg,2026-05-29 04:15:53.956") {
		t.Fatalf("non-excluded row should be autofilled:\n%s", got)
	}
}

func TestAutoFillCSVInfersUnixTimestamps(t *testing.T) {
	root := t.TempDir()
	csvPath := filepath.Join(root, "changes.csv")
	if err := writeCSVFile(csvPath, [][]string{
		{"path", "datetime"},
		{"./1715921234.jpg", ""},
		{"./1715921234567.jpg", ""},
	}); err != nil {
		t.Fatalf("writeCSVFile: %v", err)
	}

	var buf bytes.Buffer
	opts := Options{
		CSVPath: csvPath,
		Mode:    ModeCSVAuto,
		DryRun:  true,
	}
	if err := Run(opts, &buf); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := buf.String()
	wantSeconds := time.Unix(1715921234, 0).In(time.Local).Format("2006-01-02 15:04:05")
	wantMillis := time.Unix(1715921234, 567000000).In(time.Local).Format("2006-01-02 15:04:05")
	if !strings.Contains(got, "./1715921234.jpg,"+wantSeconds) {
		t.Fatalf("missing unix seconds inference:\n%s", got)
	}
	if !strings.Contains(got, "./1715921234567.jpg,"+wantMillis) {
		t.Fatalf("missing unix milliseconds inference:\n%s", got)
	}
}

func TestAutoFillCSVInfersPhotoTimesWithOptionalMillis(t *testing.T) {
	root := t.TempDir()
	csvPath := filepath.Join(root, "changes.csv")
	if err := writeCSVFile(csvPath, [][]string{
		{"path", "datetime"},
		{"./PXL_20260529_041553956.jpg", ""},
		{"./PXL_20260529_041553.jpg", ""},
	}); err != nil {
		t.Fatalf("writeCSVFile: %v", err)
	}

	var buf bytes.Buffer
	opts := Options{
		CSVPath: csvPath,
		Mode:    ModeCSVAuto,
		DryRun:  true,
	}
	if err := Run(opts, &buf); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "./PXL_20260529_041553956.jpg,2026-05-29 04:15:53.956") {
		t.Fatalf("missing milliseconds inference:\n%s", got)
	}
	if !strings.Contains(got, "./PXL_20260529_041553.jpg,2026-05-29 04:15:53") {
		t.Fatalf("missing second-only inference:\n%s", got)
	}
}

func TestParseInputTimeSupportsFractionalSeconds(t *testing.T) {
	got, err := ParseInputTime("2026-05-29 04:15:53.956", time.Local)
	if err != nil {
		t.Fatalf("ParseInputTime: %v", err)
	}
	want := time.Date(2026, 5, 29, 4, 15, 53, 956000000, time.Local)
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func writeCSVFile(path string, records [][]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.WriteAll(records); err != nil {
		return err
	}
	w.Flush()
	return w.Error()
}

func mustWriteFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte("dummy"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}
