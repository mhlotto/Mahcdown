APP=mahcdown
BINDIR=bin

.PHONY: build run lint staticcheck gosec test tidy clean

build:
	mkdir -p $(BINDIR)
	go build -o $(BINDIR)/$(APP) ./cmd/mahcdown

run:
	go run ./cmd/mahcdown $(ARGS)

lint:
	go vet ./...

staticcheck:
	staticcheck ./...

gosec:
	gosec ./...

test:
	go test ./...

tidy:
	go mod tidy

clean:
	rm -rf $(BINDIR)
