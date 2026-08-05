package image

import (
	"bytes"
	"image"
	"image/color"
	"testing"
	"time"
)

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		name   string
		data   []byte
		expect Format
	}{
		{"JPEG", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01}, JPEG},
		{"PNG", []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D}, PNG},
		{"GIF87a", []byte("GIF87a\x00\x00\x00\x00\x00\x00"), GIF},
		{"GIF89a", []byte("GIF89a\x00\x00\x00\x00\x00\x00"), GIF},
		{"WebP", append([]byte("RIFF....WEBP"), make([]byte, 8)...), WebP},
		{"Unknown", []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B}, Unknown},
		{"Too short", []byte{0xFF}, Unknown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := DetectFormat(tc.data)
			if got != tc.expect {
				t.Errorf("DetectFormat() = %v, want %v", got, tc.expect)
			}
		})
	}
}

func TestFormat_String(t *testing.T) {
	tests := []struct {
		f      Format
		expect string
	}{
		{JPEG, "jpeg"},
		{PNG, "png"},
		{GIF, "gif"},
		{WebP, "webp"},
		{Unknown, "unknown"},
		{Format(99), "unknown"},
	}

	for _, tc := range tests {
		t.Run(tc.expect, func(t *testing.T) {
			if tc.f.String() != tc.expect {
				t.Errorf("String() = %s, want %s", tc.f.String(), tc.expect)
			}
		})
	}
}

func TestFormat_MIME(t *testing.T) {
	tests := []struct {
		f      Format
		expect string
	}{
		{JPEG, "image/jpeg"},
		{PNG, "image/png"},
		{GIF, "image/gif"},
		{WebP, "image/webp"},
		{Unknown, "application/octet-stream"},
	}

	for _, tc := range tests {
		t.Run(tc.expect, func(t *testing.T) {
			if tc.f.MIME() != tc.expect {
				t.Errorf("MIME() = %s, want %s", tc.f.MIME(), tc.expect)
			}
		})
	}
}

func TestFormat_Extension(t *testing.T) {
	tests := []struct {
		f      Format
		expect string
	}{
		{JPEG, ".jpg"},
		{PNG, ".png"},
		{GIF, ".gif"},
		{WebP, ".webp"},
		{Unknown, ".bin"},
	}

	for _, tc := range tests {
		t.Run(tc.expect, func(t *testing.T) {
			if tc.f.Extension() != tc.expect {
				t.Errorf("Extension() = %s, want %s", tc.f.Extension(), tc.expect)
			}
		})
	}
}

func TestFormatFromExtension(t *testing.T) {
	tests := []struct {
		ext    string
		expect Format
	}{
		{".jpg", JPEG},
		{".jpeg", JPEG},
		{".png", PNG},
		{".gif", GIF},
		{".webp", WebP},
		{"jpg", JPEG},
		{"JPG", JPEG},
		{".unknown", Unknown},
	}

	for _, tc := range tests {
		t.Run(tc.ext, func(t *testing.T) {
			got := FormatFromExtension(tc.ext)
			if got != tc.expect {
				t.Errorf("FormatFromExtension(%s) = %v, want %v", tc.ext, got, tc.expect)
			}
		})
	}
}

func TestFormatFromMIME(t *testing.T) {
	tests := []struct {
		mime   string
		expect Format
	}{
		{"image/jpeg", JPEG},
		{"image/jpg", JPEG},
		{"image/png", PNG},
		{"image/gif", GIF},
		{"image/webp", WebP},
		{"application/octet-stream", Unknown},
	}

	for _, tc := range tests {
		t.Run(tc.mime, func(t *testing.T) {
			got := FormatFromMIME(tc.mime)
			if got != tc.expect {
				t.Errorf("FormatFromMIME(%s) = %v, want %v", tc.mime, got, tc.expect)
			}
		})
	}
}

func TestIsSupportedFormat(t *testing.T) {
	if !IsSupportedFormat(JPEG) {
		t.Error("JPEG should be supported")
	}
	if !IsSupportedFormat(PNG) {
		t.Error("PNG should be supported")
	}
	if IsSupportedFormat(Unknown) {
		t.Error("Unknown should not be supported")
	}
}

func TestDecodeFormat(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.Set(x, y, color.RGBA{255, 0, 0, 255})
		}
	}

	jpegData, err := EncodeFormat(img, JPEG, 90)
	if err != nil {
		t.Fatalf("Encode JPEG failed: %v", err)
	}

	decoded, fmt, err := DecodeFormat(jpegData)
	if err != nil {
		t.Fatalf("DecodeFormat failed: %v", err)
	}
	if fmt != JPEG {
		t.Errorf("Expected JPEG format, got %v", fmt)
	}
	if decoded == nil {
		t.Error("Decoded image should not be nil")
	}
}

func TestEncodeFormat(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))

	tests := []Format{JPEG, PNG, GIF}
	for _, fmt := range tests {
		t.Run(fmt.String(), func(t *testing.T) {
			data, err := EncodeFormat(img, fmt, 80)
			if err != nil {
				t.Fatalf("EncodeFormat(%v) failed: %v", fmt, err)
			}
			if len(data) == 0 {
				t.Error("Encoded data should not be empty")
			}

			detected := DetectFormat(data)
			if detected != fmt {
				t.Errorf("Detected format %v, expected %v", detected, fmt)
			}
		})
	}

	_, err := EncodeFormat(img, Unknown, 80)
	if err == nil {
		t.Error("Expected error for Unknown format")
	}
}

func TestImageProcessor(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 2), uint8(y * 2), 128, 255})
		}
	}

	proc := NewProcessor(img, PNG)

	resized := proc.Resize(50, 50)
	if resized == nil {
		t.Error("Resize should return processor for chaining")
	}

	processedImg := resized.Image()
	if processedImg.Bounds().Dx() != 50 || processedImg.Bounds().Dy() != 50 {
		t.Errorf("Expected 50x50, got %dx%d", processedImg.Bounds().Dx(), processedImg.Bounds().Dy())
	}
}

func TestImageProcessor_Crop(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	proc := NewProcessor(img, PNG)

	cropped := proc.Crop(10, 10, 50, 50)
	processedImg := cropped.Image()
	if processedImg.Bounds().Dx() != 50 || processedImg.Bounds().Dy() != 50 {
		t.Errorf("Expected 50x50, got %dx%d", processedImg.Bounds().Dx(), processedImg.Bounds().Dy())
	}
}

func TestImageProcessor_Grayscale(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.Set(x, y, color.RGBA{255, 128, 64, 255})
		}
	}

	proc := NewProcessor(img, PNG)
	gray := proc.Grayscale()
	processedImg := gray.Image()

	_, _, _, a := processedImg.At(5, 5).RGBA()
	if a == 0 {
		t.Error("Alpha should not be zero")
	}
}

func TestImageProcessor_SetQuality(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	proc := NewProcessor(img, JPEG)

	proc.SetQuality(95)
	proc.SetQuality(150)
	proc.SetQuality(-10)

	data, err := proc.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("Encoded data should not be empty")
	}
}

func TestImageCache(t *testing.T) {
	cache := NewImageCache(10, time.Hour)

	data := []byte("test image data")
	cache.Set("key1", data, PNG, 100, 100)

	gotData, gotFmt, gotW, gotH, ok := cache.Get("key1")
	if !ok {
		t.Fatal("Get should return true for existing key")
	}
	if string(gotData) != "test image data" {
		t.Errorf("Data mismatch: %s", string(gotData))
	}
	if gotFmt != PNG {
		t.Errorf("Format = %v, want PNG", gotFmt)
	}
	if gotW != 100 || gotH != 100 {
		t.Errorf("Dimensions = %dx%d, want 100x100", gotW, gotH)
	}

	_, _, _, _, ok = cache.Get("nonexistent")
	if ok {
		t.Error("Get should return false for nonexistent key")
	}
}

func TestImageCache_Eviction(t *testing.T) {
	cache := NewImageCache(2, time.Hour)

	cache.Set("key1", []byte("data1"), PNG, 100, 100)
	cache.Set("key2", []byte("data2"), PNG, 100, 100)
	cache.Set("key3", []byte("data3"), PNG, 100, 100)

	stats := cache.Stats()
	if stats.EntryCount > 2 {
		t.Errorf("Expected max 2 entries, got %d", stats.EntryCount)
	}
}

func TestImageCache_Delete(t *testing.T) {
	cache := NewImageCache(10, time.Hour)
	cache.Set("key1", []byte("data"), PNG, 100, 100)

	deleted := cache.Delete("key1")
	if !deleted {
		t.Error("Delete should return true for existing key")
	}

	_, _, _, _, ok := cache.Get("key1")
	if ok {
		t.Error("Get should return false after delete")
	}

	deleted = cache.Delete("key1")
	if deleted {
		t.Error("Delete should return false for nonexistent key")
	}
}

func TestImageCache_Clear(t *testing.T) {
	cache := NewImageCache(10, time.Hour)
	cache.Set("key1", []byte("data"), PNG, 100, 100)
	cache.Set("key2", []byte("data"), PNG, 100, 100)

	cache.Clear()

	stats := cache.Stats()
	if stats.EntryCount != 0 {
		t.Errorf("Expected 0 entries after clear, got %d", stats.EntryCount)
	}
}

func TestImageCache_PruneExpired(t *testing.T) {
	cache := NewImageCache(10, 50*time.Millisecond)
	cache.Set("key1", []byte("data"), PNG, 100, 100)

	time.Sleep(100 * time.Millisecond)

	pruned := cache.PruneExpired()
	if pruned != 1 {
		t.Errorf("Expected 1 pruned, got %d", pruned)
	}

	_, _, _, _, ok := cache.Get("key1")
	if ok {
		t.Error("Key should be expired")
	}
}

func TestImageCache_Keys(t *testing.T) {
	cache := NewImageCache(10, time.Hour)
	cache.Set("key1", []byte("data"), PNG, 100, 100)
	cache.Set("key2", []byte("data"), PNG, 100, 100)

	keys := cache.Keys()
	if len(keys) != 2 {
		t.Errorf("Expected 2 keys, got %d", len(keys))
	}
}

func TestFormatError(t *testing.T) {
	err := &FormatError{"test error"}
	if err.Error() != "image: test error" {
		t.Errorf("Error() = %s, want 'image: test error'", err.Error())
	}
}

func TestSupportedFormats(t *testing.T) {
	if len(SupportedFormats) == 0 {
		t.Error("SupportedFormats should not be empty")
	}
	for _, fmt := range SupportedFormats {
		if !IsSupportedFormat(fmt) {
			t.Errorf("Format %v should be supported", fmt)
		}
	}
}

func TestSupportedMIMEs(t *testing.T) {
	for mime, fmt := range SupportedMIMEs {
		if fmt == Unknown {
			t.Errorf("MIME %s should not map to Unknown", mime)
		}
	}
}

func TestSupportedExtensions(t *testing.T) {
	for ext, fmt := range SupportedExtensions {
		if fmt == Unknown {
			t.Errorf("Extension %s should not map to Unknown", ext)
		}
	}
}

var _ = bytes.Compare
