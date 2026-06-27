//go:build windows

package changedate

import (
	"fmt"
	"syscall"
	"time"
)

func setBirthTimeWindows(path string, t time.Time) error {
	ft := syscall.NsecToFiletime(t.UTC().UnixNano())

	utf16Path, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return fmt.Errorf("convert path to UTF-16: %w", err)
	}

	handle, err := syscall.CreateFile(
		utf16Path,
		syscall.FILE_WRITE_ATTRIBUTES,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return fmt.Errorf("open file for creation time update: %w", err)
	}
	defer syscall.CloseHandle(handle)

	if err := syscall.SetFileTime(handle, &ft, nil, nil); err != nil {
		return fmt.Errorf("set file creation time: %w", err)
	}
	return nil
}
