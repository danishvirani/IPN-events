package imaging

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png" // register PNG decoder
	"io"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/rwcarlsen/goexif/exif"
	"golang.org/x/image/draw"
)

const (
	maxFullSize  = 1600
	maxThumbSize = 400
	jpegQuality  = 80
)

// ProcessUpload decodes an image from src, applies EXIF orientation,
// resizes it, and saves both a full-size version and a thumbnail
// under uploadDir/photos/.
// Returns the relative paths (e.g. "photos/abc.jpg", "photos/thumbs/abc.jpg").
func ProcessUpload(src io.Reader, uploadDir string) (filename, thumbnail string, err error) {
	// Buffer the whole file so we can read EXIF then decode.
	buf, err := io.ReadAll(src)
	if err != nil {
		return "", "", fmt.Errorf("read upload: %w", err)
	}

	// Read EXIF orientation (best-effort).
	orientation := getOrientation(buf)

	img, _, err := image.Decode(bytes.NewReader(buf))
	if err != nil {
		return "", "", fmt.Errorf("decode image: %w", err)
	}

	// Apply EXIF orientation so the image displays correctly.
	img = applyOrientation(img, orientation)

	id := uuid.New().String()
	filename = filepath.Join("photos", id+".jpg")
	thumbnail = filepath.Join("photos", "thumbs", id+".jpg")

	full := resize(img, maxFullSize)
	if err := saveJPEG(filepath.Join(uploadDir, filename), full); err != nil {
		return "", "", fmt.Errorf("save full image: %w", err)
	}

	thumb := resize(img, maxThumbSize)
	if err := saveJPEG(filepath.Join(uploadDir, thumbnail), thumb); err != nil {
		return "", "", fmt.Errorf("save thumbnail: %w", err)
	}

	return filename, thumbnail, nil
}

// getOrientation reads the EXIF orientation tag from JPEG data.
// Returns 1 (normal) if unreadable.
func getOrientation(data []byte) int {
	x, err := exif.Decode(bytes.NewReader(data))
	if err != nil {
		return 1
	}
	tag, err := x.Get(exif.Orientation)
	if err != nil {
		return 1
	}
	v, err := tag.Int(0)
	if err != nil {
		return 1
	}
	return v
}

// applyOrientation transforms the image according to the EXIF orientation tag.
// iPhone photos typically use orientation 6 (rotated 90° CW).
func applyOrientation(img image.Image, orientation int) image.Image {
	switch orientation {
	case 1:
		return img // normal
	case 2:
		return flipH(img)
	case 3:
		return rotate180(img)
	case 4:
		return flipV(img)
	case 5:
		return transpose(img)
	case 6:
		return rotate90CW(img)
	case 7:
		return transverse(img)
	case 8:
		return rotate90CCW(img)
	default:
		return img
	}
}

// resize scales img so that its longest edge is at most maxPx.
// Returns the original if already small enough.
func resize(img image.Image, maxPx int) image.Image {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	if w <= maxPx && h <= maxPx {
		return img
	}

	var newW, newH int
	if w >= h {
		newW = maxPx
		newH = h * maxPx / w
	} else {
		newH = maxPx
		newW = w * maxPx / h
	}

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)
	return dst
}

func saveJPEG(path string, img image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return jpeg.Encode(f, img, &jpeg.Options{Quality: jpegQuality})
}

// --- orientation transforms ---

func rotate90CW(img image.Image) image.Image {
	b := img.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dy(), b.Dx()))
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			dst.Set(b.Max.Y-1-y, x, img.At(x, y))
		}
	}
	return dst
}

func rotate90CCW(img image.Image) image.Image {
	b := img.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dy(), b.Dx()))
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			dst.Set(y, b.Max.X-1-x, img.At(x, y))
		}
	}
	return dst
}

func rotate180(img image.Image) image.Image {
	b := img.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			dst.Set(b.Max.X-1-x, b.Max.Y-1-y, img.At(x, y))
		}
	}
	return dst
}

func flipH(img image.Image) image.Image {
	b := img.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			dst.Set(b.Max.X-1-x, y, img.At(x, y))
		}
	}
	return dst
}

func flipV(img image.Image) image.Image {
	b := img.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			dst.Set(x, b.Max.Y-1-y, img.At(x, y))
		}
	}
	return dst
}

func transpose(img image.Image) image.Image {
	b := img.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dy(), b.Dx()))
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			dst.Set(y, x, img.At(x, y))
		}
	}
	return dst
}

func transverse(img image.Image) image.Image {
	b := img.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dy(), b.Dx()))
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			dst.Set(b.Max.Y-1-y, b.Max.X-1-x, img.At(x, y))
		}
	}
	return dst
}

// colorAt is a helper that returns the color at (x, y) or transparent if out of bounds.
func colorAt(img image.Image, x, y int) color.Color {
	b := img.Bounds()
	if x < b.Min.X || x >= b.Max.X || y < b.Min.Y || y >= b.Max.Y {
		return color.Transparent
	}
	return img.At(x, y)
}
