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
mv sys.exe ../builds/
if [ $? -ne 0 ]; then
    echo "Failed to move Windows binary" >&2
    exit 1
fi

# Build for Linux
echo "Building for Linux..."
GOOS=linux GOARCH=amd64 go build -o sys
if [ $? -ne 0 ]; then
    echo "Failed to build for Linux" >&2
    exit 1
fi

# Move the Linux binary
mv sys ../builds/
if [ $? -ne 0 ]; then
    echo "Failed to move Linux binary" >&2
    exit 1
fi

echo "Builds completed successfully!"
