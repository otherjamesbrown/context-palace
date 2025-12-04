BINARY=cxp
VERSION?=0.1.0

.PHONY: build test clean install

build:
	go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY) ./cmd/cxp

test:
	go test -v ./...

clean:
	rm -f $(BINARY)

install: build
	mv $(BINARY) $(GOPATH)/bin/