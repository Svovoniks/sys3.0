#!/bin/bash

# Navigate to the source folder
cd src || { echo "Failed to navigate to src directory" >&2; exit 1; }

# Create builds directory if it doesn't exist
mkdir -p ../builds

# Build for Windows
echo "Building for Windows..."
GOOS=windows GOARCH=amd64 go build -o sys.exe
if [ $? -ne 0 ]; then
    echo "Failed to build for Windows" >&2
    exit 1
fi

# Move the Windows binary
mkdir -p ../builds/windows
mv sys.exe ../builds/windows/sys.exe
if [ $? -ne 0 ]; then
    echo "Failed to move Windows binary" >&2
    exit 1
fi

# Build for Linux
echo "Building for Linux..."
GOOS=linux GOARCH=amd64 go build -o sys-linux
if [ $? -ne 0 ]; then
    echo "Failed to build for Linux" >&2
    exit 1
fi

# Move the Linux binary
mkdir -p ../builds/linux
mv sys-linux ../builds/linux/sys
if [ $? -ne 0 ]; then
    echo "Failed to move Linux binary" >&2
    exit 1
fi

# Build for Intel Macs (macOS on amd64)
echo "Building for Intel Mac (amd64)..."
GOOS=darwin GOARCH=amd64 go build -o sys-intel-mac
if [ $? -ne 0 ]; then
    echo "Failed to build for Intel Mac" >&2
    exit 1
fi

# Move the Intel Mac binary
mkdir -p ../builds/intel-mac
mv sys-intel-mac ../builds/intel-mac/sys
if [ $? -ne 0 ]; then
    echo "Failed to move Intel Mac binary" >&2
    exit 1
fi

# Build for M Mac (macOS on arm64)
echo "Building for M Mac (arm64)..."
GOOS=darwin GOARCH=arm64 go build -o sys-mac-m
if [ $? -ne 0 ]; then
    echo "Failed to build for M Mac" >&2
    exit 1
fi

# Move the M Mac binary
mkdir -p ../builds/m-mac
mv sys-mac-m ../builds/m-mac/sys
if [ $? -ne 0 ]; then
    echo "Failed to move M Mac binary" >&2
    exit 1
fi

echo "Builds completed successfully!"
