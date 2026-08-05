// Package image provides core image operations: decode, encode, resize, convert, base64, MIME detection.
package image

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"mime"
	"path/filepath"
	"strings"
)

// Decode reads and decodes an image from an io.Reader.
// It automatically detects the format (JPEG, PNG, GIF).
func Decode(r io.Reader) (image.Image, error) {
	img, _, err := image.Decode(r)
	return img, err
}

// DecodeConfig reads the image config (dimensions, color model) without decoding the full image.
func DecodeConfig(r io.Reader) (image.Config, string, error) {
	return image.DecodeConfig(r)
}

// Encode writes an image to an io.Writer in the specified format with quality (for JPEG).
func Encode(w io.Writer, img image.Image, fmt Format, quality int) error {
	switch fmt {
	case JPEG:
		return jpeg.Encode(w, img, &jpeg.Options{Quality: quality})
	case PNG:
		return png.Encode(w, img)
	case GIF:
		return gif.Encode(w, img, nil)
	default:
		return ErrUnsupportedFormat
	}
}

// EncodeToBytes encodes an image to a byte slice in the specified format.
func EncodeToBytes(img image.Image, fmt Format, quality int) ([]byte, error) {
	var buf bytes.Buffer
	err := Encode(&buf, img, fmt, quality)
	return buf.Bytes(), err
}

// Resize resizes an image to the specified width and height using bicubic interpolation.
// If either width or height is 0, the aspect ratio is preserved.
func Resize(img image.Image, width, height int) image.Image {
	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	if width <= 0 && height <= 0 {
		return img
	}

	if width <= 0 {
		width = srcW * height / srcH
	}
	if height <= 0 {
		height = srcH * width / srcW
	}

	if width == srcW && height == srcH {
		return img
	}

	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	scaleBilinear(dst, img)
	return dst
}

// scaleBilinear performs bilinear interpolation scaling.
func scaleBilinear(dst *image.RGBA, src image.Image) {
	srcBounds := src.Bounds()
	dstBounds := dst.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()
	dstW := dstBounds.Dx()
	dstH := dstBounds.Dy()

	xRatio := float64(srcW) / float64(dstW)
	yRatio := float64(srcH) / float64(dstH)

	for y := 0; y < dstH; y++ {
		for x := 0; x < dstW; x++ {
			srcX := float64(x) * xRatio
			srcY := float64(y) * yRatio
			x0 := int(srcX)
			y0 := int(srcY)
			x1 := min(x0+1, srcW-1)
			y1 := min(y0+1, srcH-1)

			dx := srcX - float64(x0)
			dy := srcY - float64(y0)

			c00 := src.At(srcBounds.Min.X+x0, srcBounds.Min.Y+y0)
			c10 := src.At(srcBounds.Min.X+x1, srcBounds.Min.Y+y0)
			c01 := src.At(srcBounds.Min.X+x0, srcBounds.Min.Y+y1)
			c11 := src.At(srcBounds.Min.X+x1, srcBounds.Min.Y+y1)

			r00, g00, b00, a00 := c00.RGBA()
			r10, g10, b10, a10 := c10.RGBA()
			r01, g01, b01, a01 := c01.RGBA()
			r11, g11, b11, a11 := c11.RGBA()

			// Bilinear interpolation
			r := uint32(float64(r00)*(1-dx)*(1-dy) + float64(r10)*dx*(1-dy) + float64(r01)*(1-dx)*dy + float64(r11)*dx*dy)
			g := uint32(float64(g00)*(1-dx)*(1-dy) + float64(g10)*dx*(1-dy) + float64(g01)*(1-dx)*dy + float64(g11)*dx*dy)
			b := uint32(float64(b00)*(1-dx)*(1-dy) + float64(b10)*dx*(1-dy) + float64(b01)*(1-dx)*dy + float64(b11)*dx*dy)
			a := uint32(float64(a00)*(1-dx)*(1-dy) + float64(a10)*dx*(1-dy) + float64(a01)*(1-dx)*dy + float64(a11)*dx*dy)

			dst.SetRGBA(x, y, color.RGBA{
				R: uint8(r >> 8),
				G: uint8(g >> 8),
				B: uint8(b >> 8),
				A: uint8(a >> 8),
			})
		}
	}
}

// ResizeFit resizes an image to fit within the specified dimensions while preserving aspect ratio.
func ResizeFit(img image.Image, maxWidth, maxHeight int) image.Image {
	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	ratioW := float64(maxWidth) / float64(srcW)
	ratioH := float64(maxHeight) / float64(srcH)
	ratio := ratioW
	if ratioH < ratio {
		ratio = ratioH
	}

	if ratio >= 1.0 {
		return img
	}

	newW := int(float64(srcW) * ratio)
	newH := int(float64(srcH) * ratio)
	return Resize(img, newW, newH)
}

// ResizeFill resizes an image to fill the specified dimensions, cropping if necessary.
func ResizeFill(img image.Image, width, height int) image.Image {
	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	ratioW := float64(width) / float64(srcW)
	ratioH := float64(height) / float64(srcH)
	ratio := ratioW
	if ratioH > ratio {
		ratio = ratioH
	}

	newW := int(float64(srcW) * ratio)
	newH := int(float64(srcH) * ratio)

	resized := Resize(img, newW, newH)

	// Crop to exact dimensions
	x := (newW - width) / 2
	y := (newH - height) / 2
	return Crop(resized, image.Rect(x, y, x+width, y+height))
}

// Crop crops an image to the specified rectangle.
func Crop(img image.Image, rect image.Rectangle) image.Image {
	if rgba, ok := img.(*image.RGBA); ok {
		return rgba.SubImage(rect).(*image.RGBA)
	}
	if rgba, ok := img.(*image.NRGBA); ok {
		return rgba.SubImage(rect).(*image.NRGBA)
	}
	if rgba, ok := img.(*image.RGBA64); ok {
		return rgba.SubImage(rect).(*image.RGBA64)
	}
	if rgba, ok := img.(*image.NRGBA64); ok {
		return rgba.SubImage(rect).(*image.NRGBA64)
	}
	if ycbcr, ok := img.(*image.YCbCr); ok {
		return ycbcr.SubImage(rect).(*image.YCbCr)
	}

	// Generic fallback
	dst := image.NewRGBA(rect)
	draw.Draw(dst, rect, img, rect.Min, draw.Src)
	return dst
}

// Convert converts an image to a different format (color model).
// Currently supports conversion to RGBA, NRGBA, Grayscale.
func Convert(img image.Image, target Format) image.Image {
	switch target {
	case JPEG:
		// JPEG typically uses YCbCr, but we return RGBA for further processing
		return ToRGBA(img)
	case PNG:
		return ToNRGBA(img)
	case GIF:
		return ToPaletted(img)
	default:
		return ToRGBA(img)
	}
}

// ToRGBA converts any image to *image.RGBA.
func ToRGBA(img image.Image) *image.RGBA {
	if rgba, ok := img.(*image.RGBA); ok {
		return rgba
	}
	bounds := img.Bounds()
	dst := image.NewRGBA(bounds)
	draw.Draw(dst, bounds, img, bounds.Min, draw.Src)
	return dst
}

// ToNRGBA converts any image to *image.NRGBA (non-premultiplied alpha).
func ToNRGBA(img image.Image) *image.NRGBA {
	if nrgba, ok := img.(*image.NRGBA); ok {
		return nrgba
	}
	bounds := img.Bounds()
	dst := image.NewNRGBA(bounds)
	draw.Draw(dst, bounds, img, bounds.Min, draw.Src)
	return dst
}

// ToPaletted converts any image to *image.Paletted (for GIF).
func ToPaletted(img image.Image) *image.Paletted {
	if paletted, ok := img.(*image.Paletted); ok {
		return paletted
	}
	bounds := img.Bounds()
	dst := image.NewPaletted(bounds, nil)
	draw.Draw(dst, bounds, img, bounds.Min, draw.Src)
	return dst
}

// ToGrayscale converts an image to grayscale.
func ToGrayscale(img image.Image) *image.Gray {
	if gray, ok := img.(*image.Gray); ok {
		return gray
	}
	bounds := img.Bounds()
	dst := image.NewGray(bounds)
	draw.Draw(dst, bounds, img, bounds.Min, draw.Src)
	return dst
}

// ToBase64 encodes an image to a base64 string with data URI prefix.
func ToBase64(img image.Image, fmt Format, quality int) (string, error) {
	data, err := EncodeToBytes(img, fmt, quality)
	if err != nil {
		return "", err
	}
	mimeType := fmt.MIME()
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

// ToBase64Raw encodes an image to a raw base64 string (no data URI prefix).
func ToBase64Raw(img image.Image, fmt Format, quality int) (string, error) {
	data, err := EncodeToBytes(img, fmt, quality)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// FromBase64 decodes a base64 string to an image.
// Accepts both raw base64 and data URI format.
func FromBase64(s string) (image.Image, Format, error) {
	// Strip data URI prefix if present
	if strings.HasPrefix(s, "data:") {
		// Find the comma separating metadata from data
		commaIdx := strings.Index(s, ",")
		if commaIdx >= 0 {
			s = s[commaIdx+1:]
			data, err := base64.StdEncoding.DecodeString(s)
			if err != nil {
				return nil, Unknown, err
			}
			return DecodeFormat(data)
		}
	}

	// Raw base64
	data, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, Unknown, err
	}
	return DecodeFormat(data)
}

// DetectMIME detects the MIME type from raw image bytes.
func DetectMIME(data []byte) string {
	fmt := DetectFormat(data)
	return fmt.MIME()
}

// DetectMIMEFromReader detects MIME type from an io.Reader.
func DetectMIMEFromReader(r interface {
	Read([]byte) (int, error)
	Seek(int64, int) (int64, error)
}) (string, error) {
	fmt, err := DetectFormatFromReader(r)
	return fmt.MIME(), err
}

// GetDimensions returns the width and height of an image.
func GetDimensions(img image.Image) (int, int) {
	bounds := img.Bounds()
	return bounds.Dx(), bounds.Dy()
}

// GetBounds returns the bounds rectangle of an image.
func GetBounds(img image.Image) image.Rectangle {
	return img.Bounds()
}

// GetColorModel returns the color model of an image.
func GetColorModel(img image.Image) color.Model {
	return img.ColorModel()
}

// MIMEFromExtension returns the MIME type for a file extension.
func MIMEFromExtension(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	return mime.TypeByExtension(ext)
}

// ExtensionFromMIME returns the file extension for a MIME type.
func ExtensionFromMIME(mimeType string) string {
	exts, _ := mime.ExtensionsByType(mimeType)
	if len(exts) > 0 {
		return exts[0]
	}
	return ""
}
