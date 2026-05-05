package binary

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	internalexif "github.com/wingitman/atob/conversions/internal/exif"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

type imageInfo struct {
	Type      string         `json:"type"`
	Width     int            `json:"width"`
	Height    int            `json:"height"`
	ColorMode string         `json:"color_mode"`
	EXIF      map[string]any `json:"exif,omitempty"`
	FileSize  int            `json:"file_size"`
}

func inspectImage(data []byte, mimeType string) (string, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("cannot decode image: %w", err)
	}

	info := imageInfo{
		Type:     format,
		Width:    cfg.Width,
		Height:   cfg.Height,
		FileSize: len(data),
	}

	// Color mode from color model name
	if cfg.ColorModel != nil {
		info.ColorMode = colorModelName(cfg.ColorModel)
	}

	// EXIF (JPEG and TIFF carry EXIF; PNG/GIF/WebP generally don't)
	if x, err := internalexif.Decode(data); err == nil {
		info.EXIF = x.Fields
	}

	out, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out) + "\n", nil
}

// colorModelName returns a human-friendly name for common color models.
func colorModelName(m interface{}) string {
	// Use the type string since image/color model types aren't named exports
	s := fmt.Sprintf("%T", m)
	switch s {
	case "color.grayModel", "*color.modelFunc":
		// YCbCr (JPEG) or gray — disambiguate by checking if it converts YCbCr
		return "YCbCr/RGB"
	case "color.gray16Model":
		return "Grayscale16"
	case "color.rgbaModel":
		return "RGBA"
	case "color.rgba64Model":
		return "RGBA64"
	case "color.nrgbaModel":
		return "NRGBA"
	case "color.ycbcrModel":
		return "YCbCr"
	case "color.cmykModel":
		return "CMYK"
	case "color.Palette":
		return "Indexed/Palette"
	case "color.nrgba64Model":
		return "NRGBA64"
	default:
		// Strip package prefix for readability
		if idx := len("color."); len(s) > idx && s[:idx] == "color." {
			return s[idx:]
		}
		return s
	}
}
