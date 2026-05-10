.PHONY: all clean

all: example-color example-proximity example-gesture example-gesture-int

example-color:
	go build -o example-color ./examples/color

example-proximity:
	go build -o example-proximity ./examples/proximity

example-gesture:
	go build -o example-gesture ./examples/gesture

example-gesture-int:
	go build -o example-gesture-int ./examples/gesture_interrupt

clean:
	rm -f example-color example-proximity example-gesture example-gesture-int