#!/bin/bash

# Build script for high concurrency version

echo "Building high concurrency version..."

# Temporarily rename main files
mv main.go main_v1.go.bak
mv main_v2.go main.go

# Build
go build -o load-test-client-v2 .

# Restore original
mv main.go main_v2.go
mv main_v1.go.bak main.go

echo "Build complete: ./load-test-client-v2"
echo ""
echo "To run with 10K concurrent users:"
echo "  export \$(cat .env | xargs)"
echo "  export CONCURRENT_WRITERS=10000"
echo "  export READ_PERCENT=60"
echo "  export INSERT_PERCENT=20"
echo "  export UPDATE_PERCENT=20"
echo "  ./load-test-client-v2"
