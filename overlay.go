package main

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
)

// An Overlay draws on top of the photo after it has been scaled and dithered for the frame.
//
// dst is 800x480 and black and white only: anything drawn in another colour ends up
// as whichever of the two is closest, so grey lighter than 0x80 becomes white.
//
// To try a new design, write a function with this signature, add it to overlays and
// run with -overlay <name>. Open /photo/{id}/preview in the browser to see the result
// without updating the frame.
type Overlay func(dst draw.Image, now time.Time) error

var overlays = map[string]Overlay{
	"date": dateBox,
	"none": func(draw.Image, time.Time) error { return nil },
}

// overlayNames lists the available overlays, for use in the -overlay flag's help text.
func overlayNames() string {
	var names []string
	for name := range overlays {
		names = append(names, name)
	}
	slices.Sort(names)
	return strings.Join(names, ", ")
}

// labelFont is the font used by overlays that draw text. It is loaded once at startup by loadFont.
var labelFont *truetype.Font

func loadFont(filename string) error {
	b, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	f, err := freetype.ParseFont(b)
	if err != nil {
		return err
	}

	labelFont = f
	return nil
}

var months = map[time.Month]string{
	time.January:   "januar",
	time.February:  "februar",
	time.March:     "mars",
	time.April:     "april",
	time.May:       "mai",
	time.June:      "juni",
	time.July:      "juli",
	time.August:    "august",
	time.September: "september",
	time.October:   "oktober",
	time.November:  "november",
	time.December:  "desember",
}

// dateString formats t as e.g. "29. desember".
func dateString(t time.Time) string {
	return fmt.Sprintf("%d. %s", t.Day(), months[t.Month()])
}

// dateBox draws the date in white on a black box with a double border, in the bottom right corner.
func dateBox(dst draw.Image, now time.Time) error {
	if labelFont == nil {
		return errors.New("font not loaded")
	}

	bg, fg := image.Black, image.White
	ruler := color.RGBA{0xdd, 0xdd, 0xdd, 0xff}

	width, height, margin, padding := 300, 64, 16, 16
	b := dst.Bounds()
	box := image.Rect(b.Max.X-(width+margin), b.Max.Y-(height+margin), b.Max.X-margin, b.Max.Y-margin)
	draw.Draw(dst, box, bg, image.Point{}, draw.Src)

	drawSquare(dst,
		box.Min.X+margin/2,
		box.Min.Y+margin/2,
		box.Dx()-margin,
		box.Dy()-margin,
		ruler)
	drawSquare(dst,
		box.Min.X+margin/4,
		box.Min.Y+margin/4,
		box.Dx()-margin/2,
		box.Dy()-margin/2,
		ruler)

	size := 32.0
	clip := image.Rect(box.Min.X+padding, box.Min.Y+padding, box.Max.X-padding, box.Max.Y-padding)
	c := freetype.NewContext()
	c.SetDPI(72)
	c.SetFont(labelFont)
	c.SetFontSize(size)
	c.SetSrc(fg)
	c.SetDst(dst)
	c.SetClip(clip)
	c.SetHinting(font.HintingFull)

	x, y := clip.Min.X+margin, clip.Min.Y+int(c.PointToFixed(size-4)>>6)
	_, err := c.DrawString(dateString(now), freetype.Pt(x, y))
	return err
}

func drawSquare(dest draw.Image, x, y, width, height int, c color.Color) {
	for i := x; i < x+width; i++ {
		dest.Set(i, y, c)
		dest.Set(i, y+height, c)
	}
	for i := y; i < y+height; i++ {
		dest.Set(x, i, c)
		dest.Set(x+width, i, c)
	}
}
