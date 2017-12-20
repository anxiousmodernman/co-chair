generate:
	protoc -I. -I/tmp/protobuf/include -Ivendor/ ./proto/web.proto \
		--gopherjs_out=plugins=grpc:$$GOPATH/src \
		--go_out=plugins=grpc:$$GOPATH/src
	go generate ./frontend/

clean:
	rm ./proto/client/* ./proto/server/* \
		./frontend/html/frontend.js ./frontend/html/frontend.js.map

install:
	go install ./vendor/github.com/golang/protobuf/protoc-gen-go \
		./vendor/github.com/johanbrandhorst/protobuf/protoc-gen-gopherjs

	# GopherJS cannot be vendored so must be fetched
	go get -u github.com/gopherjs/gopherjs

generate_cert:
	go run "$$(go env GOROOT)/src/crypto/tls/generate_cert.go" \
		--host=localhost,127.0.0.1 \
		--ecdsa-curve=P256 \
		--ca=true

serve:
	# to use go run, provide all package main files 
	go run -race main.go web.go serve 

test:
	go test ./...
