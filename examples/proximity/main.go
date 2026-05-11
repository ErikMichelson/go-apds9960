// SPDX-FileCopyrightText: 2026 Erik Michelson
//
// SPDX-License-Identifier: MIT

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

	fmt.Println("Proximity sensor started. Press Ctrl+C to exit.")
	fmt.Println()

	for {
		if sensor.ProximityAvailable() {
			p, err := sensor.ReadProximity()
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			} else {
				fmt.Printf("Proximity: %d\n", p)
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
}
