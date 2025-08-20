APP_NAME=backup-helper

.PHONY: all build install uninstall clean test version

all: build

build:
	go build -a -o $(APP_NAME) main.go

# 显示当前版本
version:
	@./version.sh show

# 设置新版本
set-version:
	@if [ -z "$(VER)" ]; then \
		echo "Usage: make set-version VER=1.0.1"; \
		exit 1; \
	fi
	@./version.sh set $(VER)

# 获取当前版本号
get-version:
	@./version.sh get

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