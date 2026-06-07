package changedate

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"time"
)

func ReadExifTime(path string) (time.Time, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}, false
	}
	return readJPEGExifTime(data)
}

func readJPEGExifTime(data []byte) (time.Time, bool) {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return time.Time{}, false
	}

	offset := 2
	for offset+4 <= len(data) {
		if data[offset] != 0xFF {
			return time.Time{}, false
		}
		marker := data[offset+1]
		offset += 2
		if marker == 0xD9 || marker == 0xDA {
			break
		}
		if offset+2 > len(data) {
			return time.Time{}, false
		}
		segmentLen := int(binary.BigEndian.Uint16(data[offset : offset+2]))
		if segmentLen < 2 || offset+segmentLen > len(data) {
			return time.Time{}, false
		}
		payload := data[offset+2 : offset+segmentLen]
		if marker == 0xE1 && bytes.HasPrefix(payload, []byte("Exif\x00\x00")) {
			ts, ok := parseExifPayload(payload[6:])
			return ts, ok
		}
		offset += segmentLen
	}
	return time.Time{}, false
}

func parseExifPayload(payload []byte) (time.Time, bool) {
	if len(payload) < 8 {
		return time.Time{}, false
	}

	var order binary.ByteOrder
	switch string(payload[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return time.Time{}, false
	}

	if order.Uint16(payload[2:4]) != 0x002A {
		return time.Time{}, false
	}

	ifd0Offset := int(order.Uint32(payload[4:8]))
	datetime, ok := readExifIFD(payload, order, ifd0Offset)
	if ok {
		return datetime, true
	}
	return time.Time{}, false
}

func readExifIFD(payload []byte, order binary.ByteOrder, offset int) (time.Time, bool) {
	if offset < 0 || offset+2 > len(payload) {
		return time.Time{}, false
	}

	count := int(order.Uint16(payload[offset : offset+2]))
	entryBase := offset + 2
	for i := 0; i < count; i++ {
		entry := entryBase + i*12
		if entry+12 > len(payload) {
			return time.Time{}, false
		}
		tag := order.Uint16(payload[entry : entry+2])
		valueType := order.Uint16(payload[entry+2 : entry+4])
		valueCount := order.Uint32(payload[entry+4 : entry+8])
		valueOrOffset := payload[entry+8 : entry+12]

		switch tag {
		case 0x8769:
			exifOffset := int(order.Uint32(valueOrOffset))
			return readExifSubIFD(payload, order, exifOffset)
		case 0x0132:
			if valueType == 2 {
				if s, ok := readExifString(payload, order, valueCount, valueOrOffset); ok {
					if ts, ok := parseExifTimeString(s); ok {
						return ts, true
					}
				}
			}
		}
	}
	return time.Time{}, false
}

func readExifSubIFD(payload []byte, order binary.ByteOrder, offset int) (time.Time, bool) {
	if offset < 0 || offset+2 > len(payload) {
		return time.Time{}, false
	}

	count := int(order.Uint16(payload[offset : offset+2]))
	entryBase := offset + 2
	for i := 0; i < count; i++ {
		entry := entryBase + i*12
		if entry+12 > len(payload) {
			return time.Time{}, false
		}
		tag := order.Uint16(payload[entry : entry+2])
		valueType := order.Uint16(payload[entry+2 : entry+4])
		valueCount := order.Uint32(payload[entry+4 : entry+8])
		valueOrOffset := payload[entry+8 : entry+12]

		if tag == 0x9003 && valueType == 2 {
			if s, ok := readExifString(payload, order, valueCount, valueOrOffset); ok {
				if ts, ok := parseExifTimeString(s); ok {
					return ts, true
				}
			}
		}
	}
	return time.Time{}, false
}

func readExifString(payload []byte, order binary.ByteOrder, valueCount uint32, valueOrOffset []byte) (string, bool) {
	if valueCount == 0 {
		return "", false
	}
	if valueCount <= 4 {
		raw := valueOrOffset[:valueCount]
		return trimExifString(string(raw)), true
	}
	offset := int(order.Uint32(valueOrOffset))
	if offset < 0 || offset+int(valueCount) > len(payload) {
		return "", false
	}
	return trimExifString(string(payload[offset : offset+int(valueCount)])), true
}

func trimExifString(s string) string {
	return strings.TrimRight(s, "\x00 ")
}

func parseExifTimeString(value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}
	layouts := []string{
		"2006:01:02 15:04:05",
		time.DateTime,
	}
	for _, layout := range layouts {
		if ts, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return ts, true
		}
	}
	return time.Time{}, false
}

func init() {
	// Keep the compiler from removing helper paths when exif support is unused.
	_ = fmt.Sprintf
}
