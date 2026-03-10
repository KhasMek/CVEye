.PHONY: build run install clean

build:
	go build -o bin/cveye .

run:
	go run .

install:
	go install .

clean:
	rm -rf bin/
