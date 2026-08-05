// Package image provides utilities for image and PDF processing operations.
// It includes decoding, encoding, resizing, format conversion, base64 handling,
// PDF text extraction, metadata reading, and page rendering to images.
//
// The package uses only Go standard library packages - no external dependencies.
// PDF operations are limited to basic parsing capabilities available in stdlib.
//
// Example usage:
//
//	img, err := image.Decode(file)
//	if err != nil { return err }
//
//	resized := image.Resize(img, 800, 600)
//	data, err := image.Encode(resized, image.JPEG, 90)
//
// For PDF processing:
//
//	text, err := image.ExtractText("document.pdf")
//	pages, err := image.GetPageCount("document.pdf")
//	pageImg, err := image.RenderPage("document.pdf", 1, 2.0)
package image
