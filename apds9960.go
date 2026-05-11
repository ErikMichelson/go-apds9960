// SPDX-FileCopyrightText: 2026 Erik Michelson
//
// SPDX-License-Identifier: MIT

package apds9960

import (
	"errors"
	"time"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/i2c"
)

const I2CAddr = 0x39

const (
	GESTURE_NONE = iota
	GESTURE_UP
	GESTURE_DOWN
	GESTURE_LEFT
	GESTURE_RIGHT
)

type Gesture int

func (g Gesture) String() string {
	switch g {
	case GESTURE_UP:
		return "UP"
	case GESTURE_DOWN:
		return "DOWN"
	case GESTURE_LEFT:
		return "LEFT"
	case GESTURE_RIGHT:
		return "RIGHT"
	default:
		return "NONE"
	}
}

type Options struct {
	IntPin gpio.PinIO
}

type APDS9960 struct {
	dev   *i2c.Dev
	intPin gpio.PinIO

	gestureEnabled bool
	proximityEnabled bool
	colorEnabled bool
	gestureIn bool
	gestureDirectionX int
	gestureDirectionY int
	gestureDirInX int
	gestureDirInY int
	gestureSensitivity int
	detectedGesture Gesture
}

func New(bus i2c.Bus, opts *Options) (*APDS9960, error) {
	if opts == nil {
		opts = &Options{}
	}
	d := &i2c.Dev{Addr: I2CAddr, Bus: bus}
	return &APDS9960{
		dev: d,
		intPin: opts.IntPin,
		gestureSensitivity: 50,
		detectedGesture: GESTURE_NONE,
	}, nil
}

func (a *APDS9960) Begin() error {
	var id uint8
	if err := a.getID(&id); err != nil {
		return err
	}
	if id != 0xAB {
		return errors.New("invalid device id")
	}

	if err := a.setENABLE(0x00); err != nil {
		return err
	}
	if err := a.setWTIME(0xFF); err != nil {
		return err
	}
	if err := a.setGPULSE(0x8F); err != nil {
		return err
	}
	if err := a.setPPULSE(0x8F); err != nil {
		return err
	}
	if err := a.setGestureIntEnable(true); err != nil {
		return err
	}
	if err := a.setGestureMode(true); err != nil {
		return err
	}
	if err := a.enablePower(); err != nil {
		return err
	}
	if err := a.enableWait(); err != nil {
		return err
	}

	atime := uint8(256 - (10 * 100 / 278))
	if err := a.setATIME(atime); err != nil {
		return err
	}

	if err := a.setCONTROL(0x02); err != nil {
		return err
	}

	time.Sleep(10 * time.Millisecond)

	if err := a.enablePower(); err != nil {
		return err
	}

	if err := a.enableProximity(); err != nil {
		return err
	}

	if err := a.enableGesture(); err != nil {
		return err
	}

	if a.intPin != nil {
		if err := a.intPin.In(gpio.PullUp, gpio.NoEdge); err != nil {
			return err
		}
	}

	return nil
}

func (a *APDS9960) End() error {
	if err := a.setENABLE(0x00); err != nil {
		return err
	}
	a.gestureEnabled = false
	return nil
}

func (a *APDS9960) SetLEDBoost(boost uint8) error {
	var r uint8
	if err := a.getCONFIG2(&r); err != nil {
		return err
	}
	r &= 0b11001111
	r |= (boost << 4) & 0b00110000
	return a.setCONFIG2(r)
}

func (a *APDS9960) SetGestureSensitivity(sensitivity uint8) {
	if sensitivity > 100 {
		sensitivity = 100
	}
	a.gestureSensitivity = int(100 - sensitivity)
}

func (a *APDS9960) GestureAvailable() bool {
	if !a.gestureEnabled {
		if err := a.enableGesture(); err != nil {
			return false
		}
	}

	if a.intPin != nil {
		level := a.intPin.Read()
		if level != gpio.Low {
			return false
		}
	} else {
		avail := a.gestureFIFOAvailable()
		if avail <= 0 {
			return false
		}
	}

	a.handleGesture()
	if a.proximityEnabled {
		_ = a.setGestureMode(false)
	}

	return a.detectedGesture != GESTURE_NONE
}

func (a *APDS9960) ReadGesture() Gesture {
	g := a.detectedGesture
	a.detectedGesture = GESTURE_NONE
	return g
}

func (a *APDS9960) ColorAvailable() bool {
	if err := a.enableColor(); err != nil {
		return false
	}

	var r uint8
	if err := a.getSTATUS(&r); err != nil {
		return false
	}

	return (r & 0b00000001) != 0
}

func (a *APDS9960) ReadColor() (r, g, b, c int, err error) {
	colors := make([]byte, 8)
	if n := a.readCDATAL(colors); n != 8 {
		return -1, -1, -1, -1, errors.New("failed to read color data")
	}

	c = int(uint16(colors[1])<<8 | uint16(colors[0]))
	r = int(uint16(colors[3])<<8 | uint16(colors[2]))
	g = int(uint16(colors[5])<<8 | uint16(colors[4]))
	b = int(uint16(colors[7])<<8 | uint16(colors[6]))

	_ = a.disableColor()

	return r, g, b, c, nil
}

func (a *APDS9960) ReadColorNormalized() (r, g, b, c float64, err error) {
	ri, gi, bi, ci, err := a.ReadColor()
	if err != nil {
		return 0, 0, 0, 0, err
	}
	max := 3072.0 // (256 - ATIME) * 1024, ATIME=253 -> max=3072
	if ri > 0 {
		r = float64(ri) / max
	}
	if gi > 0 {
		g = float64(gi) / max
	}
	if bi > 0 {
		b = float64(bi) / max
	}
	if ci > 0 {
		c = float64(ci) / max
	}
	return r, g, b, c, nil
}

func (a *APDS9960) ProximityAvailable() bool {
	if err := a.enableProximity(); err != nil {
		return false
	}

	var r uint8
	if err := a.getSTATUS(&r); err != nil {
		return false
	}

	return (r & 0b00000010) != 0
}

func (a *APDS9960) ReadProximity() (int, error) {
	var r uint8
	if err := a.getPDATA(&r); err != nil {
		return -1, err
	}

	_ = a.disableProximity()

	return int(255 - r), nil
}

func (a *APDS9960) setGestureIntEnable(en bool) error {
	var r uint8
	if err := a.getGCONF4(&r); err != nil {
		return err
	}
	if en {
		r |= 0b00000010
	} else {
		r &= 0b11111101
	}
	return a.setGCONF4(r)
}

func (a *APDS9960) setGestureMode(en bool) error {
	var r uint8
	if err := a.getGCONF4(&r); err != nil {
		return err
	}
	if en {
		r |= 0b00000001
	} else {
		r &= 0b11111110
	}
	return a.setGCONF4(r)
}

func (a *APDS9960) enablePower() error {
	var r uint8
	if err := a.getENABLE(&r); err != nil {
		return err
	}
	if (r & 0b00000001) != 0 {
		return nil
	}
	r |= 0b00000001
	return a.setENABLE(r)
}

func (a *APDS9960) disablePower() error {
	var r uint8
	if err := a.getENABLE(&r); err != nil {
		return err
	}
	if (r & 0b00000001) == 0 {
		return nil
	}
	r &= 0b11111110
	return a.setENABLE(r)
}

func (a *APDS9960) enableColor() error {
	var r uint8
	if err := a.getENABLE(&r); err != nil {
		return err
	}
	if (r & 0b00000010) != 0 {
		a.colorEnabled = true
		return nil
	}
	r |= 0b00000010
	res := a.setENABLE(r)
	a.colorEnabled = (res == nil)
	return res
}

func (a *APDS9960) disableColor() error {
	var r uint8
	if err := a.getENABLE(&r); err != nil {
		return err
	}
	if (r & 0b00000010) == 0 {
		a.colorEnabled = false
		return nil
	}
	r &= 0b11111101
	res := a.setENABLE(r)
	if res != nil {
		a.colorEnabled = true
	} else {
		a.colorEnabled = false
	}
	return res
}

func (a *APDS9960) enableProximity() error {
	var r uint8
	if err := a.getENABLE(&r); err != nil {
		return err
	}
	if (r & 0b00000100) != 0 {
		a.proximityEnabled = true
		return nil
	}
	r |= 0b00000100
	res := a.setENABLE(r)
	a.proximityEnabled = (res == nil)
	return res
}

func (a *APDS9960) disableProximity() error {
	var r uint8
	if err := a.getENABLE(&r); err != nil {
		return err
	}
	if (r & 0b00000100) == 0 {
		a.proximityEnabled = false
		return nil
	}
	r &= 0b11111011
	res := a.setENABLE(r)
	if res != nil {
		a.proximityEnabled = true
	} else {
		a.proximityEnabled = false
	}
	return res
}

func (a *APDS9960) enableWait() error {
	var r uint8
	if err := a.getENABLE(&r); err != nil {
		return err
	}
	if (r & 0b00001000) != 0 {
		return nil
	}
	r |= 0b00001000
	return a.setENABLE(r)
}

func (a *APDS9960) disableWait() error {
	var r uint8
	if err := a.getENABLE(&r); err != nil {
		return err
	}
	if (r & 0b00001000) == 0 {
		return nil
	}
	r &= 0b11110111
	return a.setENABLE(r)
}

func (a *APDS9960) enableGesture() error {
	var r uint8
	if err := a.getENABLE(&r); err != nil {
		return err
	}
	if (r & 0b01000000) != 0 {
		a.gestureEnabled = true
		return nil
	}
	r |= 0b01000000
	res := a.setENABLE(r)
	a.gestureEnabled = (res == nil)
	return res
}

func (a *APDS9960) disableGesture() error {
	var r uint8
	if err := a.getENABLE(&r); err != nil {
		return err
	}
	if (r & 0b01000000) == 0 {
		a.gestureEnabled = true
		return nil
	}
	r &= 0b10111111
	res := a.setENABLE(r)
	if res != nil {
		a.gestureEnabled = true
	} else {
		a.gestureEnabled = false
	}
	return res
}

func (a *APDS9960) gestureFIFOAvailable() int {
	var r uint8
	if err := a.getGSTATUS(&r); err != nil {
		return -1
	}
	if (r & 0x01) == 0x00 {
		return -2
	}
	if err := a.getGFLVL(&r); err != nil {
		return -3
	}
	return int(r)
}

func (a *APDS9960) handleGesture() {
	gestureThreshold := 30
	for {
		available := a.gestureFIFOAvailable()
		if available <= 0 {
			return
		}

		fifoData := make([]uint8, available*4)
		bytesRead := a.readGFIFO_U(fifoData)
		if bytesRead == 0 {
			return
		}

		var sawHigh bool
		var prevU int

		for i := 0; i+3 < int(bytesRead); i += 4 {
			u := int(fifoData[i])
			d := int(fifoData[i+1])
			l := int(fifoData[i+2])
			r := int(fifoData[i+3])

			aboveThreshold := u >= gestureThreshold || d >= gestureThreshold ||
				l >= gestureThreshold || r >= gestureThreshold

			dropping := prevU > 0 && u < prevU-20

			dirX := r - l
			dirY := u - d

			if aboveThreshold {
				if !sawHigh {
					a.gestureDirInX = dirX
					a.gestureDirInY = dirY
				}
				sawHigh = true
			}

			if dropping && sawHigh {
				avgX := a.gestureDirInX
				avgY := a.gestureDirInY

				absX := avgX
				if absX < 0 {
					absX = -absX
				}
				absY := avgY
				if absY < 0 {
					absY = -absY
				}

				if absX > absY && absX > a.gestureSensitivity {
					if avgX < 0 {
						a.detectedGesture = GESTURE_RIGHT
					} else {
						a.detectedGesture = GESTURE_LEFT
					}
				} else if absY > a.gestureSensitivity {
					if avgY < 0 {
						a.detectedGesture = GESTURE_UP
					} else {
						a.detectedGesture = GESTURE_DOWN
					}
				}

				a.gestureDirInX = 0
				a.gestureDirInY = 0
				sawHigh = false
				prevU = 0
				continue
			}
			prevU = u
		}
	}
}

func (a *APDS9960) write(reg, val uint8) error {
	return a.dev.Tx([]byte{reg, val}, nil)
}

func (a *APDS9960) read(reg uint8) (uint8, error) {
	buf := make([]byte, 1)
	if err := a.dev.Tx([]byte{reg}, buf); err != nil {
		return 0, err
	}
	return buf[0], nil
}

func (a *APDS9960) readBlock(reg uint8, val []byte) int {
	if err := a.dev.Tx([]byte{reg}, val); err != nil {
		return 0
	}
	return len(val)
}

func (a *APDS9960) getENABLE(val *uint8) error {
	v, err := a.read(0x80)
	if err != nil {
		return err
	}
	*val = v
	return nil
}

func (a *APDS9960) setENABLE(val uint8) error {
	return a.write(0x80, val)
}

func (a *APDS9960) getATIME(val *uint8) error {
	v, err := a.read(0x81)
	if err != nil {
		return err
	}
	*val = v
	return nil
}

func (a *APDS9960) setATIME(val uint8) error {
	return a.write(0x81, val)
}

func (a *APDS9960) getWTIME(val *uint8) error {
	v, err := a.read(0x83)
	if err != nil {
		return err
	}
	*val = v
	return nil
}

func (a *APDS9960) setWTIME(val uint8) error {
	return a.write(0x83, val)
}

func (a *APDS9960) getPERS(val *uint8) error {
	v, err := a.read(0x8C)
	if err != nil {
		return err
	}
	*val = v
	return nil
}

func (a *APDS9960) setPERS(val uint8) error {
	return a.write(0x8C, val)
}

func (a *APDS9960) getCONFIG1(val *uint8) error {
	v, err := a.read(0x8D)
	if err != nil {
		return err
	}
	*val = v
	return nil
}

func (a *APDS9960) setCONFIG1(val uint8) error {
	return a.write(0x8D, val)
}

func (a *APDS9960) getPPULSE(val *uint8) error {
	v, err := a.read(0x8E)
	if err != nil {
		return err
	}
	*val = v
	return nil
}

func (a *APDS9960) setPPULSE(val uint8) error {
	return a.write(0x8E, val)
}

func (a *APDS9960) getCONTROL(val *uint8) error {
	v, err := a.read(0x8F)
	if err != nil {
		return err
	}
	*val = v
	return nil
}

func (a *APDS9960) setCONTROL(val uint8) error {
	return a.write(0x8F, val)
}

func (a *APDS9960) getCONFIG2(val *uint8) error {
	v, err := a.read(0x90)
	if err != nil {
		return err
	}
	*val = v
	return nil
}

func (a *APDS9960) setCONFIG2(val uint8) error {
	return a.write(0x90, val)
}

func (a *APDS9960) getID(val *uint8) error {
	v, err := a.read(0x92)
	if err != nil {
		return err
	}
	*val = v
	return nil
}

func (a *APDS9960) getSTATUS(val *uint8) error {
	v, err := a.read(0x93)
	if err != nil {
		return err
	}
	*val = v
	return nil
}

func (a *APDS9960) readCDATAL(val []byte) int {
	buf := make([]byte, 8)
	if n := a.readBlock(0x94, buf); n != 8 {
		return n
	}
	val[0] = buf[0]
	val[1] = buf[1]
	val[2] = buf[2]
	val[3] = buf[3]
	val[4] = buf[4]
	val[5] = buf[5]
	val[6] = buf[6]
	val[7] = buf[7]
	return 8
}

func (a *APDS9960) readRDATAL(val []byte) int {
	return a.readBlock(0x96, val)
}

func (a *APDS9960) readGDATAL(val []byte) int {
	return a.readBlock(0x98, val)
}

func (a *APDS9960) readBDATAL(val []byte) int {
	return a.readBlock(0x9A, val)
}

func (a *APDS9960) getPDATA(val *uint8) error {
	v, err := a.read(0x9C)
	if err != nil {
		return err
	}
	*val = v
	return nil
}

func (a *APDS9960) getCONFIG3(val *uint8) error {
	v, err := a.read(0x9F)
	if err != nil {
		return err
	}
	*val = v
	return nil
}

func (a *APDS9960) setCONFIG3(val uint8) error {
	return a.write(0x9F, val)
}

func (a *APDS9960) getGPENTH(val *uint8) error {
	v, err := a.read(0xA0)
	if err != nil {
		return err
	}
	*val = v
	return nil
}

func (a *APDS9960) setGPENTH(val uint8) error {
	return a.write(0xA0, val)
}

func (a *APDS9960) getGEXTH(val *uint8) error {
	v, err := a.read(0xA1)
	if err != nil {
		return err
	}
	*val = v
	return nil
}

func (a *APDS9960) setGEXTH(val uint8) error {
	return a.write(0xA1, val)
}

func (a *APDS9960) getGCONF1(val *uint8) error {
	v, err := a.read(0xA2)
	if err != nil {
		return err
	}
	*val = v
	return nil
}

func (a *APDS9960) setGCONF1(val uint8) error {
	return a.write(0xA2, val)
}

func (a *APDS9960) getGCONF2(val *uint8) error {
	v, err := a.read(0xA3)
	if err != nil {
		return err
	}
	*val = v
	return nil
}

func (a *APDS9960) setGCONF2(val uint8) error {
	return a.write(0xA3, val)
}

func (a *APDS9960) getGOFFSET_U(val *uint8) error {
	v, err := a.read(0xA4)
	if err != nil {
		return err
	}
	*val = v
	return nil
}

func (a *APDS9960) setGOFFSET_U(val uint8) error {
	return a.write(0xA4, val)
}

func (a *APDS9960) getGOFFSET_D(val *uint8) error {
	v, err := a.read(0xA5)
	if err != nil {
		return err
	}
	*val = v
	return nil
}

func (a *APDS9960) setGOFFSET_D(val uint8) error {
	return a.write(0xA5, val)
}

func (a *APDS9960) getGPULSE(val *uint8) error {
	v, err := a.read(0xA6)
	if err != nil {
		return err
	}
	*val = v
	return nil
}

func (a *APDS9960) setGPULSE(val uint8) error {
	return a.write(0xA6, val)
}

func (a *APDS9960) getGOFFSET_L(val *uint8) error {
	v, err := a.read(0xA7)
	if err != nil {
		return err
	}
	*val = v
	return nil
}

func (a *APDS9960) setGOFFSET_L(val uint8) error {
	return a.write(0xA7, val)
}

func (a *APDS9960) getGOFFSET_R(val *uint8) error {
	v, err := a.read(0xA9)
	if err != nil {
		return err
	}
	*val = v
	return nil
}

func (a *APDS9960) setGOFFSET_R(val uint8) error {
	return a.write(0xA9, val)
}

func (a *APDS9960) getGCONF3(val *uint8) error {
	v, err := a.read(0xAA)
	if err != nil {
		return err
	}
	*val = v
	return nil
}

func (a *APDS9960) setGCONF3(val uint8) error {
	return a.write(0xAA, val)
}

func (a *APDS9960) getGCONF4(val *uint8) error {
	v, err := a.read(0xAB)
	if err != nil {
		return err
	}
	*val = v
	return nil
}

func (a *APDS9960) setGCONF4(val uint8) error {
	return a.write(0xAB, val)
}

func (a *APDS9960) getGFLVL(val *uint8) error {
	v, err := a.read(0xAE)
	if err != nil {
		return err
	}
	*val = v
	return nil
}

func (a *APDS9960) getGSTATUS(val *uint8) error {
	v, err := a.read(0xAF)
	if err != nil {
		return err
	}
	*val = v
	return nil
}

func (a *APDS9960) readGFIFO_U(val []byte) int {
	return a.readBlock(0xFC, val)
}
