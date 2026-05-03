package binary

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
)

type zipFileEntry struct {
	Name               string `json:"name"`
	SizeUncompressed   uint64 `json:"size_uncompressed"`
	SizeCompressed     uint64 `json:"size_compressed"`
	Modified           string `json:"modified"`
	Method             string `json:"method"`
}

type zipInfo struct {
	Type             string         `json:"type"`
	FileCount        int            `json:"file_count"`
	TotalUncompressed uint64        `json:"total_uncompressed_bytes"`
	TotalCompressed   uint64        `json:"total_compressed_bytes"`
	CompressionRatio string         `json:"compression_ratio"`
	Files            []zipFileEntry `json:"files"`
	FileSize         int            `json:"file_size"`
}

func inspectZIP(data []byte) (string, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("not a valid ZIP file: %w", err)
	}

	info := zipInfo{
		Type:     "ZIP",
		FileSize: len(data),
		Files:    make([]zipFileEntry, 0, len(r.File)),
	}

	methodName := func(m uint16) string {
		switch m {
		case zip.Store:
			return "store"
		case zip.Deflate:
			return "deflate"
		default:
			return fmt.Sprintf("method-%d", m)
		}
	}

	for _, f := range r.File {
		info.TotalUncompressed += f.UncompressedSize64
		info.TotalCompressed += f.CompressedSize64
		info.Files = append(info.Files, zipFileEntry{
			Name:             f.Name,
			SizeUncompressed: f.UncompressedSize64,
			SizeCompressed:   f.CompressedSize64,
			Modified:         f.Modified.UTC().Format("2006-01-02T15:04:05Z"),
			Method:           methodName(f.Method),
		})
	}
	info.FileCount = len(r.File)

	if info.TotalUncompressed > 0 {
		ratio := 100.0 - float64(info.TotalCompressed)/float64(info.TotalUncompressed)*100.0
		info.CompressionRatio = fmt.Sprintf("%.1f%%", ratio)
	} else {
		info.CompressionRatio = "0%"
	}

	out, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out) + "\n", nil
}
