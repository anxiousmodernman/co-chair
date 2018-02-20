# co-chair

A configurable edge proxy that also does auth. The aim is to provide a 
single-node TLS proxy, configurable via a UI or grpc api. 

## Overview

A co-chair instance runs on a single server. Public DNS points at this server,
and co-chair will proxy TCP connections to configured backends.

Currently, co-chair picks backends by sniffing the `Host` headers of an HTTP1.1 
request, dialing the backend over TCP, and forwarding all bytes upstream.

Backends can be added via a management UI, which is protected by OAuth via Auth0.
Other OAuth providers may be added in the future, but for now Auth0 is required.
All backend configuration is stored in a local **BoltDB** file, including 
PEM-encoded TLS certificates and private keys. Certs and keys can be added via
the management UI during backend setup. 

## Caveats

This is an experimental project. Don't use it to proxy to anything valuable, yet.
I wrote co-chair to see if I could build a dynamic proxy that didn't require
running a complex service discovery system or Kubernetes. Traefik was an 
inspiration, but I'm trying to build something even simpler.

## Getting started with development

You will need

* Go 1.9 or later
* protoc
* a few go tools: dep and gopherjs

Installing `protoc` on Linux: [Download a zip file for your distribution](https://github.com/google/protobuf/releases), 
expand it somewhere on your system that's a part of your C/C++ "includes" path.
The protoc zip file has the `protoc` compiler binary as well as some required 
libraries that the compiler will need to include at build time. 

Installing `protoc` on MacOS: `brew install protoc`.

Install Go by downloading the appropriate tarball or installer for your system
[here](https://golang.org/).

Get `dep`, with `go get`.

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

