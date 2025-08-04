#!/bin/bash

# Build script for Linux cross-compilation using Docker

set -e

echo "Building Loom GUI for Linux using Docker..."

# Build the Docker image
docker build -f Dockerfile.linux -t loom-gui-linux-builder .

# Create output directory
mkdir -p build/bin/linux

# Run the build and copy artifacts
docker run --rm -v $(pwd)/build/bin/linux:/artifacts loom-gui-linux-builder

echo "Linux builds completed!"
echo "Artifacts available in: build/bin/linux/"
ls -la build/bin/linux/