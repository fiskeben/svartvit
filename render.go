package main

import (
	"image"
	"image/color"
	"image/draw"
	"time"

	xdraw "golang.org/x/image/draw"

	"github.com/fiskeben/svartvit/internal/epd"
)

// currentOverlay is drawn on every photo shown on the frame. It is set with the -overlay flag.
var currentOverlay Overlay = dateBox

// render turns a photo into what the frame shows: scaled, dithered to black and white and with the overlay on top.
func render(img image.Image, now time.Time) (*image.Paletted, error) {
	frame := prepare(img)
	if err := currentOverlay(frame, now); err != nil {
		return nil, err
	}
	return frame, nil
}

// prepare scales img to the frame's width, converts it to greyscale and dithers it to black and white.
// Photos that are wider than the frame get black bars above and below,
// taller photos are cropped to the middle.
func prepare(img image.Image) *image.Paletted {
	b := img.Bounds()
	height := b.Dy() * epd.Width / b.Dx()

	// Dither the brightness rather than the colours: dithering in colour picks black or white by the distance
	// in RGB, which makes strongly coloured photos much darker.
	scaled := image.NewGray(image.Rect(0, 0, epd.Width, height))
	xdraw.CatmullRom.Scale(scaled, scaled.Bounds(), img, b, draw.Src, nil)

	// The first colour in the palette is the background, so the bars are black.
	frame := image.NewPaletted(image.Rect(0, 0, epd.Width, epd.Height), color.Palette{color.Black, color.White})

	top := (epd.Height - height) / 2
	draw.FloydSteinberg.Draw(frame, scaled.Bounds().Add(image.Pt(0, top)), scaled, image.Point{})

	return frame
}
