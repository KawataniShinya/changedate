package changedate

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

var currentGOOS = runtime.GOOS

var allowedExt = map[string]struct{}{
	".jpg":  {},
	".jpeg": {},
	".png":  {},
	".heic": {},
	".mp4":  {},
	".mov":  {},
}

func Run(opts Options, out io.Writer) error {
	if opts.Location == nil {
		opts.Location = time.Local
	}
	if opts.Dir == "" {
		opts.Dir = "."
	}
	if err := validateOptions(opts); err != nil {
		return err
	}

	if opts.Mode == ModeExport {
		return ExportCSV(opts, out)
	}
	if opts.Mode == ModeCSVAuto {
		return AutoFillCSV(opts, out)
	}
	if opts.Mode == ModeBatch {
		return RunBatch(opts, out)
	}

	if err := validateBirthTimeOptions(opts); err != nil {
		return err
	}

	files, err := collectTargets(opts)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		if opts.Mode == ModeCSV {
			return fmt.Errorf("no valid target rows found in --csv %s", opts.CSVPath)
		}
		if len(opts.Files) > 0 {
			return fmt.Errorf("no valid target files found in --files")
		}
		return fmt.Errorf("no target files found in %s", opts.Dir)
	}
	if opts.SetBirthTime && currentGOOS == "linux" {
		for _, file := range files {
			if !file.Skipped {
				if _, err := smbRemotePath(opts.SMBRoot, file.Path); err != nil {
					return err
				}
			}
		}
	}
	fmt.Fprintf(logWriter(opts), "%s: %d target files\n", opts.Mode, len(files))

	if !opts.DryRun && opts.BackupCSVDir != "" && isMutatingMode(opts.Mode) {
		backupPath, err := writeBackupCSV(opts.BackupCSVDir, opts.Location, files)
		if err != nil {
			return err
		}
		fmt.Fprintf(logWriter(opts), "backup csv written: %s\n", backupPath)
	}

	progress := newProgressReporter(logWriter(opts), string(opts.Mode), len(files))
	progress.Start("processing files")
	changes := make([]Change, 0, len(files))
	for i, file := range files {
		progress.Step(i+1, file.Path)
		if file.Skipped {
			changes = append(changes, Change{
				Path:      file.Path,
				Skipped:   true,
				SkipCause: file.SkipCause,
			})
			continue
		}
		after, skipped, cause, err := resolveTargetTime(opts, file, i)
		if err != nil {
			return err
		}
		ch := Change{
			Path:    file.Path,
			Before:  file.ModTime,
			After:   after,
			Skipped: skipped,
		}
		if skipped {
			ch.SkipCause = cause
			changes = append(changes, ch)
			continue
		}

		if !opts.DryRun {
			if err := os.Chtimes(file.Path, file.ModTime, after); err != nil {
				return fmt.Errorf("chtimes %s: %w", file.Path, err)
			}
			if opts.SetBirthTime && currentGOOS != "linux" {
				if err := setBirthTime(file.Path, after); err != nil {
					return fmt.Errorf("set birth time %s: %w", file.Path, err)
				}
			}
		}
		changes = append(changes, ch)
	}
	if opts.SetBirthTime && currentGOOS == "linux" && !opts.DryRun {
		if err := setBirthTimesSMB(opts, changes); err != nil {
			return err
		}
	}
	progress.Done("processing complete")

	if opts.DryRun {
		sortChangesForDisplay(changes)
		fmt.Fprintln(out, "path\tbefore\tafter")
	}
	for _, ch := range changes {
		logChange(out, ch)
	}

	if opts.DryRun {
		fmt.Fprintln(out, "dry-run: no files were modified")
	}

	return nil
}

func ExportCSV(opts Options, out io.Writer) error {
	files, err := collectTargets(opts)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		if len(opts.Files) > 0 {
			return fmt.Errorf("no valid target files found in --files")
		}
		return fmt.Errorf("no target files found in %s", opts.Dir)
	}

	sort.SliceStable(files, func(i, j int) bool {
		if files[i].ModTime.Equal(files[j].ModTime) {
			return files[i].Path < files[j].Path
		}
		return files[i].ModTime.Before(files[j].ModTime)
	})
	fmt.Fprintf(logWriter(opts), "export-csv: %d target files\n", len(files))

	progress := newProgressReporter(logWriter(opts), "export-csv", len(files))
	progress.Start("writing csv")
	w := csv.NewWriter(out)
	if err := w.Write([]string{"path", "datetime"}); err != nil {
		return err
	}
	for i, file := range files {
		if err := w.Write([]string{file.Path, formatTime(file.ModTime)}); err != nil {
			return err
		}
		progress.Step(i+1, file.Path)
	}
	w.Flush()
	progress.Done("csv written")
	return w.Error()
}

func sortChangesForDisplay(changes []Change) {
	sort.SliceStable(changes, func(i, j int) bool {
		left := changes[i]
		right := changes[j]
		if left.Skipped != right.Skipped {
			return !left.Skipped && right.Skipped
		}
		if left.Skipped {
			if left.Before.Equal(right.Before) {
				return left.Path < right.Path
			}
			return left.Before.Before(right.Before)
		}
		if left.After.Equal(right.After) {
			return left.Path < right.Path
		}
		return left.After.Before(right.After)
	})
}

func collectTargets(opts Options) ([]FileEntry, error) {
	if opts.Mode == ModeCSV {
		return CollectFromCSV(opts.CSVPath, opts.Location)
	}
	if len(opts.Files) > 0 {
		return CollectSpecificFiles(opts.Files)
	}
	return CollectFiles(opts.Dir)
}

func validateOptions(opts Options) error {
	switch opts.Mode {
	case ModeAll:
		if opts.FixedTime.IsZero() {
			return fmt.Errorf("mode all requires --datetime")
		}
	case ModeSequence:
		if opts.StartTime.IsZero() {
			return fmt.Errorf("mode sequence requires --start")
		}
		if opts.Step <= 0 {
			return fmt.Errorf("mode sequence requires --step > 0")
		}
	case ModeExif:
	case ModeCSV:
		if opts.CSVPath == "" {
			return fmt.Errorf("mode csv requires --csv")
		}
	case ModeCSVAuto:
		if opts.CSVPath == "" {
			return fmt.Errorf("mode csv-autofill requires --csv")
		}
	case ModeExport:
	case ModeBatch:
		if opts.BackupCSVDir == "" {
			return fmt.Errorf("mode batch requires --backup-csv-dir")
		}
	default:
		return fmt.Errorf("unsupported mode %q", opts.Mode)
	}
	return nil
}

func isMutatingMode(mode Mode) bool {
	switch mode {
	case ModeAll, ModeSequence, ModeExif, ModeCSV:
		return true
	default:
		return false
	}
}

func ensureSetFileAvailable() error {
	if currentGOOS != "darwin" {
		return nil
	}
	if _, err := setFileLookPath("SetFile"); err != nil {
		return fmt.Errorf("SetFile not found: %w. Xcode Command Line Tools をインストールしてください", err)
	}
	return nil
}

func resolveTargetTime(opts Options, file FileEntry, index int) (time.Time, bool, string, error) {
	switch opts.Mode {
	case ModeAll:
		return opts.FixedTime, false, "", nil
	case ModeSequence:
		return opts.StartTime.Add(time.Duration(index) * opts.Step), false, "", nil
	case ModeExif:
		ts, ok := ReadExifTime(file.Path)
		if !ok {
			return time.Time{}, true, "no exif timestamp", nil
		}
		return ts.In(opts.Location), false, "", nil
	case ModeCSV:
		if file.TargetTime == nil {
			return time.Time{}, false, "", fmt.Errorf("missing target time for %s", file.Path)
		}
		return file.TargetTime.In(opts.Location), false, "", nil
	case ModeExport:
		return time.Time{}, true, "export mode does not modify files", nil
	default:
		return time.Time{}, false, "", fmt.Errorf("unsupported mode %q", opts.Mode)
	}
}

func logChange(out io.Writer, ch Change) {
	if ch.Skipped {
		before := "-"
		if !ch.Before.IsZero() {
			before = formatTime(ch.Before)
		}
		fmt.Fprintf(out, "SKIP\t%s\t%s\t%s\n", ch.Path, before, ch.SkipCause)
		return
	}
	fmt.Fprintf(out, "%s\t%s\t%s\n", ch.Path, formatTime(ch.Before), formatTime(ch.After))
}

func formatTime(ts time.Time) string {
	return ts.Format("2006-01-02 15:04:05")
}

func CollectFiles(root string) ([]FileEntry, error) {
	entries := make([]FileEntry, 0)
	visited := make(map[string]struct{})
	if err := collectFilesRecursive(root, visited, &entries); err != nil {
		return nil, err
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Base == entries[j].Base {
			return entries[i].Path < entries[j].Path
		}
		return entries[i].Base < entries[j].Base
	})
	return entries, nil
}

func collectFilesRecursive(dir string, visited map[string]struct{}, entries *[]FileEntry) error {
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", dir, err)
	}
	if _, ok := visited[resolved]; ok {
		return nil
	}
	visited[resolved] = struct{}{}

	children, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, child := range children {
		path := filepath.Join(dir, child.Name())
		if child.Type()&os.ModeSymlink != 0 {
			info, err := os.Stat(path)
			if err != nil {
				return err
			}
			if info.IsDir() {
				if err := collectFilesRecursive(path, visited, entries); err != nil {
					return err
				}
				continue
			}
			ext := strings.ToLower(filepath.Ext(child.Name()))
			if _, ok := allowedExt[ext]; !ok {
				continue
			}
			*entries = append(*entries, FileEntry{
				Path:    path,
				Base:    strings.ToLower(child.Name()),
				Ext:     ext,
				ModTime: info.ModTime(),
			})
			continue
		}
		if child.IsDir() {
			if err := collectFilesRecursive(path, visited, entries); err != nil {
				return err
			}
			continue
		}
		ext := strings.ToLower(filepath.Ext(child.Name()))
		if _, ok := allowedExt[ext]; !ok {
			continue
		}
		info, err := child.Info()
		if err != nil {
			return err
		}
		*entries = append(*entries, FileEntry{
			Path:    path,
			Base:    strings.ToLower(child.Name()),
			Ext:     ext,
			ModTime: info.ModTime(),
		})
	}
	return nil
}

func CollectSpecificFiles(paths []string) ([]FileEntry, error) {
	entries := make([]FileEntry, 0, len(paths))
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", path, err)
		}
		if info.IsDir() {
			return nil, fmt.Errorf("%s is a directory; pass file paths only", path)
		}
		ext := strings.ToLower(filepath.Ext(path))
		if _, ok := allowedExt[ext]; !ok {
			return nil, fmt.Errorf("%s has unsupported extension %s", path, ext)
		}
		entries = append(entries, FileEntry{
			Path:    path,
			Base:    strings.ToLower(filepath.Base(path)),
			Ext:     ext,
			ModTime: info.ModTime(),
		})
	}
	return entries, nil
}
