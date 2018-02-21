#!/bin/bash

set -e

echo "cleaning..."
rm -f proto/client/* proto/server/* \
    frontend/html/frontend.js frontend/html/frontend.js.map

echo "generating code from proto files..."
protoc -I. -I/tmp/protobuf/include -Ivendor/ ./proto/web.proto \
    --gopherjs_out=plugins=grpc:${GOPATH}/src \
    --go_out=plugins=grpc:$GOPATH/src

echo "go generate..."
go generate ./frontend/

go build -race
