package changedate

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSetBirthTimeFormatsUSDate(t *testing.T) {
	origGOOS := currentGOOS
	origRunSetFile := runSetFile
	t.Cleanup(func() {
		currentGOOS = origGOOS
		runSetFile = origRunSetFile
	})

	currentGOOS = "darwin"
	var gotPath, gotFormatted string
	runSetFile = func(path, formatted string) error {
		gotPath = path
		gotFormatted = formatted
		return nil
	}

	path := filepath.Join(t.TempDir(), "photo.jpg")
	ts := time.Date(2024, 1, 15, 23, 50, 11, 0, time.Local)
	if err := setBirthTime(path, ts); err != nil {
		t.Fatalf("setBirthTime: %v", err)
	}

	if gotPath != path {
		t.Fatalf("path = %q, want %q", gotPath, path)
	}
	if gotFormatted != "01/15/2024 23:50:11" {
		t.Fatalf("formatted = %q, want %q", gotFormatted, "01/15/2024 23:50:11")
	}
}

func TestLinuxBirthTimeRequiresSMBConfiguration(t *testing.T) {
	origGOOS := currentGOOS
	t.Cleanup(func() { currentGOOS = origGOOS })
	currentGOOS = "linux"
	err := validateBirthTimeOptions(Options{SetBirthTime: true})
	if err == nil || !strings.Contains(err.Error(), "--smb-share") {
		t.Fatalf("expected SMB configuration error, got %v", err)
	}
}

func TestEnsureSetFileAvailableReturnsHelpfulError(t *testing.T) {
	origGOOS := currentGOOS
	origLookPath := setFileLookPath
	t.Cleanup(func() {
		currentGOOS = origGOOS
		setFileLookPath = origLookPath
	})

	currentGOOS = "darwin"
	setFileLookPath = func(string) (string, error) {
		return "", errors.New("not found")
	}

	err := ensureSetFileAvailable()
	if err == nil {
		t.Fatal("expected error")
	}
	if got := err.Error(); !containsAll(got, []string{"SetFile not found", "Xcode Command Line Tools"}) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunWithBirthTimeCallsSetFileOnMac(t *testing.T) {
	origGOOS := currentGOOS
	origRunSetFile := runSetFile
	origLookPath := setFileLookPath
	t.Cleanup(func() {
		currentGOOS = origGOOS
		runSetFile = origRunSetFile
		setFileLookPath = origLookPath
	})

	currentGOOS = "darwin"
	setFileLookPath = func(string) (string, error) {
		return "/usr/bin/SetFile", nil
	}

	called := 0
	var gotPath string
	runSetFile = func(path, formatted string) error {
		called++
		gotPath = path
		return nil
	}

	root := t.TempDir()
	path := filepath.Join(root, "photo.jpg")
	mustWriteFile(t, path)
	before := time.Date(2024, 1, 1, 8, 0, 0, 0, time.Local)
	if err := os.Chtimes(path, before, before); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	opts := Options{
		Dir:          root,
		Mode:         ModeAll,
		FixedTime:    time.Date(2024, 1, 1, 10, 0, 0, 0, time.Local),
		SetBirthTime: true,
	}
	if err := Run(opts, io.Discard); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if called != 1 {
		t.Fatalf("runSetFile called %d times, want 1", called)
	}
	if gotPath != path {
		t.Fatalf("runSetFile path = %q, want %q", gotPath, path)
	}
}

func TestRunWithBirthTimeUpdatesBirthTimeOnMac(t *testing.T) {
	if currentGOOS != "darwin" {
		t.Skip("birth time update is macOS-specific")
	}
	if _, err := setFileLookPath("SetFile"); err != nil {
		t.Skip("SetFile is not installed")
	}

	root := t.TempDir()
	path := filepath.Join(root, "photo.jpg")
	mustWriteFile(t, path)

	beforeBirth, err := statBirthTimeUnix(path)
	if err != nil {
		t.Fatalf("stat before: %v", err)
	}

	opts := Options{
		Dir:          root,
		Mode:         ModeAll,
		FixedTime:    time.Date(2024, 1, 1, 10, 0, 0, 0, time.Local),
		SetBirthTime: true,
	}
	if err := Run(opts, io.Discard); err != nil {
		t.Fatalf("Run: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !info.ModTime().Equal(opts.FixedTime) {
		t.Fatalf("mtime = %v, want %v", info.ModTime(), opts.FixedTime)
	}

	afterBirth, err := statBirthTimeUnix(path)
	if err != nil {
		t.Fatalf("stat after: %v", err)
	}
	if afterBirth == beforeBirth {
		t.Fatalf("birth time did not change: before=%d after=%d", beforeBirth, afterBirth)
	}
}

func containsAll(s string, subs []string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

func statBirthTimeUnix(path string) (int64, error) {
	out, err := exec.Command("stat", "-f", "%B", path).Output()
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
}
