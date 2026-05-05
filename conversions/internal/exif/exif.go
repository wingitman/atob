// Package exif extracts EXIF metadata from JPEG and TIFF image data.
//
// It implements a minimal subset of the EXIF 2.3 / TIFF 6.0 specification,
// sufficient to read the fields that the binary image inspector exposes.
// It handles both little-endian and big-endian TIFF byte orders, and locates
// the EXIF IFD embedded in JPEG APP1 markers.
package exif

import (
	"encoding/binary"
	"fmt"
	"math"
)

// TIFF tag IDs we extract.
const (
	tagMake             uint16 = 0x010f
	tagModel            uint16 = 0x0110
	tagOrientation      uint16 = 0x0112
	tagSoftware         uint16 = 0x0131
	tagDateTime         uint16 = 0x0132
	tagExifIFD          uint16 = 0x8769
	tagGPSIFD           uint16 = 0x8825
	tagDateTimeOriginal uint16 = 0x9003
	tagExposureTime     uint16 = 0x829a
	tagFNumber          uint16 = 0x829d
	tagISOSpeedRatings  uint16 = 0x8827
	tagFlash            uint16 = 0x9209
	tagFocalLength      uint16 = 0x920a
	tagPixelXDimension  uint16 = 0xa002
	tagPixelYDimension  uint16 = 0xa003

	tagGPSLatitudeRef  uint16 = 0x0001
	tagGPSLatitude     uint16 = 0x0002
	tagGPSLongitudeRef uint16 = 0x0003
	tagGPSLongitude    uint16 = 0x0004
	tagGPSAltitudeRef  uint16 = 0x0005
	tagGPSAltitude     uint16 = 0x0006
)

// TIFF field type codes.
const (
	typeASCII     uint16 = 2
	typeShort     uint16 = 3
	typeLong      uint16 = 4
	typeRational  uint16 = 5
	typeByte      uint16 = 1
	typeSByte     uint16 = 6
	typeUndefined uint16 = 7
	typeSShort    uint16 = 8
	typeSLong     uint16 = 9
	typeSRational uint16 = 10
	typeFloat     uint16 = 11
	typeDouble    uint16 = 12
)

var mainTags = map[uint16]string{
	tagMake:             "Make",
	tagModel:            "Model",
	tagSoftware:         "Software",
	tagDateTime:         "DateTime",
	tagDateTimeOriginal: "DateTimeOriginal",
	tagOrientation:      "Orientation",
	tagPixelXDimension:  "PixelXDimension",
	tagPixelYDimension:  "PixelYDimension",
	tagFocalLength:      "FocalLength",
	tagFNumber:          "FNumber",
	tagISOSpeedRatings:  "ISOSpeedRatings",
	tagExposureTime:     "ExposureTime",
	tagFlash:            "Flash",
}

var gpsTags = map[uint16]string{
	tagGPSLatitudeRef:  "GPSLatitudeRef",
	tagGPSLatitude:     "GPSLatitude",
	tagGPSLongitudeRef: "GPSLongitudeRef",
	tagGPSLongitude:    "GPSLongitude",
	tagGPSAltitudeRef:  "GPSAltitudeRef",
	tagGPSAltitude:     "GPSAltitude",
}

// Data holds extracted EXIF fields.
type Data struct {
	Fields map[string]any
}

// Decode parses EXIF metadata from b (JPEG or raw TIFF bytes).
// Returns nil if no EXIF data is found or the input is not a supported format.
func Decode(b []byte) (*Data, error) {
	if len(b) < 4 {
		return nil, fmt.Errorf("too short")
	}

	var tiffData []byte

	if b[0] == 0xff && b[1] == 0xd8 { // JPEG SOI
		tiffData = findJPEGExif(b)
		if tiffData == nil {
			return nil, fmt.Errorf("no EXIF APP1 marker in JPEG")
		}
	} else if (b[0] == 'I' && b[1] == 'I') || (b[0] == 'M' && b[1] == 'M') {
		tiffData = b
	} else {
		return nil, fmt.Errorf("unsupported format")
	}

	return parseTIFF(tiffData)
}

// findJPEGExif scans JPEG APP1 segments for "Exif\x00\x00" and returns
// the TIFF payload that follows.
func findJPEGExif(data []byte) []byte {
	i := 2 // skip SOI FF D8
	for i+4 <= len(data) {
		if data[i] != 0xff {
			break
		}
		marker := data[i+1]
		segLen := int(data[i+2])<<8 | int(data[i+3])
		if segLen < 2 {
			break
		}
		segEnd := i + 2 + segLen
		if segEnd > len(data) {
			break
		}
		if marker == 0xe1 { // APP1
			payload := data[i+4 : segEnd]
			if len(payload) >= 6 &&
				payload[0] == 'E' && payload[1] == 'x' && payload[2] == 'i' &&
				payload[3] == 'f' && payload[4] == 0 && payload[5] == 0 {
				return payload[6:]
			}
		}
		i = segEnd
	}
	return nil
}

// parseTIFF reads a TIFF-format byte slice and populates a Data struct.
func parseTIFF(data []byte) (*Data, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("TIFF too short")
	}

	var bo binary.ByteOrder
	switch {
	case data[0] == 'I' && data[1] == 'I':
		bo = binary.LittleEndian
	case data[0] == 'M' && data[1] == 'M':
		bo = binary.BigEndian
	default:
		return nil, fmt.Errorf("bad TIFF byte order")
	}

	if bo.Uint16(data[2:4]) != 42 {
		return nil, fmt.Errorf("bad TIFF magic")
	}

	ifd0 := int(bo.Uint32(data[4:8]))
	d := &Data{Fields: make(map[string]any)}

	exifOff, gpsOff := readIFD(data, ifd0, bo, mainTags, d.Fields)

	if exifOff > 0 {
		readIFD(data, exifOff, bo, mainTags, d.Fields)
	}

	if gpsOff > 0 {
		gpsFields := make(map[string]any)
		readIFD(data, gpsOff, bo, gpsTags, gpsFields)
		mergeGPS(gpsFields, d.Fields)
	}

	if len(d.Fields) == 0 {
		return nil, fmt.Errorf("no EXIF fields found")
	}
	return d, nil
}

// readIFD reads a TIFF IFD at offset, stores recognised fields into m
// (using the provided tagNames map), and returns EXIF and GPS sub-IFD offsets.
func readIFD(data []byte, offset int, bo binary.ByteOrder, tagNames map[uint16]string, m map[string]any) (exifOff, gpsOff int) {
	if offset+2 > len(data) {
		return
	}
	count := int(bo.Uint16(data[offset : offset+2]))
	pos := offset + 2

	for i := 0; i < count; i++ {
		if pos+12 > len(data) {
			break
		}
		entry := data[pos : pos+12]
		pos += 12

		tag := bo.Uint16(entry[0:2])
		typ := bo.Uint16(entry[2:4])
		cnt := int(bo.Uint32(entry[4:8]))
		rawVal := entry[8:12]

		switch tag {
		case tagExifIFD:
			exifOff = int(bo.Uint32(rawVal))
		case tagGPSIFD:
			gpsOff = int(bo.Uint32(rawVal))
		default:
			if name, ok := tagNames[tag]; ok {
				if v := readValue(data, typ, cnt, rawVal, bo); v != nil {
					m[name] = v
				}
			}
		}
	}
	return
}

// readValue decodes an IFD field value into a Go type.
func readValue(data []byte, typ uint16, count int, raw []byte, bo binary.ByteOrder) any {
	sz := typeSize(typ) * count
	var buf []byte
	if sz <= 4 {
		buf = raw[:sz]
	} else {
		off := int(bo.Uint32(raw))
		if off+sz > len(data) || off < 0 {
			return nil
		}
		buf = data[off : off+sz]
	}

	switch typ {
	case typeASCII:
		s := string(buf)
		for i, c := range s {
			if c == 0 {
				return s[:i]
			}
		}
		return s

	case typeShort:
		if count == 1 && len(buf) >= 2 {
			return int(bo.Uint16(buf[0:2]))
		}
		vals := make([]int, count)
		for i := 0; i < count && (i+1)*2 <= len(buf); i++ {
			vals[i] = int(bo.Uint16(buf[i*2 : i*2+2]))
		}
		return vals

	case typeLong:
		if count == 1 && len(buf) >= 4 {
			return int(bo.Uint32(buf[0:4]))
		}
		vals := make([]int, count)
		for i := 0; i < count && (i+1)*4 <= len(buf); i++ {
			vals[i] = int(bo.Uint32(buf[i*4 : i*4+4]))
		}
		return vals

	case typeRational:
		if count == 1 && len(buf) >= 8 {
			num := bo.Uint32(buf[0:4])
			den := bo.Uint32(buf[4:8])
			if den == 0 {
				return nil
			}
			return fmt.Sprintf("%d/%d", num, den)
		}
		vals := make([]string, count)
		for i := 0; i < count && (i+1)*8 <= len(buf); i++ {
			num := bo.Uint32(buf[i*8 : i*8+4])
			den := bo.Uint32(buf[i*8+4 : i*8+8])
			if den == 0 {
				vals[i] = "0/1"
			} else {
				vals[i] = fmt.Sprintf("%d/%d", num, den)
			}
		}
		return vals

	case typeSRational:
		if count == 1 && len(buf) >= 8 {
			num := int32(bo.Uint32(buf[0:4]))
			den := int32(bo.Uint32(buf[4:8]))
			if den == 0 {
				return nil
			}
			return fmt.Sprintf("%d/%d", num, den)
		}

	case typeFloat:
		if count == 1 && len(buf) >= 4 {
			return math.Float32frombits(bo.Uint32(buf[0:4]))
		}

	case typeDouble:
		if count == 1 && len(buf) >= 8 {
			return math.Float64frombits(bo.Uint64(buf[0:8]))
		}

	case typeByte:
		if count == 1 && len(buf) >= 1 {
			return int(buf[0])
		}
		return append([]byte(nil), buf...)

	case typeUndefined:
		return append([]byte(nil), buf...)
	}
	return nil
}

// typeSize returns the byte width of a single element of the given TIFF type.
func typeSize(typ uint16) int {
	switch typ {
	case typeByte, typeASCII, typeSByte, typeUndefined:
		return 1
	case typeShort, typeSShort:
		return 2
	case typeLong, typeSLong, typeFloat:
		return 4
	case typeRational, typeSRational, typeDouble:
		return 8
	}
	return 1
}

// mergeGPS converts GPS sub-IFD fields into decimal lat/lon/alt and adds them
// to the main fields map.
func mergeGPS(gps, dst map[string]any) {
	if latVals, ok := gps["GPSLatitude"].([]string); ok && len(latVals) == 3 {
		lat := dmsToDecimal(latVals)
		if ref, _ := gps["GPSLatitudeRef"].(string); ref == "S" {
			lat = -lat
		}
		dst["GPSLatitude"] = lat
	}
	if lonVals, ok := gps["GPSLongitude"].([]string); ok && len(lonVals) == 3 {
		lon := dmsToDecimal(lonVals)
		if ref, _ := gps["GPSLongitudeRef"].(string); ref == "W" {
			lon = -lon
		}
		dst["GPSLongitude"] = lon
	}
	if alt, ok := gps["GPSAltitude"]; ok {
		dst["GPSAltitude"] = alt
	}
}

// dmsToDecimal converts a degrees/minutes/seconds rational triplet (as "n/d" strings)
// to a decimal degree value.
func dmsToDecimal(dms []string) float64 {
	deg := parseRat(dms[0])
	min := parseRat(dms[1])
	sec := parseRat(dms[2])
	return deg + min/60 + sec/3600
}

// parseRat parses a "numerator/denominator" rational string to float64.
func parseRat(s string) float64 {
	var num, den float64
	fmt.Sscanf(s, "%f/%f", &num, &den)
	if den == 0 {
		return 0
	}
	return num / den
}
