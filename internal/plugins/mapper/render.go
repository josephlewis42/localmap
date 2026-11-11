package mapper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"net/http"
	"os"
	"os/exec"
	"strconv"

	"example.com/localmap/internal/cache"
	"example.com/localmap/internal/config"
)

// RenderOptions defines the parameters used by the render pipeline.
type RenderOptions struct {
	Width      int     `json:"width"`
	Height     int     `json:"height"`
	Profile    string  `json:"profile"`
	Monochrome bool    `json:"monochrome"`
	Colors     int     `json:"colors"`
	Depth      int     `json:"depth"`
	Dither     bool    `json:"dither"`
	CenterLat  float64 `json:"center_lat"`
	CenterLon  float64 `json:"center_lon"`
	Zoom       float64 `json:"zoom"`
	HTMLURL    string  `json:"-"` // full URL to /map/html, set at request time
}

func NewRenderOptions(r *http.Request, cfg config.Config) RenderOptions {
	q := r.URL.Query()

	opts := RenderOptions{
		Width:      parseInt(q.Get("width_px"), 800),
		Height:     parseInt(q.Get("height_px"), 480),
		Profile:    q.Get("profile"),
		Monochrome: q.Get("monochrome") == "true",
		Colors:     parseInt(q.Get("colors"), 256),
		Depth:      parseInt(q.Get("depth"), 8),
		Dither:     q.Get("dither") == "true",
		CenterLat:  cfg.MapLatDD,
		CenterLon:  cfg.MapLonDD,
		Zoom:       cfg.MapZoom,
	}

	switch opts.Profile {
	case "trmnl_v1_alias":
		opts.Monochrome = true
		opts.Colors = 2
		opts.Depth = 1
		opts.Dither = true
		opts.Width = 800
		opts.Height = 480
	}

	if opts.Width <= 0 {
		opts.Width = 800
	}
	if opts.Height <= 0 {
		opts.Height = 480
	}
	if opts.Zoom <= 0 {
		opts.Zoom = 12
	}
	if opts.Colors <= 2 {
		opts.Monochrome = true
	}

	return opts
}

func (opts RenderOptions) Args() []string {
	switch opts.Profile {
	case "trmnl_v1_alias":
		// https://docs.trmnl.com/go/diy/imagemagick-guide#h_6b95d41fbd
		return []string{
			"-dither", "FloydSteinberg",
			"-remap", "pattern:gray50",
			"-depth", "1",
			"-strip",
		}
	}

	var args []string
	if opts.Monochrome {
		args = append(args, "-colorspace", "Gray")
	}

	if opts.Dither {
		args = append(args, "-dither", "FloydSteinberg")
	}

	if opts.Colors > 0 {
		args = append(args, "-colors", strconv.Itoa(opts.Colors))
	}

	if opts.Depth > 0 {
		args = append(args, "-depth", strconv.Itoa(opts.Depth))
	}

	return args
}

func (opts RenderOptions) ToCacheKey() string {
	jsonData, err := json.Marshal(opts)
	if err != nil {
		return ""
	}

	return cache.MakeKey(map[string]string{
		"struct": string(jsonData),
	})
}

// RenderPNG renders a PNG image from provided rendering options.
func RenderPNG(opts RenderOptions) ([]byte, error) {
	raw, err := renderWithBrowser(opts)
	if err != nil {
		return nil, err
	}

	return processScreenshot(raw, opts)
}

func processScreenshot(raw []byte, opts RenderOptions) ([]byte, error) {
	if opts.Depth == 1 {
		return processWithImageMagick(raw, opts)
	}

	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}

	processed := processImage(img, opts)
	var buf bytes.Buffer
	if err := png.Encode(&buf, processed); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func processWithImageMagick(raw []byte, opts RenderOptions) ([]byte, error) {
	bin, err := exec.LookPath("magick")
	if err != nil {
		bin, err = exec.LookPath("convert")
		if err != nil {
			return nil, fmt.Errorf("imagemagick not found (tried magick and convert): %w", err)
		}
	}

	inFile, err := os.CreateTemp("", "localmap-in-*.png")
	if err != nil {
		return nil, err
	}
	defer os.Remove(inFile.Name())
	if _, err := inFile.Write(raw); err != nil {
		inFile.Close()
		return nil, err
	}
	inFile.Close()

	outFile, err := os.CreateTemp("", "localmap-out-*.png")
	if err != nil {
		return nil, err
	}
	outName := outFile.Name()
	outFile.Close()
	defer os.Remove(outName)

	args := []string{inFile.Name()}
	args = append(args, opts.Args()...)
	args = append(args, fmt.Sprintf("png:%s", outName))

	cmd := exec.Command(bin, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("imagemagick failed: %w\n%s", err, out)
	}

	return os.ReadFile(outName)
}

func processImage(src image.Image, opts RenderOptions) image.Image {
	bounds := src.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, src, bounds.Min, draw.Src)

	if opts.Monochrome || opts.Colors <= 2 {
		return processMonochrome(rgba, opts)
	}

	return rgba
}

func processMonochrome(src *image.RGBA, opts RenderOptions) image.Image {
	bounds := src.Bounds()
	out := image.NewRGBA(bounds)
	width := bounds.Dx()
	height := bounds.Dy()

	err := make([][]float64, height)
	for y := 0; y < height; y++ {
		err[y] = make([]float64, width)
	}

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			idxX := x - bounds.Min.X
			idxY := y - bounds.Min.Y
			c := color.GrayModel.Convert(src.At(x, y)).(color.Gray)
			value := float64(c.Y) + err[idxY][idxX]

			var newColor color.RGBA
			if value >= 128 {
				newColor = color.RGBA{255, 255, 255, 255}
			} else {
				newColor = color.RGBA{0, 0, 0, 255}
			}
			out.Set(x, y, newColor)

			if opts.Dither {
				er := value - float64(newColor.R)
				if idxX+1 < width {
					err[idxY][idxX+1] += er * 7 / 16
				}
				if idxY+1 < height {
					if idxX > 0 {
						err[idxY+1][idxX-1] += er * 3 / 16
					}
					err[idxY+1][idxX] += er * 5 / 16
					if idxX+1 < width {
						err[idxY+1][idxX+1] += er * 1 / 16
					}
				}
				if idxX+2 < width {
					err[idxY][idxX+2] += er * 1 / 16
				}
			}
		}
	}
	return out
}
