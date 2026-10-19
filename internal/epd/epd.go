// Package epd drives a Waveshare 7.5" e-Paper display (B), version 2: 800x480 pixels in black, white and red.
//
// It is a port of Waveshare's epd7in5b_V2.py. Only black and white are used; the red layer is always left blank.
package epd

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"time"

	"periph.io/x/conn/v3"
	"periph.io/x/conn/v3/gpio"
)

const (
	Width  = 800
	Height = 480

	// bufferSize is the size of one layer (black or red) with one bit per pixel.
	bufferSize = Width * Height / 8
)

// busyTimeout is how long to wait for the display to finish an operation before giving up.
// A full refresh takes about 15-20 seconds.
var busyTimeout = time.Minute

// Bus is the SPI connection to the display. spi.Conn satisfies it.
type Bus interface {
	Tx(w, r []byte) error
}

// OutPin is a GPIO pin used as output. gpio.PinOut satisfies it.
type OutPin interface {
	Out(l gpio.Level) error
}

// InPin is a GPIO pin used as input. gpio.PinIn satisfies it.
type InPin interface {
	Read() gpio.Level
}

// Device is a display connected through SPI and three GPIO pins.
type Device struct {
	bus   Bus
	reset OutPin
	dc    OutPin // low for commands, high for data
	busy  InPin  // low while the display is busy

	// maxTx is the largest number of bytes that can be sent in a single SPI transfer.
	maxTx int
	sleep func(time.Duration)
}

// New returns a device using the given SPI connection and pins.
func New(bus Bus, reset, dc OutPin, busy InPin) *Device {
	maxTx := 4096 // the default buffer size of the Linux spidev driver
	if l, ok := bus.(conn.Limits); ok && l.MaxTxSize() > 0 {
		maxTx = l.MaxTxSize()
	}

	return &Device{
		bus:   bus,
		reset: reset,
		dc:    dc,
		busy:  busy,
		maxTx: maxTx,
		sleep: time.Sleep,
	}
}

// Show displays img and puts the display to sleep afterwards. It blocks until the display has been refreshed.
//
// img must be Width x Height pixels. Dark pixels are shown as black, light pixels as white.
func (d *Device) Show(img image.Image) error {
	buf, err := Buffer(img)
	if err != nil {
		return err
	}

	if err = d.init(); err != nil {
		return fmt.Errorf("failed to initialize display: %w", err)
	}

	if err = d.clear(); err != nil {
		return fmt.Errorf("failed to clear display: %w", err)
	}

	if err = d.refresh(buf); err != nil {
		return fmt.Errorf("failed to update display: %w", err)
	}

	if err = d.powerOff(); err != nil {
		return fmt.Errorf("failed to put display to sleep: %w", err)
	}

	return nil
}

// Buffer converts img to the display's format: one bit per pixel, most significant bit first, 0 for black.
func Buffer(img image.Image) ([]byte, error) {
	b := img.Bounds()
	if b.Dx() != Width || b.Dy() != Height {
		return nil, fmt.Errorf("image must be %dx%d, got %dx%d", Width, Height, b.Dx(), b.Dy())
	}

	buf := make([]byte, bufferSize)
	for i := range buf {
		buf[i] = 0xff
	}

	for y := range Height {
		for x := range Width {
			if color.GrayModel.Convert(img.At(b.Min.X+x, b.Min.Y+y)).(color.Gray).Y < 0x80 {
				i := x + y*Width
				buf[i/8] &^= 0x80 >> (i % 8)
			}
		}
	}

	return buf, nil
}

func (d *Device) init() error {
	if err := d.hardwareReset(); err != nil {
		return err
	}

	steps := []struct {
		command byte
		data    []byte
	}{
		{0x01, []byte{0x07, 0x07, 0x3f, 0x3f}}, // power setting: VGH=20V, VGL=-20V, VDH=15V, VDL=-15V
		{0x04, nil},                            // power on
	}
	for _, s := range steps {
		if err := d.send(s.command, s.data...); err != nil {
			return err
		}
	}

	d.sleep(100 * time.Millisecond)
	if err := d.waitUntilIdle(); err != nil {
		return err
	}

	steps = []struct {
		command byte
		data    []byte
	}{
		{0x00, []byte{0x0f}},                   // panel setting: KW-3f, KWR-2F, BWROTP 0f, BWOTP 1f
		{0x61, []byte{0x03, 0x20, 0x01, 0xe0}}, // resolution: source 800, gate 480
		{0x15, []byte{0x00}},                   // dual SPI off
		{0x50, []byte{0x11, 0x07}},             // VCOM and data interval
		{0x60, []byte{0x22}},                   // TCON
		{0x65, []byte{0x00, 0x00, 0x00, 0x00}}, // resolution gate start
	}
	for _, s := range steps {
		if err := d.send(s.command, s.data...); err != nil {
			return err
		}
	}

	return nil
}

func (d *Device) hardwareReset() error {
	for _, step := range []struct {
		level gpio.Level
		wait  time.Duration
	}{
		{gpio.High, 200 * time.Millisecond},
		{gpio.Low, 4 * time.Millisecond},
		{gpio.High, 200 * time.Millisecond},
	} {
		if err := d.reset.Out(step.level); err != nil {
			return err
		}
		d.sleep(step.wait)
	}

	return nil
}

// clear sets the whole display to white.
func (d *Device) clear() error {
	white := make([]byte, bufferSize)
	for i := range white {
		white[i] = 0xff
	}

	return d.refresh(white)
}

// refresh shows the black layer in buf, leaving the red layer blank.
func (d *Device) refresh(black []byte) error {
	if err := d.send(0x10, black...); err != nil { // black layer
		return err
	}

	if err := d.send(0x13, make([]byte, bufferSize)...); err != nil { // red layer, 0 is no red
		return err
	}

	if err := d.send(0x12); err != nil { // refresh
		return err
	}

	d.sleep(100 * time.Millisecond)
	return d.waitUntilIdle()
}

func (d *Device) powerOff() error {
	if err := d.send(0x02); err != nil { // power off
		return err
	}

	if err := d.waitUntilIdle(); err != nil {
		return err
	}

	if err := d.send(0x07, 0xa5); err != nil { // deep sleep
		return err
	}

	d.sleep(2 * time.Second)

	return errors.Join(d.reset.Out(gpio.Low), d.dc.Out(gpio.Low))
}

// waitUntilIdle polls the busy pin until the display is ready for the next command.
func (d *Device) waitUntilIdle() error {
	for waited := time.Duration(0); ; waited += 10 * time.Millisecond {
		if err := d.send(0x71); err != nil { // get status, which updates the busy pin
			return err
		}

		if d.busy.Read() == gpio.High {
			break
		}

		if waited > busyTimeout {
			return fmt.Errorf("display still busy after %s", busyTimeout)
		}
		d.sleep(10 * time.Millisecond)
	}

	d.sleep(200 * time.Millisecond)
	return nil
}

// send sends a command followed by its data, if any.
func (d *Device) send(command byte, data ...byte) error {
	if err := d.dc.Out(gpio.Low); err != nil {
		return err
	}

	if err := d.bus.Tx([]byte{command}, nil); err != nil {
		return err
	}

	if len(data) == 0 {
		return nil
	}

	if err := d.dc.Out(gpio.High); err != nil {
		return err
	}

	for len(data) > 0 {
		n := min(len(data), d.maxTx)
		if err := d.bus.Tx(data[:n], nil); err != nil {
			return err
		}
		data = data[n:]
	}

	return nil
}
