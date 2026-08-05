package image

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
)

var (
	ErrNotPDF       = errors.New("not a valid PDF file")
	ErrPDFEncrypted = errors.New("PDF is encrypted")
	ErrPageNotFound = errors.New("page not found")
)

type PDFMetadata struct {
	Title        string
	Author       string
	Subject      string
	Creator      string
	Producer     string
	CreationDate string
	ModDate      string
	Pages        int
}

func ExtractText(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	return extractTextFromReader(f)
}

func extractTextFromReader(r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}

	if len(data) < 5 || string(data[:5]) != strconst.StrPdf {
		return "", ErrNotPDF
	}

	var text strings.Builder
	inStream := false
	streamStart := 0

	for i := 0; i < len(data)-1; i++ {
		if data[i] == 's' && data[i+1] == 't' && i+5 < len(data) && string(data[i:i+6]) == "stream" {
			inStream = true
			streamStart = i + 6
			if streamStart < len(data) && data[streamStart] == '\r' {
				streamStart++
			}
			if streamStart < len(data) && data[streamStart] == '\n' {
				streamStart++
			}
		} else if inStream && data[i] == 'e' && data[i+1] == 'n' && i+7 < len(data) && string(data[i:i+8]) == "endstream" {
			streamData := data[streamStart:i]
			decoded := decodeStream(streamData)
			text.WriteString(decoded)
			inStream = false
		}
	}

	return text.String(), nil
}

func decodeStream(data []byte) string {
	var result strings.Builder
	i := 0
	for i < len(data) {
		switch {
		case data[i] == '\\' && i+1 < len(data):
			switch data[i+1] {
			case 'n':
				result.WriteByte('\n')
			case 'r':
				result.WriteByte('\r')
			case 't':
				result.WriteByte('\t')
			case '(':
				result.WriteByte('(')
			case ')':
				result.WriteByte(')')
			case '\\':
				result.WriteByte('\\')
			default:
				if i+3 < len(data) {
					if isOctal(data[i+1]) && isOctal(data[i+2]) && isOctal(data[i+3]) {
						val := (int(data[i+1]-'0') << 6) | (int(data[i+2]-'0') << 3) | int(data[i+3]-'0')
						result.WriteByte(byte(val))
						i += 3
					} else {
						result.WriteByte(data[i+1])
					}
				}
			}
			i += 2
		case data[i] >= 32 && data[i] <= 126:
			result.WriteByte(data[i])
			i++
		default:
			i++
		}
	}
	return result.String()
}

func isOctal(b byte) bool {
	return b >= '0' && b <= '7'
}

func GetPageCount(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer func() { _ = f.Close() }()

	data, err := io.ReadAll(f)
	if err != nil {
		return 0, err
	}

	if len(data) < 5 || string(data[:5]) != strconst.StrPdf {
		return 0, ErrNotPDF
	}

	count := 0
	for i := 0; i < len(data)-9; i++ {
		if string(data[i:i+9]) == "/Count " {
			j := i + 9
			for j < len(data) && data[j] >= '0' && data[j] <= '9' {
				j++
			}
			if j > i+9 {
				var c int
				_, _ = fmt.Sscanf(string(data[i+9:j]), "%d", &c)
				if c > count {
					count = c
				}
			}
		}
	}
	return count, nil
}

func GetMetadata(path string) (PDFMetadata, error) {
	var meta PDFMetadata
	f, err := os.Open(path)
	if err != nil {
		return meta, err
	}
	defer func() { _ = f.Close() }()

	data, err := io.ReadAll(f)
	if err != nil {
		return meta, err
	}

	if len(data) < 5 || string(data[:5]) != strconst.StrPdf {
		return meta, ErrNotPDF
	}

	meta.Pages, _ = GetPageCount(path)

	fields := map[string]*string{
		"/Title":        &meta.Title,
		"/Author":       &meta.Author,
		"/Subject":      &meta.Subject,
		"/Creator":      &meta.Creator,
		"/Producer":     &meta.Producer,
		"/CreationDate": &meta.CreationDate,
		"/ModDate":      &meta.ModDate,
	}

	for field, ptr := range fields {
		idx := findField(data, field)
		if idx >= 0 {
			end := findEndOfString(data, idx+len(field))
			if end > idx+len(field) {
				*ptr = string(data[idx+len(field) : end])
			}
		}
	}

	return meta, nil
}

func findField(data []byte, field string) int {
	fieldBytes := []byte(field)
	for i := 0; i <= len(data)-len(fieldBytes); i++ {
		if string(data[i:i+len(fieldBytes)]) == field {
			return i
		}
	}
	return -1
}

func findEndOfString(data []byte, start int) int {
	paren := 0
	inString := false
	for i := start; i < len(data); i++ {
		if data[i] == '(' && (i == start || data[i-1] != '\\') {
			if !inString {
				inString = true
				continue
			}
			paren++
		} else if data[i] == ')' && (i == 0 || data[i-1] != '\\') {
			if paren > 0 {
				paren--
			} else if inString {
				return i
			}
		}
	}
	return -1
}

func RenderPage(path string, pageNum int, dpi int) ([]byte, error) {
	return nil, fmt.Errorf("PDF rendering not implemented (requires external library)")
}

func ExtractImages(path string) ([][]byte, error) {
	return nil, fmt.Errorf("PDF image extraction not implemented (requires external library)")
}

func IsPDFEncrypted(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer func() { _ = f.Close() }()

	data, err := io.ReadAll(f)
	if err != nil {
		return false, err
	}

	if len(data) < 5 || string(data[:5]) != strconst.StrPdf {
		return false, ErrNotPDF
	}

	for i := 0; i < len(data)-8; i++ {
		if string(data[i:i+9]) == "/Encrypt " {
			return true, nil
		}
	}
	return false, nil
}

func ValidatePDF(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	data, err := io.ReadAll(f)
	if err != nil {
		return err
	}

	if len(data) < 5 || string(data[:5]) != strconst.StrPdf {
		return ErrNotPDF
	}

	if hasEOFMarker(data) {
		return nil
	}

	return errors.New("PDF missing EOF marker (possibly truncated)")
}

func hasEOFMarker(data []byte) bool {
	for i := len(data) - 10; i >= 0 && i < len(data); i-- {
		if i+5 <= len(data) && string(data[i:i+5]) == "%%EOF" {
			return true
		}
	}
	return false
}

func GetPDFVersion(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	header := make([]byte, 8)
	_, err = f.Read(header)
	if err != nil {
		return "", err
	}

	if len(header) >= 8 && string(header[:5]) == strconst.StrPdf {
		return string(header[5:8]), nil
	}
	return "", ErrNotPDF
}
