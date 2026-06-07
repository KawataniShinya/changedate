package changedate

import (
	"io"
	"time"
)

type Mode string

const (
	ModeAll      Mode = "all"
	ModeSequence Mode = "sequence"
	ModeExif     Mode = "exif"
	ModeCSV      Mode = "csv"
	ModeCSVAuto  Mode = "csv-autofill"
	ModeExport   Mode = "export-csv"
	ModeBatch    Mode = "batch"
)

type Options struct {
	Dir                  string
	Files                []string
	Mode                 Mode
	DryRun               bool
	Location             *time.Location
	SetBirthTime         bool
	BackupCSVDir         string
	LogWriter            io.Writer
	AutofillExcludeRegex string

	FixedTime  time.Time
	StartTime  time.Time
	Step       time.Duration
	CSVPath    string
	CSVOutPath string
}

type FileEntry struct {
	Path       string
	Base       string
	Ext        string
	ModTime    time.Time
	TargetTime *time.Time
	Skipped    bool
	SkipCause  string
}

type Change struct {
	Path      string
	Before    time.Time
	After     time.Time
	Skipped   bool
	SkipCause string
}
