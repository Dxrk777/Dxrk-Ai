package image

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"sync"
)

type Operation func(*ImageProcessor) error

type ImageProcessor struct {
	img     image.Image
	format  Format
	quality int
	mu      sync.Mutex
}

func NewProcessor(img image.Image, format Format) *ImageProcessor {
	return &ImageProcessor{
		img:     img,
		format:  format,
		quality: 85,
	}
}

func (p *ImageProcessor) Resize(width, height int) *ImageProcessor {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.img == nil {
		return p
	}
	bounds := p.img.Bounds()
	newImg := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			srcX := x * bounds.Dx() / width
			srcY := y * bounds.Dy() / height
			newImg.Set(x, y, p.img.At(bounds.Min.X+srcX, bounds.Min.Y+srcY))
		}
	}
	p.img = newImg
	return p
}

func (p *ImageProcessor) Crop(x, y, width, height int) *ImageProcessor {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.img == nil {
		return p
	}
	subImg := p.img.(interface {
		SubImage(r image.Rectangle) image.Image
	}).SubImage(image.Rect(x, y, x+width, y+height))
	p.img = subImg
	return p
}

func (p *ImageProcessor) Rotate(angle float64) *ImageProcessor {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.img == nil {
		return p
	}
	bounds := p.img.Bounds()
	centerX := float64(bounds.Dx()) / 2
	centerY := float64(bounds.Dy()) / 2
	cosA := cos(angle)
	sinA := sin(angle)
	newImg := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			dx := float64(x) - centerX
			dy := float64(y) - centerY
			srcX := int(dx*cosA - dy*sinA + centerX)
			srcY := int(dx*sinA + dy*cosA + centerY)
			if srcX >= bounds.Min.X && srcX < bounds.Max.X && srcY >= bounds.Min.Y && srcY < bounds.Max.Y {
				newImg.Set(x, y, p.img.At(srcX, srcY))
			}
		}
	}
	p.img = newImg
	return p
}

func (p *ImageProcessor) SetQuality(q int) *ImageProcessor {
	p.mu.Lock()
	defer p.mu.Unlock()
	if q < 1 {
		q = 1
	}
	if q > 100 {
		q = 100
	}
	p.quality = q
	return p
}

func (p *ImageProcessor) SetFormat(f Format) *ImageProcessor {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.format = f
	return p
}

func (p *ImageProcessor) Grayscale() *ImageProcessor {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.img == nil {
		return p
	}
	bounds := p.img.Bounds()
	gray := image.NewGray(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			gray.Set(x, y, p.img.At(x, y))
		}
	}
	p.img = gray
	return p
}

func (p *ImageProcessor) Blur(radius int) *ImageProcessor {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.img == nil || radius <= 0 {
		return p
	}
	bounds := p.img.Bounds()
	blurred := image.NewRGBA(bounds)
	kernel := gaussianKernel(radius)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			var r, g, b, a uint32
			var weightSum float64
			for ky := -radius; ky <= radius; ky++ {
				for kx := -radius; kx <= radius; kx++ {
					px := x + kx
					py := y + ky
					if px >= bounds.Min.X && px < bounds.Max.X && py >= bounds.Min.Y && py < bounds.Max.Y {
						cr, cg, cb, ca := p.img.At(px, py).RGBA()
						w := kernel[ky+radius][kx+radius]
						r += uint32(float64(cr) * w)
						g += uint32(float64(cg) * w)
						b += uint32(float64(cb) * w)
						a += uint32(float64(ca) * w)
						weightSum += w
					}
				}
			}
			if weightSum > 0 {
				blurred.Set(x, y, color.RGBA64{
					R: uint16(float64(r) / weightSum),
					G: uint16(float64(g) / weightSum),
					B: uint16(float64(b) / weightSum),
					A: uint16(float64(a) / weightSum),
				})
			}
		}
	}
	p.img = blurred
	return p
}

func (p *ImageProcessor) Encode() ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.img == nil {
		return nil, fmt.Errorf("no image to encode")
	}
	var buf bytes.Buffer
	switch p.format {
	case JPEG:
		err := jpeg.Encode(&buf, p.img, &jpeg.Options{Quality: p.quality})
		if err != nil {
			return nil, err
		}
	case PNG:
		err := png.Encode(&buf, p.img)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported format: %s", p.format)
	}
	return buf.Bytes(), nil
}

func (p *ImageProcessor) Save(path string) error {
	data, err := p.Encode()
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (p *ImageProcessor) Image() image.Image {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.img
}

func cos(angle float64) float64 {
	return 1 - angle*angle/2 + angle*angle*angle*angle/24
}

func sin(angle float64) float64 {
	return angle - angle*angle*angle/6
}

func gaussianKernel(radius int) [][]float64 {
	size := 2*radius + 1
	kernel := make([][]float64, size)
	sigma := float64(radius) / 3.0
	sum := 0.0
	for i := 0; i < size; i++ {
		kernel[i] = make([]float64, size)
		for j := 0; j < size; j++ {
			x := float64(i - radius)
			y := float64(j - radius)
			val := exp(-(x*x + y*y) / (2 * sigma * sigma))
			kernel[i][j] = val
			sum += val
		}
	}
	for i := 0; i < size; i++ {
		for j := 0; j < size; j++ {
			kernel[i][j] /= sum
		}
	}
	return kernel
}

func exp(x float64) float64 {
	result := 1.0
	term := 1.0
	for i := 1; i < 20; i++ {
		term *= x / float64(i)
		result += term
	}
	return result
}
