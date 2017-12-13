# co-chair

Work-in-progress: a configurable edge proxy that also does auth. The aim is to provide a 
single-node proxy, configurable via a UI or api. 


## Getting started with development

You will need

* Go 1.9 or later
* protoc
* a few go tools: dep and gopherjs

To install Go, [download the tarball for your distribution](https://golang.org/dl/) and expand it 
under /usr/local per the instructions on the Go website. It is recommended
that you do **not** use brew or a system package manager to install Go. These 
will work, but the distribution is simple enough that they're not required.

If Go is installed, we need to add the Go binary tools directory to our PATH

```sh
export GOPATH="$HOME/go"               # set a GOPATH;
export PATH="/usr/local/go/bin:$PATH"  # go toolchain;
export PATH="$GOPATH/bin:$PATH"        # our own binaries; 
```

We should clone our repo underneath the GOPATH:

```sh
mkdir -p $GOPATH/src/gitlab.com/DSASanFrancisco
cd $GOPATH/src/gitlab.com/DSASanFrancisco
git clone git@gitlab.com:DSASanFrancisco/co-chair.git
```

Installing `protoc` is similar. [Download a zip file for your distribution](https://github.com/google/protobuf/releases), 
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
make serve
```

