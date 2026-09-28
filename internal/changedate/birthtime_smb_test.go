package changedate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSMBRemotePathRejectsEscapeAndCommands(t *testing.T) {
	root := t.TempDir()
	path, err := smbRemotePath(root, filepath.Join(root, "photos", "a b.JPG"))
	if err != nil || path != `photos\a b.JPG` {
		t.Fatalf("path = %q, err = %v", path, err)
	}
	for _, bad := range []string{filepath.Join(root, "..", "outside.jpg"), filepath.Join(root, "photos", `a";quit.jpg`)} {
		if _, err := smbRemotePath(root, bad); err == nil {
			t.Fatalf("accepted unsafe path %q", bad)
		}
	}
}

func TestSetBirthTimesSMBSendsOnlyCreationTime(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	outputPath := filepath.Join(root, "commands.txt")
	script := "#!/bin/sh\ncat > '" + outputPath + "'\n"
	if err := os.WriteFile(filepath.Join(bin, "smbclient"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	opts := Options{SMBShare: "//server/share", SMBRoot: root, SMBAuthFile: filepath.Join(root, "auth"), Location: loc}
	changes := []Change{{Path: filepath.Join(root, "photos", "a b.JPG"), After: time.Date(2026, 9, 27, 14, 34, 50, 0, loc)}}
	if err := setBirthTimesSMB(opts, changes); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "utimes \"photos\\a b.JPG\" 2026:09:27-14:34:50 -1 -1 -1\n") {
		t.Fatalf("wrong smbclient command: %q", data)
	}
}

func TestSetBirthTimesSMBRejectsReportedFailure(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "smbclient"), []byte("#!/bin/sh\necho NT_STATUS_ACCESS_DENIED\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	err := setBirthTimesSMB(Options{SMBShare: "//server/share", SMBRoot: root, Location: time.Local},
		[]Change{{Path: filepath.Join(root, "photo.jpg"), After: time.Now()}})
	if err == nil || !strings.Contains(err.Error(), "NT_STATUS_ACCESS_DENIED") {
		t.Fatalf("expected SMB error, got %v", err)
	}
}
