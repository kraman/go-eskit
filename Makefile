TMPDIR := $(shell mktemp -d)
PROTO_FILES = $(shell find pkg -name \*.proto | sed 's/.proto/.pb.gw.go/')
GO_FILES = $(shell find pkg -name \*.go)

all: build/main

build/main: pkg/main.go $(PROTO_FILES) $(GO_FILES)
	@echo "* compiling $@"
	@go build -o $@ $<

build/protoc/bin/protoc:
	@echo '* downloading protoc'
	@mkdir -p build
	@mkdir -p $(TMPDIR)/protoc
	@wget https://github.com/protocolbuffers/protobuf/releases/download/v3.7.1/protoc-3.7.1-linux-x86_64.zip -O $(TMPDIR)/protoc/protoc.zip -q > /dev/null
	@cd $(TMPDIR)/protoc && unzip protoc.zip > /dev/null
	@rm -f $(TMPDIR)/protoc/protoc.zip
	@mv $(TMPDIR)/protoc build/protoc

build/bin/protoc-gen-go:
	@echo '* builiding protoc-gen-go'
	@mkdir -p $(@D)
	@go get github.com/golang/protobuf
	@go get github.com/golang/protobuf/protoc-gen-go
	@go build -o $@ $(shell go list -f '{{ .Dir }}' -m github.com/golang/protobuf)/protoc-gen-go/main.go

build/bin/protoc-gen-grpc-gateway:
	@echo '* builiding protoc-gen-grpc-gateway'
	@mkdir -p $(@D)
	@go get github.com/grpc-ecosystem/grpc-gateway
	@go get github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway
	@go build -o $@ $(shell go list -f '{{ .Dir }}' -m github.com/grpc-ecosystem/grpc-gateway)/protoc-gen-grpc-gateway/main.go

build/bin/protoc-gen-swagger:
	@echo '* builiding protoc-gen-swagger'
	@mkdir -p $(@D)
	@go get github.com/grpc-ecosystem/grpc-gateway/protoc-gen-swagger
	@go build -o $@ $(shell go list -f '{{ .Dir }}' -m github.com/grpc-ecosystem/grpc-gateway)/protoc-gen-swagger/main.go	

%.pb.gw.go: %.proto build/protoc/bin/protoc build/bin/protoc-gen-go build/bin/protoc-gen-grpc-gateway build/bin/protoc-gen-swagger
	@echo "* compiling $@"
	PATH=build/bin:$(PATH) build/protoc/bin/protoc \
		-I $(shell go list -f '{{ .Dir }}' -m github.com/grpc-ecosystem/grpc-gateway)/third_party/googleapis \
		-I build/protoc/include \
		-I=$(@D) \
		--go_out=plugins=grpc:$(@D) \
		--grpc-gateway_out=logtostderr=true:$(@D) \
		--swagger_out=logtostderr=true:$(@D) \
		$<

clean:
	@find pkg -name \*.pb.go -exec rm {} \;
	@find pkg -name \*.pb.gw.go -exec rm {} \;
	@find pkg -name \*.swagger.json -exec rm {} \;

cleaner: clean
	@rm -rf build
	
