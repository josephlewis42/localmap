package mapper

import (
	"image"
	"testing"
)

func TestProcessMonochrome(t *testing.T) {
	// Create a simple test image: 50x50 white background
	img := image.NewRGBA(image.Rect(0, 0, 50, 50))
	for y := 0; y < 50; y++ {
		for x := 0; x < 50; x++ {
			img.Set(x, y, image.White)
		}
	}

	opts := RenderOptions{
		Monochrome: true,
		Dither:     false,
	}

	result := processMonochrome(img, opts)
	if result == nil {
		t.Error("processMonochrome returned nil")
	}

	bounds := result.Bounds()
	if bounds.Dx() != 50 || bounds.Dy() != 50 {
		t.Errorf("Result dimensions: got %dx%d, want %dx%d",
			bounds.Dx(), bounds.Dy(), 50, 50)
	}
}

func TestProcessMonochromeWithDither(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 50, 50))
	for y := 0; y < 50; y++ {
		for x := 0; x < 50; x++ {
			img.Set(x, y, image.White)
		}
	}

	opts := RenderOptions{
		Monochrome: true,
		Dither:     true,
	}

	result := processMonochrome(img, opts)
	if result == nil {
		t.Error("processMonochrome with dither returned nil")
	}

	bounds := result.Bounds()
	if bounds.Dx() != 50 || bounds.Dy() != 50 {
		t.Errorf("Dithered result dimensions: got %dx%d, want %dx%d",
			bounds.Dx(), bounds.Dy(), 50, 50)
	}
}

func TestProcessImage(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, image.White)
		}
	}

	opts := RenderOptions{
		Monochrome: true,
		Dither:     false,
	}

	result := processImage(img, opts)
	if result == nil {
		t.Error("processImage returned nil")
	}

	bounds := result.Bounds()
	if bounds.Dx() != 100 || bounds.Dy() != 100 {
		t.Errorf("Processed image dimensions: got %dx%d, want %dx%d",
			bounds.Dx(), bounds.Dy(), 100, 100)
	}
}
