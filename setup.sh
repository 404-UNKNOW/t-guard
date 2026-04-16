#!/bin/bash

echo "🚀 T-Guard One-Click Initialization..."

# 1. Check Go Environment
if ! command -v go &> /dev/null
then
    echo "❌ Error: Go is not installed. Please install Go (https://go.dev/dl/)"
    exit 1
fi

# 2. Install Dependencies
echo "📦 Fetching dependencies..."
go mod tidy

# 3. Config Guide
if [ ! -f config.yaml ]; then
    echo "📄 Creating default config.yaml from example..."
    cp config.example.yaml config.yaml
    echo "✅ Success. Please edit config.yaml to add your API keys."
fi

# 4. Build
echo "🔨 Building T-Guard..."
go build -o t-guard main.go

if [ $? -eq 0 ]; then
    echo "------------------------------------------------"
    echo "🎉 Initialization Complete!"
    echo "👉 Run ./t-guard to start the real-time sentinel."
    echo "------------------------------------------------"
else
    echo "❌ Build failed. Please check your internet connection or Go environment."
fi
