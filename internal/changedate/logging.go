package changedate

import (
	"io"
	"os"
)

func logWriter(opts Options) io.Writer {
	if opts.LogWriter != nil {
		return opts.LogWriter
	}
	return os.Stderr
}
