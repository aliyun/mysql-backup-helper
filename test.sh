#!/bin/bash

cd "$(dirname "$0")" || { echo "无法进入目录"; exit 1; }

git checkout develop || { echo "切换分支失败"; exit 1; }

# 获取远程更新，并判断是否有变更
git fetch origin
LOCAL=$(git rev-parse HEAD)
REMOTE=$(git rev-parse origin/develop)

if [ "$LOCAL" != "$REMOTE" ]; then
    echo "检测到代码更新，开始 pull 并重新构建..."
    git pull
    go build -a -o backup-helper main.go
else
    echo "没有代码更新，跳过构建。"
fi

if [ ! -f "./backup-helper" ]; then
    echo "错误：backup-helper 不存在，请确认之前构建成功"
    exit 1
fi

echo "========== 测试场景1：中文，OSS，qpress压缩 =========="
LANG=zh_CN.UTF-8 ./backup-helper --config config.json --backup --mode=oss --compress-type=qp

echo "========== 测试场景2：英文，OSS，zstd压缩 =========="
LANG=en_US.UTF-8 ./backup-helper --config config.json --backup --mode=oss --compress-type=zstd

echo "========== 测试场景3：中文，OSS，无压缩 =========="
LANG=zh_CN.UTF-8 ./backup-helper --config config.json --backup --mode=oss --compress=false

echo "========== 测试场景4：英文，stream模式，qpress压缩 =========="
LANG=en_US.UTF-8 ./backup-helper --config config.json --backup --mode=stream --stream-port=9999 &

sleep 2
echo "========== 测试场景4：本地拉流 =========="
# 这里假设你有 nc 或 socat 客户端拉流
(echo "FROM RDS: START";) | nc localhost 9999 > /tmp/streamed-backup.xb

# 等待 stream 结束
wait

echo "========== 测试场景5：无 --backup，仅参数检查 =========="
LANG=zh_CN.UTF-8 ./backup-helper --config config.json

echo "========== 测试场景6：无 config，命令行参数 =========="
LANG=en_US.UTF-8 ./backup-helper --host=127.0.0.1 --user=root --password=123456 --port=3306 --backup --mode=oss --compress-type=qp

# ========== AI诊断相关 ==========
echo "========== 测试场景7：AI诊断自动（--ai-diagnose=on，需配置QwenAPIKey，建议用错误参数触发） =========="
LANG=zh_CN.UTF-8 ./backup-helper --config config.json --backup --mode=oss --compress-type=qp --ai-diagnose=on --user=wronguser

echo "========== 测试场景8：AI诊断关闭（--ai-diagnose=off，失败时不触发AI） =========="
LANG=zh_CN.UTF-8 ./backup-helper --config config.json --backup --mode=oss --compress-type=qp --ai-diagnose=off --user=wronguser

echo "========== 测试场景9：AI诊断交互（不加--ai-diagnose，失败时手动输入y/n） =========="
LANG=zh_CN.UTF-8 ./backup-helper --config config.json --backup --mode=oss --compress-type=qp --user=wronguser

echo "========== 所有测试完成 ==========" 