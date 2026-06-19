.DEFAULT_GOAL := build

PLUGIN_NAME := response-reference
BUILD_DIR := bin
GO_VERSION := 1.24

linux-amd64:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(BUILD_DIR)/linux-amd64/$(PLUGIN_NAME) .

linux: linux-amd64

android-arm64:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o $(BUILD_DIR)/android-arm64/$(PLUGIN_NAME) .

android-arm:
	GOOS=linux GOARCH=arm CGO_ENABLED=0 go build -o $(BUILD_DIR)/android-arm/$(PLUGIN_NAME) .

android: android-arm64 android-arm

darwin-amd64:
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -o $(BUILD_DIR)/darwin-amd64/$(PLUGIN_NAME) .

darwin-arm64:
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o $(BUILD_DIR)/darwin-arm64/$(PLUGIN_NAME) .

darwin: darwin-amd64 darwin-arm64

windows-amd64:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o $(BUILD_DIR)/windows-amd64/$(PLUGIN_NAME).exe .

windows: windows-amd64

all: linux android darwin windows

build:
	CGO_ENABLED=0 go build -o $(PLUGIN_NAME) .

deps:
	go mod tidy

fmt:
	go fmt ./...

vet:
	go vet ./...

test:
	go test -v ./...

clean:
	rm -rf $(BUILD_DIR)
	rm -f $(PLUGIN_NAME)
	rm -f $(PLUGIN_NAME).exe

.PHONY: all build clean deps fmt vet test linux linux-amd64 android android-arm64 android-arm darwin darwin-amd64 darwin-arm64 windows windows-amd64
