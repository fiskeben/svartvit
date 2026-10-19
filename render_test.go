package main

import (
	"image"
	"image/color"
	"testing"

	"github.com/fiskeben/svartvit/internal/epd"
)

func Test_prepare(t *testing.T) {
	white := func(w, h int) image.Image {
		img := image.NewGray(image.Rect(0, 0, w, h))
		for i := range img.Pix {
			img.Pix[i] = 0xff
		}
		return img
	}

	isWhite := func(frame *image.Paletted, x, y int) bool {
		return frame.At(x, y) == color.White
	}

	t.Run("wide photo gets black bars", func(t *testing.T) {
		frame := prepare(white(1600, 400)) // scales to 800x200, bars of 140 px

		if got := frame.Bounds(); got != image.Rect(0, 0, epd.Width, epd.Height) {
			t.Fatalf("got bounds %v", got)
		}
		if isWhite(frame, 400, 139) || isWhite(frame, 400, 340) {
			t.Error("expected black bars above and below the photo")
		}
		if !isWhite(frame, 400, 140) || !isWhite(frame, 400, 339) {
			t.Error("expected the photo to fill the middle of the frame")
		}
	})

	t.Run("tall photo is cropped", func(t *testing.T) {
		frame := prepare(white(400, 400)) // scales to 800x800

		if !isWhite(frame, 0, 0) || !isWhite(frame, 799, 479) {
			t.Error("expected the photo to fill the whole frame")
		}
	})
}
