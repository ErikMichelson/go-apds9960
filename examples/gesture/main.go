package main

import (
	"fmt"
	"log"
	"time"

	"github.com/ErikMichelson/go-apds9960"
	"periph.io/x/conn/v3/driver/driverreg"
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

	sensor, err := apds9960.New(bus, nil)
	if err != nil {
		log.Fatalf("failed to create APDS9960: %v", err)
	}

	if err := sensor.Begin(); err != nil {
		log.Fatalf("failed to begin APDS9960: %v", err)
	}
	defer sensor.End()

	fmt.Println("Gesture sensor started. Swipe over sensor. Press Ctrl+C to exit.")
	fmt.Println("Gestures: UP, DOWN, LEFT, RIGHT")
	fmt.Println()

	runNormal(sensor)
}

func runNormal(sensor *apds9960.APDS9960) {
	for {
		if sensor.GestureAvailable() {
			g := sensor.ReadGesture()
			fmt.Printf("Gesture: %s\n", g)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func runDebug(sensor *apds9960.APDS9960) {
	for {
		avail := sensor.GestureFIFOAvailableDebug()
		fmt.Printf("FIFO available: %d\n", avail)
		if avail > 0 {
			data := sensor.ReadGestureFIFODebug(avail)
			fmt.Printf("FIFO data (%d bytes): %v\n", len(data), data)
		}
		sensor.DumpGestureRegisters()
		fmt.Println("---")
		time.Sleep(100 * time.Millisecond)
	}
}
