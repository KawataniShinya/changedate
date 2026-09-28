package changedate

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func setBirthTimesSMB(opts Options, changes []Change) error {
	var commands strings.Builder
	count := 0
	for _, change := range changes {
		if change.Skipped {
			continue
		}
		remotePath, err := smbRemotePath(opts.SMBRoot, change.Path)
		if err != nil {
			return err
		}
		fmt.Fprintf(&commands, "utimes \"%s\" %s -1 -1 -1\n", remotePath,
			change.After.In(opts.Location).Format("2006:01:02-15:04:05"))
		count++
	}
	if count == 0 {
		return nil
	}
	commands.WriteString("quit\n")
	cmd := execCommand("smbclient", opts.SMBShare, "-A", opts.SMBAuthFile)
	cmd.Stdin = strings.NewReader(commands.String())
	cmd.Env = append(os.Environ(), "TZ="+opts.Location.String())
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("smbclient creation time update failed: %w: %s", err, output.String())
	}
	if strings.Contains(output.String(), "NT_STATUS_") || strings.Contains(strings.ToLower(output.String()), "error") {
		return fmt.Errorf("smbclient creation time update reported an error: %s", output.String())
	}
	fmt.Fprintf(logWriter(opts), "SMB creation time updated for %d files\n", count)
	return nil
}

func smbRemotePath(root, path string) (string, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(rootAbs, pathAbs)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("file %s is outside --smb-root %s", path, root)
	}
	if strings.ContainsAny(rel, "\r\n\";") || strings.ContainsRune(rel, '\\') {
		return "", fmt.Errorf("file %s contains characters unsupported by smbclient commands", path)
	}
	return strings.ReplaceAll(rel, string(filepath.Separator), "\\"), nil
}
