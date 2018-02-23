#!/bin/bash

pushd ..
./build.sh
popd

cp ../co-chair .

docker build -t anxiousmodernman/co-chair .


