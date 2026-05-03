package binary

import (
	"archive/tar"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
)

type tarFileEntry struct {
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Mode     string `json:"mode"`
	Modified string `json:"modified"`
	Type     string `json:"type"`
}

type tarInfo struct {
	Type        string         `json:"type"`
	Compression string         `json:"compression"`
	FileCount   int            `json:"file_count"`
	TotalSize   int64          `json:"total_size_bytes"`
	Files       []tarFileEntry `json:"files"`
	FileSize    int            `json:"file_size"`
}

func inspectTAR(data []byte, compression string) (string, error) {
	var r io.Reader = bytes.NewReader(data)
	var err error

	switch compression {
	case "gzip":
		r, err = gzip.NewReader(r)
		if err != nil {
			return "", fmt.Errorf("invalid gzip stream: %w", err)
		}
	case "bzip2":
		r = bzip2.NewReader(r)
	case "xz":
		// xz not in stdlib; try to read raw tar and report
		return "", fmt.Errorf("xz-compressed tar not supported; decompress with 'xz -d' first")
	}

	info := tarInfo{
		Type:        "TAR",
		Compression: compression,
		FileSize:    len(data),
		Files:       make([]tarFileEntry, 0),
	}

	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			break // truncated or corrupt — report what we have
		}

		typeName := "file"
		switch hdr.Typeflag {
		case tar.TypeDir:
			typeName = "dir"
		case tar.TypeSymlink:
			typeName = "symlink"
		case tar.TypeLink:
			typeName = "hardlink"
		}

		info.Files = append(info.Files, tarFileEntry{
			Name:     hdr.Name,
			Size:     hdr.Size,
			Mode:     fmt.Sprintf("%04o", hdr.Mode&0777),
			Modified: hdr.ModTime.UTC().Format("2006-01-02T15:04:05Z"),
			Type:     typeName,
		})
		info.TotalSize += hdr.Size
	}
	info.FileCount = len(info.Files)

	out, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out) + "\n", nil
}
