BIN_DIR   ?= $(HOME)/.local/bin
CLI_BIN    = sauce
GUI_BIN    = sauce-gui
GUI_TAGS   = webkit2_41

.PHONY: all cli gui install install-cli install-gui test clean

all: cli gui

cli:
	go build -o $(CLI_BIN) ./cmd/sauce

gui:
	cd gui && wails build -tags $(GUI_TAGS) -o $(GUI_BIN)

install: install-cli install-gui

install-cli: cli
	install -Dm755 $(CLI_BIN) $(BIN_DIR)/$(CLI_BIN)

install-gui: gui
	install -Dm755 gui/build/bin/$(GUI_BIN) $(BIN_DIR)/$(GUI_BIN)

test:
	go test ./...

clean:
	rm -f $(CLI_BIN)
	rm -rf gui/build
