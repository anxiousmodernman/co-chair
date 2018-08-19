#!/bin/bash

set -e

echo "cleaning..."
rm -f proto/client/* proto/server/* \
    frontend/html/frontend.js frontend/html/frontend.js.map

echo "generating code from proto files..."
# the /tmp path is for CI only
protoc -I. -I/tmp/protobuf/include -Ivendor/ ./proto/web.proto \
    --go_out=plugins=grpc:$GOPATH/src


# do optimized js build
(
    cd ui
    npm run build
)

# Remove the compiled frontend
rm -rf frontend/static
cp -r ui/build frontend/static

# Generate 
echo "go generate..."
( 
    cd frontend
    go run assets_generate.go
)

go build 
