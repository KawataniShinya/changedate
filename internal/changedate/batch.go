package changedate

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

func RunBatch(opts Options, out io.Writer) error {
	if opts.BackupCSVDir == "" {
		return fmt.Errorf("mode batch requires --backup-csv-dir")
	}
	if opts.SetBirthTime && currentGOOS != "darwin" {
		fmt.Fprintf(logWriter(opts), "warning: --set-birthtime is macOS-only; ignoring on %s\n", currentGOOS)
		opts.SetBirthTime = false
	}

	fmt.Fprintln(logWriter(opts), "batch: collecting targets")
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

	backupPath, err := writeBackupCSV(opts.BackupCSVDir, opts.Location, files)
	if err != nil {
		return err
	}
	fmt.Fprintf(logWriter(opts), "backup csv written: %s\n", backupPath)

	autofillPath := deriveBatchAutofillPath(backupPath)
	fmt.Fprintln(logWriter(opts), "batch: autofilling csv")
	if err := AutoFillCSV(Options{
		CSVPath:              backupPath,
		CSVOutPath:           autofillPath,
		Location:             opts.Location,
		LogWriter:            opts.LogWriter,
		AutofillExcludeRegex: opts.AutofillExcludeRegex,
	}, io.Discard); err != nil {
		return err
	}
	fmt.Fprintf(logWriter(opts), "autofill csv written: %s\n", autofillPath)

	previewOpts := opts
	previewOpts.Mode = ModeCSV
	previewOpts.CSVPath = autofillPath
	previewOpts.DryRun = true
	previewOpts.BackupCSVDir = ""
	fmt.Fprintln(logWriter(opts), "batch: dry-run preview")
	if err := Run(previewOpts, out); err != nil {
		return err
	}
	if opts.DryRun {
		fmt.Fprintln(logWriter(opts), "batch: dry-run completed")
		return nil
	}

	applyOpts := opts
	applyOpts.Mode = ModeCSV
	applyOpts.CSVPath = autofillPath
	applyOpts.DryRun = false
	applyOpts.BackupCSVDir = ""
	fmt.Fprintln(logWriter(opts), "batch: applying updates")
	if err := Run(applyOpts, io.Discard); err != nil {
		return err
	}
	fmt.Fprintln(logWriter(opts), "batch update completed")
	return nil
}

func deriveBatchAutofillPath(backupPath string) string {
	dir := filepath.Dir(backupPath)
	base := filepath.Base(backupPath)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	return filepath.Join(dir, base+".autofill.csv")
}
