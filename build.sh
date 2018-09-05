#!/bin/bash

set -e

function generate_cert {
    ./generate-cert.sh
}

function clean_proto {
    rm -f proto/client/* proto/server/* 
}

function clean_static {
    rm -f frontend/static/*
}

function build_proto {
    # the /tmp path is for CI only
    protoc -I. -I/tmp/protobuf/include -Ivendor/ ./proto/web.proto \
        --go_out=plugins=grpc:$GOPATH/src
}

function build_static {
    mkdir -p frontend/static
    # Do the static frontend build, unless if parcel is running.
    # We need the -f option here, since parcel is a node process.
    if ! pgrep -f "parcel" > /dev/null ; then
        # we could pass --public-url to parcel build
        echo "do a parcel build"
        (cd ui && parcel build)
    fi
    cp ui/dist/* frontend/static/
    (cd frontend && go run assets_generate.go)
}

function build_all {

    if [ ! -f ./key.pem ]; then
        generate_cert
    fi

    clean_proto
    build_proto

    clean_static
    build_static
    go build 
}

case "$1" in
    all)
        build_all 
    ;;
    clean)
        clean_static
        clean_proto
    ;;
    static)
        clean_static
        build_static
    ;;
    proto)
        clean_proto
        build_proto
    ;;
    help)
        echo "Usage:"
        echo "    ./build.sh [help|clean|all|static|proto]"
    ;;
    *)
        # TODO: build_minimal
        build_all
    ;;

esac
