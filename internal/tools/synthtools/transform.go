package synthtools

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// CSVOpts configures CSV transformation.
type CSVOpts struct {
	Delimiter rune
	HasHeader bool
	Columns   []int
	Filter    map[string]string
	SortBy    string
	Limit     int
}

// TransformJSON evaluates a dot-notation query against JSON input.
func TransformJSON(input string, query string) (string, error) {
	var data any
	if err := json.Unmarshal([]byte(input), &data); err != nil {
		return "", fmt.Errorf("invalid JSON input: %w", err)
	}
	if query == "" {
		return input, nil
	}
	for _, part := range strings.Split(query, ".") {
		switch v := data.(type) {
		case map[string]any:
			val, ok := v[part]
			if !ok {
				return "", fmt.Errorf("key %q not found", part)
			}
			data = val
		case []any:
			idx, err := strconv.Atoi(part)
			if err != nil || idx < 0 || idx >= len(v) {
				return "", fmt.Errorf("invalid array index %q", part)
			}
			data = v[idx]
		default:
			return "", fmt.Errorf("cannot traverse into %T at key %q", data, part)
		}
	}
	b, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// TransformCSV performs operations on CSV input.
func TransformCSV(input string, opts CSVOpts) (string, error) {
	if opts.Delimiter == 0 {
		opts.Delimiter = ','
	}
	r := csv.NewReader(strings.NewReader(input))
	r.Comma = opts.Delimiter
	records, err := r.ReadAll()
	if err != nil {
		return "", fmt.Errorf("parse CSV: %w", err)
	}
	if len(records) == 0 {
		return "", nil
	}
	var header []string
	startRow := 0
	if opts.HasHeader {
		header = records[0]
		startRow = 1
	}
	rows := records[startRow:]
	if len(opts.Columns) > 0 && header != nil {
		newHdr := make([]string, 0, len(opts.Columns))
		filtered := make([][]string, len(rows))
		for _, c := range opts.Columns {
			if c >= 0 && c < len(header) {
				newHdr = append(newHdr, header[c])
			} else {
				newHdr = append(newHdr, "")
			}
		}
		for i, row := range rows {
			nr := make([]string, 0, len(opts.Columns))
			for _, c := range opts.Columns {
				if c >= 0 && c < len(row) {
					nr = append(nr, row[c])
				} else {
					nr = append(nr, "")
				}
			}
			filtered[i] = nr
		}
		header, rows = newHdr, filtered
	}
	if len(opts.Filter) > 0 && header != nil {
		var filtered [][]string
		for _, row := range rows {
			match := true
			for colName, colVal := range opts.Filter {
				idx := -1
				for i, h := range header {
					if h == colName {
						idx = i
						break
					}
				}
				if idx < 0 || idx >= len(row) || row[idx] != colVal {
					match = false
					break
				}
			}
			if match {
				filtered = append(filtered, row)
			}
		}
		rows = filtered
	}
	if opts.SortBy != "" && header != nil {
		sortIdx := -1
		for i, h := range header {
			if h == opts.SortBy {
				sortIdx = i
				break
			}
		}
		if sortIdx >= 0 {
			sort.SliceStable(rows, func(i, j int) bool {
				if sortIdx < len(rows[i]) && sortIdx < len(rows[j]) {
					return rows[i][sortIdx] < rows[j][sortIdx]
				}
				return false
			})
		}
	}
	if opts.Limit > 0 && len(rows) > opts.Limit {
		rows = rows[:opts.Limit]
	}
	var sb strings.Builder
	w := csv.NewWriter(&sb)
	w.Comma = opts.Delimiter
	if header != nil {
		_ = w.Write(header)
	}
	_ = w.WriteAll(rows)
	return sb.String(), nil
}

// TransformYAML extracts a value at a dot-notation path from YAML input.
func TransformYAML(input string, path string) (string, error) {
	var data any
	if err := yamlUnmarshal([]byte(input), &data); err != nil {
		return "", fmt.Errorf("invalid YAML input: %w", err)
	}
	if path == "" || path == "." {
		return input, nil
	}
	for _, part := range strings.Split(path, ".") {
		m, ok := data.(map[string]any)
		if !ok {
			return "", fmt.Errorf("cannot traverse into %T at key %q", data, part)
		}
		val, ok := m[part]
		if !ok {
			return "", fmt.Errorf("key %q not found", part)
		}
		data = val
	}
	b, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func Base64Encode(input string) string { return base64.StdEncoding.EncodeToString([]byte(input)) }

func Base64Decode(input string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(input))
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}
	return string(b), nil
}

func URLEncode(input string) string { return url.QueryEscape(input) }

func URLDecode(input string) (string, error) {
	s, err := url.QueryUnescape(input)
	if err != nil {
		return "", fmt.Errorf("URL decode: %w", err)
	}
	return s, nil
}

func HashString(input string, algorithm string) string {
	data := []byte(input)
	switch strings.ToLower(algorithm) {
	case "md5":
		h := md5.Sum(data)
		return hex.EncodeToString(h[:])
	case "sha1":
		h := sha1.Sum(data)
		return hex.EncodeToString(h[:])
	case "sha512":
		h := sha512.Sum512(data)
		return hex.EncodeToString(h[:])
	default: // sha256
		h := sha256.Sum256(data)
		return hex.EncodeToString(h[:])
	}
}

func HMACString(input string, key string, algorithm string) string {
	switch strings.ToLower(algorithm) {
	case "md5":
		h := hmac.New(md5.New, []byte(key))
		h.Write([]byte(input))
		return hex.EncodeToString(h.Sum(nil))
	case "sha1":
		h := hmac.New(sha1.New, []byte(key))
		h.Write([]byte(input))
		return hex.EncodeToString(h.Sum(nil))
	case "sha512":
		h := hmac.New(sha512.New, []byte(key))
		h.Write([]byte(input))
		return hex.EncodeToString(h.Sum(nil))
	default:
		h := hmac.New(sha256.New, []byte(key))
		h.Write([]byte(input))
		return hex.EncodeToString(h.Sum(nil))
	}
}

func FormatDuration(d time.Duration, format string) string {
	switch strings.ToLower(format) {
	case "long":
		h, m, s := int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60
		var parts []string
		if h > 0 {
			parts = append(parts, fmt.Sprintf("%d hours", h))
		}
		if m > 0 {
			parts = append(parts, fmt.Sprintf("%d minutes", m))
		}
		if s > 0 || len(parts) == 0 {
			parts = append(parts, fmt.Sprintf("%d seconds", s))
		}
		return strings.Join(parts, " ")
	case "compact":
		h, m := int(d.Hours()), int(d.Minutes())%60
		if h > 0 {
			return fmt.Sprintf("%dh%dm", h, m)
		}
		return fmt.Sprintf("%dm", m)
	default:
		return d.Round(time.Millisecond).String()
	}
}

func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func FormatNumber(n int64) string {
	s := strconv.FormatInt(n, 10)
	neg := n < 0
	if neg {
		s = s[1:]
	}
	if len(s) <= 3 {
		if neg {
			return "-" + s
		}
		return s
	}
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	if neg {
		return "-" + string(result)
	}
	return string(result)
}

func UUIDv4() string {
	now := time.Now().UnixNano()
	b := make([]byte, 6)
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		now&0xffffffff, (now>>32)&0xffff,
		((now>>48)&0x0fff)|0x4000, ((now>>60)&0x3)|0x8000, b)
}

func Timestamp() string    { return time.Now().UTC().Format(time.RFC3339) }
func TimestampUnix() int64 { return time.Now().Unix() }

func FormatNumberFloat(n float64, decimals int) string {
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return fmt.Sprintf("%v", n)
	}
	s := fmt.Sprintf("%.*f", decimals, n)
	parts := strings.Split(s, ".")
	intPart := parts[0]
	neg := strings.HasPrefix(intPart, "-")
	if neg {
		intPart = intPart[1:]
	}
	if len(intPart) > 3 {
		var result []byte
		for i, c := range intPart {
			if i > 0 && (len(intPart)-i)%3 == 0 {
				result = append(result, ',')
			}
			result = append(result, byte(c))
		}
		intPart = string(result)
	}
	if neg {
		intPart = "-" + intPart
	}
	if len(parts) > 1 {
		return intPart + "." + parts[1]
	}
	return intPart
}
