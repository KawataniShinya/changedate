package changedate

import (
	"fmt"
	"time"
)

var inputLayouts = []string{
	"2006-01-02 15:04:05",
	"2006-01-02 15:04:05.000",
	"2006-01-02 15:04:05.000000",
	"2006-01-02 15:04:05.000000000",
	"2006-01-02 15:04",
	"2006-01-02",
	"20060102_150405",
	"20060102_150405.000",
	"20060102_150405.000000",
	"20060102_150405.000000000",
	time.RFC3339,
	time.RFC3339Nano,
}

func ParseInputTime(value string, loc *time.Location) (time.Time, error) {
	if loc == nil {
		loc = time.Local
	}
	for _, layout := range inputLayouts {
		if ts, err := time.ParseInLocation(layout, value, loc); err == nil {
			return ts, nil
		}
		if ts, err := time.Parse(layout, value); err == nil {
			return ts, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported timestamp format %q", value)
}
