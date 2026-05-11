# Go APDS-9960

Go driver for the APDS-9960 RGB, gesture, proximity, and ambient light sensor.

This is a modified Go port of the original [Adafruit APDS-9960 C++ library](https://github.com/adafruit/Adafruit_APDS9960).

## Requirements

- Go 1.25
- [periph.io](https://periph.io/) for GPIO/I2C access

## Installation

```bash
go mod download
```

## Examples

```bash
make
./example-color
./example-proximity
./example-gesture
./example-gesture-int
```

## Gesture sensor

This library uses a different algorithm for gesture detection compared to the original upstream library.
While testing with the original algorithm, the sensor had a stronger bias for detecting UP gesture while this approach
results in more balanced results. 

## License

The library is licensed under MIT.
This project follows the [REUSE specification](https://reuse.software) for license and copyright notices.
