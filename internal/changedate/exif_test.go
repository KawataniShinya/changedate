package changedate

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadExifTimeJPEG(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "photo.jpg")
	if err := os.WriteFile(path, buildMinimalExifJPEG(t, "2024:01:02 03:04:05"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	ts, ok := ReadExifTime(path)
	if !ok {
		t.Fatalf("expected exif time")
	}
	want := time.Date(2024, 1, 2, 3, 4, 5, 0, time.Local)
	if !ts.Equal(want) {
		t.Fatalf("ts = %v, want %v", ts, want)
	}
}

func buildMinimalExifJPEG(t *testing.T, datetime string) []byte {
	t.Helper()
	if len(datetime) != len("2006:01:02 15:04:05") {
		t.Fatalf("datetime length mismatch")
	}

	tiff := make([]byte, 0, 128)
	tiff = append(tiff, 'I', 'I')               // little-endian
	tiff = append(tiff, 0x2A, 0x00)             // TIFF magic
	tiff = append(tiff, 0x08, 0x00, 0x00, 0x00) // IFD0 offset
	ifd0 := []byte{
		0x01, 0x00, // 1 entry
		0x69, 0x87, // ExifIFDPointer
		0x04, 0x00, // LONG
		0x01, 0x00, 0x00, 0x00,
		0x1A, 0x00, 0x00, 0x00, // offset to exif IFD
		0x00, 0x00, 0x00, 0x00, // next IFD offset
	}
	tiff = append(tiff, ifd0...)
	exifIFDOffset := 8 + len(ifd0)
	datetimeOffset := exifIFDOffset + 18
	exifIFD := []byte{
		0x01, 0x00, // 1 entry
		0x03, 0x90, // DateTimeOriginal
		0x02, 0x00, // ASCII
		byte(len(datetime) + 1), 0x00, 0x00, 0x00,
		byte(datetimeOffset), byte(datetimeOffset >> 8), byte(datetimeOffset >> 16), byte(datetimeOffset >> 24), // offset to string
		0x00, 0x00, 0x00, 0x00,
	}
	tiff = append(tiff, exifIFD...)
	tiff = append(tiff, []byte(datetime)...)
	tiff = append(tiff, 0x00)

	app1 := make([]byte, 0, len(tiff)+10)
	app1 = append(app1, 0xFF, 0xE1)
	length := len(tiff) + 8
	app1 = append(app1, byte(length>>8), byte(length))
	app1 = append(app1, []byte("Exif\x00\x00")...)
	app1 = append(app1, tiff...)

	jpeg := []byte{0xFF, 0xD8}
	jpeg = append(jpeg, app1...)
	jpeg = append(jpeg, 0xFF, 0xD9)
	return jpeg
}
