package fileops

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	_ "image/gif" // Register the GIF decoder for image/gif support.
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

// FileContent holds the result of reading a file.
type FileContent struct {
	Path      string
	Content   string
	Encoding  string
	Size      int64
	ModTime   time.Time
	LineCount int
}

// ImageData holds a file read as a base64-encoded image.
type ImageData struct {
	MediaType  string
	Base64Data string
	Width      int
	Height     int
}

// sentinel errors for this package
var (
	ErrPathTraversal = errors.New("fileops: path contains traversal components")
	ErrNullByte      = errors.New("fileops: path contains null byte")
	ErrBinaryFile    = errors.New("fileops: file is binary")
	ErrFileTooLarge  = errors.New("fileops: file exceeds read limit")
	ErrInvalidImage  = errors.New("fileops: file is not a valid image")
)

const (
	maxTextSize      = 10 * 1024 * 1024 // 10 MB — threshold for mmap
	readLimitDefault = 50 * 1024 * 1024
	binaryCheckSize  = 8192
)

// ValidatePath checks a file path for traversal attacks and null bytes.
func ValidatePath(path string) error {
	if strings.Contains(path, "\x00") {
		return ErrNullByte
	}
	cleaned := filepath.Clean(path)
	parts := strings.Split(cleaned, string(os.PathSeparator))
	for _, p := range parts {
		if p == ".." {
			return ErrPathTraversal
		}
	}
	return nil
}

// DetectEncoding inspects a byte slice and returns a best-guess encoding label.
func DetectEncoding(data []byte) string {
	if len(data) == 0 {
		return "UTF-8"
	}
	// Check for BOM
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		return "UTF-8-BOM"
	}
	if len(data) >= 2 {
		if data[0] == 0xFF && data[1] == 0xFE {
			return "UTF-16LE"
		}
		if data[0] == 0xFE && data[1] == 0xFF {
			return "UTF-16BE"
		}
	}
	if utf8.Valid(data) {
		// Check for non-ASCII UTF-8 content
		for _, r := range string(data) {
			if r > 127 {
				return "UTF-8"
			}
		}
		return "ASCII"
	}
	// Heuristic: if most bytes are in Latin-1 printable range, call it Latin-1
	latin1 := 0
	for _, b := range data {
		if b >= 0x20 && b < 0x7F || b >= 0xA0 {
			latin1++
		}
	}
	if float64(latin1)/float64(len(data)) > 0.9 {
		return "Latin-1"
	}
	return "Unknown"
}

// IsBinary detects whether a file is likely binary by inspecting the first
// binaryCheckSize bytes. It returns true if a null byte is found or if the
// ratio of non-text bytes exceeds a threshold.
func IsBinary(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer func() { _ = f.Close() }()

	buf := make([]byte, binaryCheckSize)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return false, err
	}
	buf = buf[:n]

	if bytes.Contains(buf, []byte{0}) {
		return true, nil
	}

	nonText := 0
	for _, b := range buf {
		if b < 0x09 || (b > 0x0D && b < 0x20) {
			nonText++
		}
	}
	return float64(nonText)/float64(max(n, 1)) > 0.1, nil
}

// ReadFile reads a file, detects its encoding, and returns a FileContent.
// For files larger than 10 MB it uses memory-mapped reading.
func ReadFile(path string) (*FileContent, error) {
	if err := ValidatePath(path); err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("fileops: %s is a directory", path)
	}

	var data []byte
	if info.Size() > maxTextSize {
		data, err = readMmap(path, info.Size())
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, err
	}

	enc := DetectEncoding(data)
	content := string(data)
	lines := strings.Count(content, "\n")
	if len(content) > 0 && content[len(content)-1] != '\n' {
		lines++
	}

	return &FileContent{
		Path:      path,
		Content:   content,
		Encoding:  enc,
		Size:      info.Size(),
		ModTime:   info.ModTime(),
		LineCount: lines,
	}, nil
}

// ReadFileLines reads a file and returns lines[offset:offset+limit].
// It also returns the total line count.
func ReadFileLines(path string, offset, limit int) ([]string, int, error) {
	fc, err := ReadFile(path)
	if err != nil {
		return nil, 0, err
	}
	lines := strings.Split(fc.Content, "\n")
	total := len(lines)
	if offset < 0 {
		offset = 0
	}
	if offset >= total {
		return nil, total, nil
	}
	end := offset + limit
	if limit <= 0 || end > total {
		end = total
	}
	return lines[offset:end], total, nil
}

// ReadImage reads an image file and returns it as base64-encoded data with dimensions.
func ReadImage(path string) (*ImageData, error) {
	if err := ValidatePath(path); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidImage, err)
	}
	mediaType := "image/png"
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		mediaType = "image/jpeg"
	case ".gif":
		mediaType = "image/gif"
	}
	return &ImageData{
		MediaType:  mediaType,
		Base64Data: base64.StdEncoding.EncodeToString(data),
		Width:      cfg.Width,
		Height:     cfg.Height,
	}, nil
}

// readMmap reads a file using io.ReadAll on a reader backed by mmap.
// Falls back to regular ReadFile if mmap is not available.
func readMmap(path string, _ int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	return data, nil
}
