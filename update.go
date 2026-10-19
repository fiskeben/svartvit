package main

import (
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"log"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// display shows an image on the frame. It is nil unless the server runs with -epd.
var display interface {
	Show(img image.Image) error
}

// displayMu makes sure only one update of the frame happens at a time.
var displayMu sync.Mutex

// updateImage shows a random photo immediately, then on every interval boundary
// (e.g. on the hour for an interval of one hour) until ctx is cancelled.
func updateImage(ctx context.Context, interval time.Duration) {
	update := func() {
		paths := getFilenames()
		if len(paths) == 0 {
			log.Printf("no photos to show")
			return
		}

		if err := showPhoto(pick(paths), func(msg string) { log.Print(msg) }); err != nil {
			log.Printf("failed to update current image: %v", err)
		}
	}

	update()

	select {
	case <-ctx.Done():
		return
	case <-time.After(time.Until(time.Now().Truncate(interval).Add(interval))):
		update()
	}

	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			update()
		}
	}
}

// getFilenames returns the paths of all enabled photos.
func getFilenames() []string {
	e, err := read()
	if err != nil {
		log.Printf("failed to read images: %v", err)
		return nil
	}

	paths := make([]string, 0, len(e))
	for _, img := range e {
		if img.Disabled {
			continue
		}
		paths = append(paths, workingDirectory+"/photos/"+img.Filename)
	}

	return paths
}

// showPhoto renders the photo, saves the result to currentImageFile and shows it on the frame.
// progress is called with a short message before each step.
func showPhoto(filename string, progress func(msg string)) error {
	progress("Preparing " + filepath.Base(filename))
	frame, err := renderFile(filename)
	if err != nil {
		return err
	}

	displayMu.Lock()
	defer displayMu.Unlock()

	if err = writePNG(currentImageFile, frame); err != nil {
		return fmt.Errorf("failed to save current image: %w", err)
	}

	if display == nil {
		progress("Saved to " + currentImageFile + " (no display connected)")
		return nil
	}

	progress("Updating the frame, this takes about 30 seconds")
	if err = display.Show(frame); err != nil {
		return err
	}

	progress("Done")
	return nil
}

// renderFile decodes the photo in filename and renders it for the frame.
func renderFile(filename string) (*image.Paletted, error) {
	src, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer src.Close()

	img, _, err := image.Decode(src)
	if err != nil {
		return nil, fmt.Errorf("failed to decode %s: %w", filename, err)
	}

	return render(img, time.Now())
}

// writePNG replaces filename atomically so readers never see a partially written image.
func writePNG(filename string, img image.Image) error {
	tmp, err := os.CreateTemp(filepath.Dir(filename), filepath.Base(filename)+"-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op once the file has been renamed

	if err = png.Encode(tmp, img); err != nil {
		return errors.Join(err, tmp.Close())
	}

	if err = tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmp.Name(), filename)
}

func pick(images []string) string {
	return images[rand.IntN(len(images))]
}
