package main

import (
	"image"
	"os"
	"testing"
	"time"
)

// TestOverlays renders every overlay onto a test photo and saves it as testimage-<name>.png for inspection.
func TestOverlays(t *testing.T) {
	requireFiles(t, "font.ttf", "testimages/58984.jpg")

	if err := loadFont("font.ttf"); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open("testimages/58984.jpg")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Date(2024, time.December, 29, 12, 0, 0, 0, time.Local)

	for name, overlay := range overlays {
		t.Run(name, func(t *testing.T) {
			frame := prepare(img)
			if err := overlay(frame, now); err != nil {
				t.Fatal(err)
			}

			filename := "testimage-" + name + ".png"
			if err := writePNG(filename, frame); err != nil {
				t.Fatal(err)
			}
			t.Log("Image created at", filename)
		})
	}
}

// requireFiles skips the test unless all the files exist.
// The font and the test photos aren't in the repository, so they have to be added locally.
func requireFiles(t *testing.T, filenames ...string) {
	t.Helper()
	for _, f := range filenames {
		if _, err := os.Stat(f); err != nil {
			t.Skipf("%s not found, add it locally to run this test", f)
		}
	}
}

func Test_dateString(t *testing.T) {
	if got := dateString(time.Date(2024, time.December, 29, 0, 0, 0, 0, time.Local)); got != "29. desember" {
		t.Errorf("got %q, want %q", got, "29. desember")
	}
}
