package main

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

type fakeDisplay struct {
	shown image.Image
}

func (d *fakeDisplay) Show(img image.Image) error {
	d.shown = img
	return nil
}

func Test_showPhoto(t *testing.T) {
	requireFiles(t, "font.ttf", "testimages/58984.jpg")

	if err := loadFont("font.ttf"); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	currentImageFile = filepath.Join(dir, "current.png")

	d := &fakeDisplay{}
	display = d
	defer func() { display = nil }()

	var messages []string
	if err := showPhoto("testimages/58984.jpg", func(msg string) { messages = append(messages, msg) }); err != nil {
		t.Fatal(err)
	}

	if d.shown == nil {
		t.Fatal("expected the photo to be shown on the display")
	}
	if len(messages) == 0 {
		t.Error("expected progress messages")
	}

	f, err := os.Open(currentImageFile)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if _, err = png.Decode(f); err != nil {
		t.Fatalf("current image is not a valid PNG: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected only the current image in %s, found %d files", dir, len(entries))
	}
}

func Test_showPhoto_undecodable(t *testing.T) {
	dir := t.TempDir()
	currentImageFile = filepath.Join(dir, "current.png")

	notAnImage := filepath.Join(dir, "photo.jpg")
	if err := os.WriteFile(notAnImage, []byte("not an image"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := showPhoto(notAnImage, func(string) {}); err == nil {
		t.Fatal("expected an error for a file that isn't an image")
	}
}
