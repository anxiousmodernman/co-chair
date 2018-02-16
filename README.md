# co-chair

A configurable edge proxy that also does auth. The aim is to provide a 
single-node proxy, configurable via a UI or grpc api. 

## Getting started with development

You will need

* Go 1.9 or later
* protoc
* a few go tools: dep and gopherjs

Installing `protoc` on Linux: [Download a zip file for your distribution](https://github.com/google/protobuf/releases), 
expand it somewhere on your system that's a part of your C/C++ "includes" path.
The protoc zip file has the `protoc` compiler binary as well as some required 
libraries that the compiler will need to include at build time. 

Get `dep`, to fetch Go dependencies from the package management files Gopkg.toml/lock

```
go get -u github.com/golang/dep/cmd/dep  # installs to $GOPATH/bin
dep ensure  # takes a few minutes; clones stuff to vendor/ dir 
```

If that all those baseline tools are installed, and fetching dependencies with 
dep works, the Makefile takes us the last mile.

```
make install
make generate
make generate_cert
make serve-no-auth
```

## Running tests

Running tests requires modifications to **/etc/hosts**. Please add the following
line to give us 3 additional aliases to lochalhost.

```
127.0.0.1   server1  server2  server3
```

Then run tests with

```
make test
```

