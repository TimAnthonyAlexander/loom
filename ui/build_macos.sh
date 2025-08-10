#!/bin/bash
set -e

echo "Building Loom for macOS..."

# Check if Wails CLI is installed and install if needed
echo "Ensuring Wails CLI is installed..."
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# Define the wails binary path
WAILS_BIN="$HOME/go/bin/wails"
if [ ! -f "$WAILS_BIN" ]; then
    GOPATH=$(go env GOPATH)
    WAILS_BIN="$GOPATH/bin/wails"
fi

echo "Using Wails binary at: $WAILS_BIN"

# Navigate to the frontend directory and build assets
echo "Building frontend assets..."
cd frontend
npm install
npm run build
cd ..

# Build the macOS .app bundle
echo "Building macOS application bundle..."
"$WAILS_BIN" build -platform darwin/universal -clean

echo "Build complete! The .app bundle is in the build/bin directory."