package epd

import (
	"errors"
	"fmt"
	"io"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/spi"
	"periph.io/x/conn/v3/spi/spireg"
	"periph.io/x/host/v3"
)

// Pins and SPI port as wired on the Waveshare e-Paper HAT, using BCM numbering.
// Chip select (GPIO8) is driven by the SPI port itself.
const (
	spiPort  = "SPI0.0"
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

	if err = busy.In(gpio.Float, gpio.NoEdge); err != nil {
		port.Close()
		return nil, nil, fmt.Errorf("failed to set up %s: %w", busyPin, err)
	}

	return New(bus, reset, dc, busy), port, nil
}
