GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
TARGET=wharfie
SRC=$(shell find . -type f -name '*.go' -not -path "./vendor/*")

.PHONY: all test build

all: test build

test: $(SRC)
	$(GOTEST) -v ./...

build: $(TARGET)
	@true

clean:
	$(GOCLEAN)
	rm -f $(TARGET)

$(TARGET): $(SRC)
	$(GOBUILD) -o $(TARGET) -v -ldflags "-X main.version=`git describe --tags --dirty`"
