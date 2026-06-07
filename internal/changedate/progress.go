package changedate

import (
	"fmt"
	"io"
)

type progressReporter struct {
	w         io.Writer
	label     string
	total     int
	stepEvery int
	nextAt    int
}

func newProgressReporter(w io.Writer, label string, total int) *progressReporter {
	if w == nil || total <= 0 {
		return nil
	}
	stepEvery := total / 20
	if stepEvery < 1 {
		stepEvery = 1
	}
	return &progressReporter{
		w:         w,
		label:     label,
		total:     total,
		stepEvery: stepEvery,
		nextAt:    1,
	}
}

func (p *progressReporter) Start(detail string) {
	if p == nil {
		return
	}
	if detail == "" {
		fmt.Fprintf(p.w, "%s: 0/%d\n", p.label, p.total)
		return
	}
	fmt.Fprintf(p.w, "%s: 0/%d %s\n", p.label, p.total, detail)
}

func (p *progressReporter) Step(current int, detail string) {
	if p == nil || current < p.nextAt {
		return
	}
	if detail == "" {
		fmt.Fprintf(p.w, "%s: %d/%d\n", p.label, current, p.total)
	} else {
		fmt.Fprintf(p.w, "%s: %d/%d %s\n", p.label, current, p.total, detail)
	}
	p.nextAt = current + p.stepEvery
}

func (p *progressReporter) Done(detail string) {
	if p == nil {
		return
	}
	if detail == "" {
		fmt.Fprintf(p.w, "%s: %d/%d done\n", p.label, p.total, p.total)
		return
	}
	fmt.Fprintf(p.w, "%s: %d/%d %s\n", p.label, p.total, p.total, detail)
}
