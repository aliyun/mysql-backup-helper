#!/bin/bash

# 版本管理脚本
VERSION_FILE="VERSION"

# 读取当前版本
get_version() {
    if [ -f "$VERSION_FILE" ]; then
        cat "$VERSION_FILE" | tr -d ' \t\n\r'
    else
        echo "0.0.0"
    fi
}

# 设置新版本
set_version() {
    local new_version="$1"
    if [ -z "$new_version" ]; then
        echo "Usage: $0 set <version>"
        echo "Example: $0 set 1.0.1"
        exit 1
    fi
    
    echo "$new_version" > "$VERSION_FILE"
    echo "Version updated to: $new_version"
}

# 显示当前版本
show_version() {
    echo "Current version: $(get_version)"
}

# 主函数
case "$1" in
    "get")
        get_version
        ;;
    "set")
        set_version "$2"
        ;;
    "show"|"")
        show_version
        ;;
    *)
        echo "Usage: $0 {get|set|show}"
        echo "  get          - Get current version"
        echo "  set <version> - Set new version"
        echo "  show         - Show current version (default)"
        exit 1
        ;;
esac
