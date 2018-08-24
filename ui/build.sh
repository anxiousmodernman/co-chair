#!/bin/bash

set -e


# generate typescript from protoc
PROTOC_GEN_TS_PATH="./node_modules/.bin/protoc-gen-ts"
 
# Directory to write generated code to (.js and .d.ts files)  
OUT_DIR="./generated"
 
protoc -I../proto \
    --plugin="protoc-gen-ts=${PROTOC_GEN_TS_PATH}" \
    --js_out="import_style=commonjs,binary:${OUT_DIR}" \
    --ts_out="service=true:${OUT_DIR}" \
    ../proto/web.proto

# clean parcel build
rm -rf dist

# compile typescript
# don't need this???
#tsc

# move CSS assets
#cp src/*.css build/dist

# run parcel build
#parcel index.html
