package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ErikMichelson/go-apds9960"
	"periph.io/x/conn/v3/driver/driverreg"
	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/conn/v3/i2c/i2creg"
	"periph.io/x/host/v3"
)

func main() {
	if _, err := driverreg.Init(); err != nil {
		log.Fatal(err)
	}

	if _, err := host.Init(); err != nil {
		log.Fatal(err)
	}

	bus, err := i2creg.Open("")
	if err != nil {
		log.Fatalf("failed to open I2C bus: %v", err)
	}
	defer bus.Close()

	intPin := gpioreg.ByName("GPIO4")
	if intPin == nil {
		log.Fatal("failed to open GPIO4")
	}

	sensor, err := apds9960.New(bus, &apds9960.Options{
		IntPin: intPin,
	})
	if err != nil {
		log.Fatalf("failed to create APDS9960: %v", err)
	}

	if err := sensor.Begin(); err != nil {
		log.Fatalf("failed to begin APDS9960: %v", err)
	}
	defer sensor.End()

	fmt.Println("Gesture sensor started with interrupt pin (GPIO4)")
	fmt.Println("Swipe over sensor. Press Ctrl+C to exit.")
	fmt.Println("Gestures: UP, DOWN, LEFT, RIGHT")
	fmt.Println()

	if err := intPin.In(gpio.PullUp, gpio.FallingEdge); err != nil {
		log.Fatalf("failed to set interrupt mode: %v", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("Waiting for gesture interrupts...")

	for {
		select {
		case <-sigChan:
			fmt.Println("\nExiting...")
			return
		default:
			if intPin.WaitForEdge(10 * time.Millisecond) {
				for sensor.GestureAvailable() {
					g := sensor.ReadGesture()
					fmt.Printf("Gesture: %s\n", g)
				}
			}
		}
	}
}
