APP_NAME=backup-helper

.PHONY: all build install uninstall clean test

all: build

build:
	go build -a -o $(APP_NAME) main.go

install: build
	@echo "Installing $(APP_NAME) to /usr/local/bin/ ..."
	sudo cp $(APP_NAME) /usr/local/bin/$(APP_NAME)
	@echo "You can now run '$(APP_NAME)' from anywhere."

uninstall:
	@echo "Removing $(APP_NAME) from /usr/local/bin/ ..."
	sudo rm -f /usr/local/bin/$(APP_NAME)

clean:
	rm -f $(APP_NAME)

test:
	bash ./test.sh