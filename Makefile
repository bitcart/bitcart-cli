all: build

build:
	go build ${ARGS}

clean:
	rm -f bitcart-cli dist/bitcart-cli-*
