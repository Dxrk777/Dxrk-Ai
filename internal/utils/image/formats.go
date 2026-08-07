// Package image provides image format detection and supported format constants.
package image

import (
	"bytes"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"strings"

	"github.com/Dxrk777/Dxrk/internal/strconst"
)

// Format represents an image format type.
type Format int

const (
	// Unknown format
	Unknown Format = iota
	// JPEG format
	JPEG
	// PNG format
	PNG
	// GIF format
	GIF
	// WebP format (limited support via stdlib)
	WebP
)

// String returns the format name.
func (f Format) String() string {
	switch f {
	case JPEG:
		return "jpeg"
	case PNG:
		return "png"
	case GIF:
		return "gif"
	case WebP:
		return "webp"
	default:
		return strconst.StrUnknown
	}
}

// MIME returns the MIME type for the format.
func (f Format) MIME() string {
	switch f {
	case JPEG:
		return "image/jpeg"
	case PNG:
		return "image/png"
	case GIF:
		return "image/gif"
	case WebP:
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

// Extension returns the file extension for the format.
func (f Format) Extension() string {
	switch f {
	case JPEG:
		return ".jpg"
	case PNG:
		return ".png"
	case GIF:
		return ".gif"
	case WebP:
		return ".webp"
	default:
		return ".bin"
	}
}

// SupportedFormats lists all supported formats.
var SupportedFormats = []Format{JPEG, PNG, GIF, WebP}

// SupportedMIMEs maps MIME types to formats.
var SupportedMIMEs = map[string]Format{
	"image/jpeg": JPEG,
	"image/jpg":  JPEG,
	"image/png":  PNG,
	"image/gif":  GIF,
	"image/webp": WebP,
}

// SupportedExtensions maps file extensions to formats.
var SupportedExtensions = map[string]Format{
	".jpg":  JPEG,
	".jpeg": JPEG,
	".png":  PNG,
	".gif":  GIF,
	".webp": WebP,
}

// DetectFormat detects the image format from raw bytes.
func DetectFormat(data []byte) Format {
	if len(data) < 12 {
		return Unknown
	}

	// JPEG: FF D8 FF
	if bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}) {
		return JPEG
	}

	// PNG: 89 50 4E 47 0D 0A 1A 0A
	if bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
		return PNG
	}

	// GIF: GIF87a or GIF89a
	if bytes.HasPrefix(data, []byte("GIF87a")) || bytes.HasPrefix(data, []byte("GIF89a")) {
		return GIF
	}

	// WebP: RIFF....WEBP
	if len(data) >= 12 && bytes.HasPrefix(data, []byte("RIFF")) && bytes.HasPrefix(data[8:12], []byte("WEBP")) {
		return WebP
	}

	return Unknown
}

// DetectFormatFromReader detects format from an io.Reader without consuming it entirely.
func DetectFormatFromReader(r interface {
	Read([]byte) (int, error)
	Seek(int64, int) (int64, error)
}) (Format, error) {
	header := make([]byte, 12)
	n, err := r.Read(header)
	if err != nil {
		return Unknown, err
	}
	// Seek back
	_, err = r.Seek(-int64(n), 1)
	if err != nil {
		return Unknown, err
	}
	return DetectFormat(header[:n]), nil
}

// DecodeFormat decodes an image from bytes, detecting format automatically.
func DecodeFormat(data []byte) (image.Image, Format, error) {
	fmt := DetectFormat(data)
	if fmt == Unknown {
		return nil, Unknown, ErrUnsupportedFormat
	}

	var img image.Image
	var err error

	switch fmt {
	case JPEG:
		img, err = jpeg.Decode(bytes.NewReader(data))
	case PNG:
		img, err = png.Decode(bytes.NewReader(data))
	case GIF:
		img, err = gif.Decode(bytes.NewReader(data))
	default:
		return nil, Unknown, ErrUnsupportedFormat
	}

	return img, fmt, err
}

// EncodeFormat encodes an image to bytes in the specified format.
func EncodeFormat(img image.Image, fmt Format, quality int) ([]byte, error) {
	var buf bytes.Buffer
	var err error

	switch fmt {
	case JPEG:
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality})
	case PNG:
		err = png.Encode(&buf, img)
	case GIF:
		err = gif.Encode(&buf, img, nil)
	default:
		return nil, ErrUnsupportedFormat
	}

	return buf.Bytes(), err
}

// FormatFromExtension returns the format for a file extension.
func FormatFromExtension(ext string) Format {
	ext = strings.ToLower(ext)
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	if f, ok := SupportedExtensions[ext]; ok {
		return f
	}
	return Unknown
}

// FormatFromMIME returns the format for a MIME type.
func FormatFromMIME(mime string) Format {
	mime = strings.ToLower(mime)
	if f, ok := SupportedMIMEs[mime]; ok {
		return f
	}
	return Unknown
}

// IsSupportedFormat checks if a format is supported.
func IsSupportedFormat(fmt Format) bool {
	for _, f := range SupportedFormats {
		if f == fmt {
			return true
		}
	}
	return false
}

// ErrUnsupportedFormat is returned when a format is not supported.
var ErrUnsupportedFormat = &FormatError{"unsupported format"}

// FormatError represents a format-related error.
type FormatError struct {
	msg string
}

func (e *FormatError) Error() string {
	return "image: " + e.msg
}
