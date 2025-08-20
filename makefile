APP_NAME=backup-helper

.PHONY: all build install uninstall clean test version set-version get-version resolved-version

all: build

# 解析版本优先级：Git 标签 > VERSION 文件 > 0.0.0
GIT_TAG := $(shell git describe --tags --abbrev=0 2>/dev/null)
ifeq ($(strip $(GIT_TAG)),)
  FILE_VERSION := $(shell test -f VERSION && cat VERSION || echo 0.0.0)
  RESOLVED_VERSION := $(FILE_VERSION)
else
  RESOLVED_VERSION := $(GIT_TAG)
endif

# 构建（嵌入版本，不依赖运行时文件）
build:
	@echo "Building $(APP_NAME) (version: $(RESOLVED_VERSION))..."
	@go build -a -ldflags="-X 'backup-helper/utils.BuildVersion=$(RESOLVED_VERSION)'" -o $(APP_NAME) main.go
	@echo "Build completed: $(APP_NAME)"

# 显示通过 Git 标签/文件解析出的版本
resolved-version:
	@echo $(RESOLVED_VERSION)

# 兼容的版本管理（仍可使用 VERSION 文件管理）
version:
	@./version.sh show

set-version:
	@if [ -z "$(VER)" ]; then \
		echo "Usage: make set-version VER=1.0.1"; \
		exit 1; \
	fi
	@./version.sh set $(VER)

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