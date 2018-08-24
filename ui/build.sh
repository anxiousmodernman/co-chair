#!/bin/bash

set -e

# clean parcel build
rm -rf dist

# clean target
rm -rf build/dist

# compile typescript
tsc

# move CSS assets
cp src/*.css build/dist

# run parcel build
parcel index.html
