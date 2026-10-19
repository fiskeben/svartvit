package epd

import (
	"errors"
	"fmt"
	"io"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/pin"
	"periph.io/x/conn/v3/spi"
	"periph.io/x/conn/v3/spi/spireg"
	"periph.io/x/host/v3"
)

// Pins and SPI port as wired on the Waveshare e-Paper HAT, using BCM numbering.
// Chip select (csPin) is driven by the SPI port itself.
const (
	spiPort  = "SPI0.0"
	csPin    = "GPIO8"
	resetPin = "GPIO17"
	dcPin    = "GPIO25"
	busyPin  = "GPIO24"
)

// Open connects to the display through the Raspberry Pi's SPI port and GPIO pins.
// The returned io.Closer releases the SPI port.
func Open() (*Device, io.Closer, error) {
	state, err := host.Init()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize host drivers: %w", err)
	}

	port, err := spireg.Open(spiPort)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open %s: %w", spiPort, err)
	}

	bus, err := port.Connect(4*physic.MegaHertz, spi.Mode0, 8)
	if err != nil {
		port.Close()
		return nil, nil, fmt.Errorf("failed to connect to %s: %w", spiPort, err)
	}

	reset, dc, busy := gpioreg.ByName(resetPin), gpioreg.ByName(dcPin), gpioreg.ByName(busyPin)
	if reset == nil || dc == nil || busy == nil {
		port.Close()

		// Drivers that fail to load don't make host.Init fail, so report them here.
		failed := make([]error, 0, len(state.Failed))
		for _, f := range state.Failed {
			failed = append(failed, fmt.Errorf("%s: %w", f.D, f.Err))
		}
		return nil, nil, fmt.Errorf("GPIO pins not found, failed drivers: %w", errors.Join(failed...))
	}

	// The kernel drives chip select either as an output or through the SPI function. Other programs, such as
	// Waveshare's Python driver, can leave it as an input, and then the display ignores everything sent to it.
	if cs, ok := gpioreg.ByName(csPin).(pin.PinFunc); ok {
		if f := cs.Func(); f == gpio.IN || f == gpio.IN_LOW || f == gpio.IN_HIGH {
			port.Close()
			return nil, nil, fmt.Errorf("%s (SPI chip select) is configured as input, reboot or run 'raspi-gpio set 8 op dh' to fix", csPin)
		}
	}

	if err = busy.In(gpio.Float, gpio.NoEdge); err != nil {
		port.Close()
		return nil, nil, fmt.Errorf("failed to set up %s: %w", busyPin, err)
	}

	return New(bus, reset, dc, busy), port, nil
}
