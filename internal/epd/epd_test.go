package epd

import (
	"bytes"
	"image"
	"image/color"
	"testing"
	"time"

	"periph.io/x/conn/v3/gpio"
)

// fakeBus records everything sent to the display, split into commands and data by the DC pin.
type fakeBus struct {
	dc        *fakePin
	maxTxSeen int
	sent      []transfer
}

type transfer struct {
	command byte
	data    []byte
}

func (b *fakeBus) Tx(w, _ []byte) error {
	b.maxTxSeen = max(b.maxTxSeen, len(w))
	if b.dc.level == gpio.Low {
		b.sent = append(b.sent, transfer{command: w[0]})
		return nil
	}
	last := &b.sent[len(b.sent)-1]
	last.data = append(last.data, w...)
	return nil
}

type fakePin struct {
	level gpio.Level
}

func (p *fakePin) Out(l gpio.Level) error {
	p.level = l
	return nil
}

func (p *fakePin) Read() gpio.Level {
	return p.level
}

func newFakeDevice() (*Device, *fakeBus, *fakePin) {
	dc, busy := &fakePin{}, &fakePin{level: gpio.High}
	bus := &fakeBus{dc: dc}
	d := New(bus, &fakePin{}, dc, busy)
	d.sleep = func(time.Duration) {}
	return d, bus, busy
}

func TestBuffer(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, Width, Height))
	for i := range img.Pix {
		img.Pix[i] = 0xff
	}
	img.SetGray(0, 0, color.Gray{})         // first pixel: highest bit of the first byte
	img.SetGray(9, 0, color.Gray{0x7f})     // tenth pixel: second highest bit of the second byte
	img.SetGray(799, 479, color.Gray{0x80}) // light enough to stay white

	buf, err := Buffer(img)
	if err != nil {
		t.Fatal(err)
	}

	if len(buf) != bufferSize {
		t.Fatalf("got buffer of %d bytes, want %d", len(buf), bufferSize)
	}
	if buf[0] != 0x7f || buf[1] != 0xbf || buf[len(buf)-1] != 0xff {
		t.Errorf("got %#x %#x ... %#x, want 0x7f 0xbf ... 0xff", buf[0], buf[1], buf[len(buf)-1])
	}
	for i := 2; i < len(buf); i++ {
		if buf[i] != 0xff {
			t.Fatalf("byte %d is %#x, want 0xff", i, buf[i])
		}
	}
}

func TestBuffer_wrongSize(t *testing.T) {
	if _, err := Buffer(image.NewGray(image.Rect(0, 0, Height, Width))); err == nil {
		t.Fatal("expected an error for an image that isn't 800x480")
	}
}

func TestShow(t *testing.T) {
	d, bus, _ := newFakeDevice()

	img := image.NewGray(image.Rect(0, 0, Width, Height)) // all black

	if err := d.Show(img); err != nil {
		t.Fatal(err)
	}

	var commands []byte
	for _, s := range bus.sent {
		if s.command != 0x71 { // skip busy polling
			commands = append(commands, s.command)
		}
	}

	want := []byte{
		0x01, 0x04, 0x00, 0x61, 0x15, 0x50, 0x60, 0x65, // init
		0x10, 0x13, 0x12, // clear
		0x10, 0x13, 0x12, // display
		0x02, 0x07, // power off, deep sleep
	}
	if !bytes.Equal(commands, want) {
		t.Errorf("got commands % x\nwant % x", commands, want)
	}

	var black [][]byte
	for _, s := range bus.sent {
		if s.command == 0x10 {
			black = append(black, s.data)
		}
	}
	if len(black) != 2 || !bytes.Equal(black[0], bytes.Repeat([]byte{0xff}, bufferSize)) || !bytes.Equal(black[1], make([]byte, bufferSize)) {
		t.Error("expected a white layer when clearing followed by a black layer for the image")
	}

	if bus.maxTxSeen > d.maxTx {
		t.Errorf("sent %d bytes in one transfer, limit is %d", bus.maxTxSeen, d.maxTx)
	}
}

func TestShow_busyTimeout(t *testing.T) {
	d, _, busy := newFakeDevice()
	busy.level = gpio.Low

	defer func(old time.Duration) { busyTimeout = old }(busyTimeout)
	busyTimeout = 50 * time.Millisecond

	if err := d.Show(image.NewGray(image.Rect(0, 0, Width, Height))); err == nil {
		t.Fatal("expected an error when the display never becomes idle")
	}
}
