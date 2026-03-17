package imaging

import (
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // register PNG decoder
	"io"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"golang.org/x/image/draw"
)

const (
	maxFullSize  = 1600
	maxThumbSize = 400
	jpegQuality  = 80
)

// ProcessUpload decodes an image from src, resizes it, and saves both a
// full-size version and a thumbnail under uploadDir/photos/.
// Returns the relative paths (e.g. "photos/abc.jpg", "photos/thumbs/abc.jpg").
func ProcessUpload(src io.Reader, uploadDir string) (filename, thumbnail string, err error) {
	img, _, err := image.Decode(src)
	if err != nil {
		return "", "", fmt.Errorf("decode image: %w", err)
	}

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
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return jpeg.Encode(f, img, &jpeg.Options{Quality: jpegQuality})
}
