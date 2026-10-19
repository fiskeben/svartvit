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
	"golang.org/x/image/math/fixed"
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

// dateBox draws the date in black on a white box with a double border, in the bottom right corner.
// The box is sized to fit the text.
func dateBox(dst draw.Image, now time.Time) error {
	if labelFont == nil {
		return errors.New("font not loaded")
	}

	bg, fg, ruler := image.White, image.Black, image.Black

	face := truetype.NewFace(labelFont, &truetype.Options{Size: 32, DPI: 72, Hinting: font.HintingFull})
	defer face.Close()
	d := &font.Drawer{Dst: dst, Src: fg, Face: face}

	// Use the ink bounds rather than the advance so the padding is the same on both sides,
	// and the height of a digit so the baseline doesn't move with letters that go below it.
	text := dateString(now)
	ink, _ := d.BoundString(text)
	digit, _ := d.BoundString("0")
	textWidth, capHeight := (ink.Max.X - ink.Min.X).Ceil(), (digit.Max.Y - digit.Min.Y).Ceil()

	// The photo frame covers more of the panel's right edge than its bottom edge.
	height, marginRight, marginBottom, padding := 64, 64, 32, 24
	width := textWidth + 2*padding
	b := dst.Bounds()
	box := image.Rect(b.Max.X-(width+marginRight), b.Max.Y-(height+marginBottom), b.Max.X-marginRight, b.Max.Y-marginBottom)
	draw.Draw(dst, box, bg, image.Point{}, draw.Src)

	drawSquare(dst, box.Min.X+8, box.Min.Y+8, box.Dx()-16, box.Dy()-16, ruler)
	drawSquare(dst, box.Min.X+4, box.Min.Y+4, box.Dx()-8, box.Dy()-8, ruler)

	d.Dot = fixed.P(box.Min.X+padding, box.Min.Y+(height+capHeight)/2).Sub(fixed.Point26_6{X: ink.Min.X})
	d.DrawString(text)
	return nil
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
