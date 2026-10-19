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

func Test_prepare_keepsBrightnessOfColours(t *testing.T) {
	for _, c := range []color.RGBA{
		{0xff, 0x00, 0x00, 0xff}, // red
		{0xff, 0x80, 0x00, 0xff}, // orange
		{0x00, 0xff, 0x00, 0xff}, // green
		{0x80, 0x80, 0x80, 0xff}, // grey
	} {
		img := image.NewRGBA(image.Rect(0, 0, epd.Width, epd.Height))
		for i := 0; i < len(img.Pix); i += 4 {
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, c.A
		}

		frame := prepare(img)

		white := 0
		for _, p := range frame.Pix {
			if p == 1 {
				white++
			}
		}

		got := float64(white) / float64(len(frame.Pix))
		want := float64(color.GrayModel.Convert(c).(color.Gray).Y) / 0xff
		if got < want-0.02 || got > want+0.02 {
			t.Errorf("%v: got %.0f%% white pixels, want about %.0f%%", c, 100*got, 100*want)
		}
	}
}
