APP=mahcdown
BINDIR?=bin
INSTALL_DIR?=/usr/local/bin

.PHONY: build run install uninstall lint staticcheck gosec test tidy clean

build:
	mkdir -p $(BINDIR)
	go build -o $(BINDIR)/$(APP) ./cmd/mahcdown

run:
	go run ./cmd/mahcdown $(ARGS)

install: build
	mkdir -p $(INSTALL_DIR)
	install -m 0755 $(BINDIR)/$(APP) $(INSTALL_DIR)/$(APP)

uninstall:
	rm -f $(INSTALL_DIR)/$(APP)

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
	rm -f $(BINDIR)/$(APP)
