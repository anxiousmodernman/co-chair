# co-chair

[![Build Status](https://travis-ci.org/anxiousmodernman/co-chair.svg?branch=master)](https://travis-ci.org/anxiousmodernman/co-chair)

A configurable tcp proxy.

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

Right now, you'll need an Auth0 account to use the WebUI.

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

Install `dep`, with `go get`.

```
go get -u github.com/golang/dep/cmd/dep  # installs to $GOPATH/bin
dep ensure  # takes a few minutes; clones stuff to vendor/ dir 
```

Install the other dependencies with our provided script

```
./install.sh
```

If all is well, build with

```
./build.sh
```

This will drop a `co-chair` binary. Run in development mode

```
./generate-cert.sh   # only need to do this once
./co-chair serve --bypassAuth0
```

Then visit https://localhost:2016 to see the management UI, which is normally 
behind an Auth0 login. Development certs and a development BoltDB instance
will be created in the directory where co-chair is run. 

## Running tests

Running tests requires modifications to **/etc/hosts**. Please add the following
line to give us 3 additional aliases to lochalhost.

```
127.0.0.1   server1  server2  server3
```

Then run tests with

```
go test ./...
```

## License

This project is licensed under either

* Apache License, Version 2.0, ([LICENSE-APACHE](LICENSE-APACHE) or http://www.apache.org/licenses/LICENSE-2.0)
* MIT license ([LICENSE-MIT](LICENSE-MIT) or http://opensource.org/licenses/MIT)

## Acknowledgements

* [Johan Brandhorst](https://github.com/johanbrandhorst/grpcweb-boilerplate) for
  a really readable example of calling gRPC from a web client.
* [improbable-gen/grpc-web](https://github.com/improbable-eng/grpc-web) for the
  underlying plumbing
* All the gophers in the Slack