buildflags = GOOS=linux GOARCH=amd64 CGO_ENABLED=0

all: container

build:
	$(buildflags) go build .

container: build
	docker build -t async:latest .

.PHONY: build container
