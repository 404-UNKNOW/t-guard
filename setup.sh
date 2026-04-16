#!/bin/bash

echo "🚀 T-Guard 一键初始化启动..."

# 1. 检查 Go 环境
if ! command -v go &> /dev/null
then
    echo "❌ 错误: 未检测到 Go 语言环境，请先安装 Go (https://go.dev/dl/)"
    exit 1
fi

# 2. 安装依赖
echo "📦 正在获取依赖..."
go mod tidy

# 3. 配置文件引导
if [ ! -f config.yaml ]; then
    echo "📄 正在为您创建 config.yaml 默认配置文件..."
    cp config.example.yaml config.yaml
    echo "✅ 已生成。建议您稍后用记事本打开 config.yaml 修改 API Key。"
fi

# 4. 编译
echo "🔨 正在编译程序..."
go build -o t-guard main.go

if [ $? -eq 0 ]; then
    echo "------------------------------------------------"
    echo "🎉 初始化成功！"
    echo "👉 输入 ./t-guard 即可启动实时监控界面。"
    echo "------------------------------------------------"
else
    echo "❌ 编译失败，请检查网络或环境。"
fi
