#!/bin/bash

# Script to run Loom app with debug options
# This will enable Chrome DevTools and better error logging

echo "Starting Loom with debug options..."

# Check if the app exists
APP_PATH="./build/bin/Loom.app"
if [ ! -d "$APP_PATH" ]; then
    echo "Error: Loom.app not found at $APP_PATH"
    echo "Please build the app first with: make build-macos"
    exit 1
fi

# Set environment variables for debugging
export WAILS_DEBUG=1
export WAILS_LOGGING=debug

# Run the app with debug options
echo "Launching Loom with debug mode..."
echo "Chrome DevTools should open automatically in your browser"
echo "Press Ctrl+C to stop the app"

# On macOS, we can use the open command with specific arguments
# The debug mode should automatically open Chrome DevTools
open "$APP_PATH"

echo "App launched. Check your browser for Chrome DevTools."
echo "If DevTools don't open automatically, you can manually open:"
echo "chrome://inspect/#devices"
echo "Then click 'Open dedicated DevTools for Node'" 