package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"changedate/internal/changedate"
)

func main() {
	var fileArgs multiStringFlag
	var (
		dir             = flag.String("dir", ".", "Target directory to scan recursively")
		mode            = flag.String("mode", "sequence", "Mode: all, sequence, exif, csv, csv-autofill, export-csv, or batch")
		datetime        = flag.String("datetime", "", "Timestamp for --mode all (e.g. \"2024-01-01 10:00:00\")")
		start           = flag.String("start", "", "Starting timestamp for --mode sequence")
		stepSecs        = flag.Int("step", 60, "Step size in seconds for --mode sequence")
		csvPath         = flag.String("csv", "", "CSV file for --mode csv; columns: path,datetime")
		csvOut          = flag.String("csv-out", "", "Output CSV path for --mode export-csv; defaults to stdout")
		backupDir       = flag.String("backup-csv-dir", "", "Directory to write a timestamped backup CSV before modifying files")
		logFile         = flag.String("log-file", "", "Append stderr-style logs to this file")
		autofillExclude = flag.String("autofill-exclude-regex", "", "Regex to exclude filenames from --mode csv-autofill")
		setBirth        = flag.Bool("set-birthtime", false, "Also set file creation time on macOS/Windows, or Linux SMB shares")
		smbShare        = flag.String("smb-share", "", "SMB share for Linux creation time updates (e.g. //server/share)")
		smbRoot         = flag.String("smb-root", "", "Local mount root corresponding to --smb-share")
		smbAuthFile     = flag.String("smb-auth-file", "", "smbclient authentication file for Linux creation time updates")
		dryRun          = flag.Bool("dry-run", false, "Print planned changes without modifying files")
		timeLocal       = flag.String("timezone", "", "Optional IANA timezone name for parsing input times")
	)
	flag.BoolVar(setBirth, "with-creation-time", false, "Alias for --set-birthtime")
	flag.Var(&fileArgs, "files", "Comma-separated list of target files; may be provided multiple times")
	flag.Parse()

	loc := time.Local
	logSink := io.Writer(os.Stderr)
	var logCloser *os.File
	if *logFile != "" {
		if err := os.MkdirAll(filepath.Dir(*logFile), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "create log dir: %v\n", err)
			os.Exit(1)
		}
		f, err := os.OpenFile(*logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "open --log-file: %v\n", err)
			os.Exit(1)
		}
		logCloser = f
		logSink = io.MultiWriter(os.Stderr, f)
		defer logCloser.Close()
	}

	if *timeLocal != "" {
		loaded, err := time.LoadLocation(*timeLocal)
		if err != nil {
			fmt.Fprintf(logSink, "invalid --timezone: %v\n", err)
			os.Exit(2)
		}
		loc = loaded
	}

	opts := changedate.Options{
		Dir:                  *dir,
		Files:                fileArgs.Values(),
		Mode:                 changedate.Mode(*mode),
		DryRun:               *dryRun,
		Location:             loc,
		SetBirthTime:         *setBirth,
		SMBShare:             *smbShare,
		SMBRoot:              *smbRoot,
		SMBAuthFile:          *smbAuthFile,
		BackupCSVDir:         *backupDir,
		LogWriter:            logSink,
		AutofillExcludeRegex: *autofillExclude,
		CSVPath:              *csvPath,
		CSVOutPath:           *csvOut,
	}

	switch opts.Mode {
	case changedate.ModeAll:
		if *datetime == "" {
			fmt.Fprintln(logSink, "--datetime is required for --mode all")
			os.Exit(2)
		}
		ts, err := changedate.ParseInputTime(*datetime, loc)
		if err != nil {
			fmt.Fprintf(logSink, "invalid --datetime: %v\n", err)
			os.Exit(2)
		}
		opts.FixedTime = ts
	case changedate.ModeSequence:
		if *start == "" {
			fmt.Fprintln(logSink, "--start is required for --mode sequence")
			os.Exit(2)
		}
		if *stepSecs <= 0 {
			fmt.Fprintln(logSink, "--step must be greater than 0")
			os.Exit(2)
		}
		ts, err := changedate.ParseInputTime(*start, loc)
		if err != nil {
			fmt.Fprintf(logSink, "invalid --start: %v\n", err)
			os.Exit(2)
		}
		opts.StartTime = ts
		opts.Step = time.Duration(*stepSecs) * time.Second
	case changedate.ModeExif:
		// no extra args
	case changedate.ModeCSV:
		if *csvPath == "" {
			fmt.Fprintln(logSink, "--csv is required for --mode csv")
			os.Exit(2)
		}
	case changedate.ModeCSVAuto:
		if *csvPath == "" {
			fmt.Fprintln(logSink, "--csv is required for --mode csv-autofill")
			os.Exit(2)
		}
	case changedate.ModeExport:
		// no extra args
	case changedate.ModeBatch:
		if *backupDir == "" {
			fmt.Fprintln(logSink, "--backup-csv-dir is required for --mode batch")
			os.Exit(2)
		}
	default:
		fmt.Fprintf(logSink, "unsupported --mode %q\n", *mode)
		os.Exit(2)
	}

	if *autofillExclude != "" && (opts.Mode == changedate.ModeCSVAuto || opts.Mode == changedate.ModeBatch) {
		if _, err := regexp.Compile(*autofillExclude); err != nil {
			fmt.Fprintf(logSink, "invalid --autofill-exclude-regex: %v\n", err)
			os.Exit(2)
		}
	}

	var out *os.File = os.Stdout
	if opts.Mode == changedate.ModeExport && *csvOut != "" {
		f, err := os.Create(*csvOut)
		if err != nil {
			fmt.Fprintf(logSink, "create --csv-out: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		out = f
	}

	if err := changedate.Run(opts, out); err != nil {
		fmt.Fprintln(logSink, err)
		os.Exit(1)
	}
}

type multiStringFlag []string

func (m *multiStringFlag) String() string {
	return strings.Join(*m, ",")
}

func (m *multiStringFlag) Set(value string) error {
	if value == "" {
		return nil
	}
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		*m = append(*m, part)
	}
	return nil
}

func (m *multiStringFlag) Values() []string {
	out := make([]string, len(*m))
	copy(out, *m)
	return out
}
