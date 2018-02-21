#!/bin/bash

# install 
go install ./vendor/github.com/golang/protobuf/protoc-gen-go \
    ./vendor/github.com/johanbrandhorst/protobuf/protoc-gen-gopherjs

# GopherJS cannot be vendored so must be fetched
go get -u github.com/gopherjs/gopherjs