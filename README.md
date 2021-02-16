# Sukhavati Miner

`Sukhavati Miner` is a Golang implementation of Sukhavati full-node miner.

## Requirements

[Go](http://golang.org) 1.15 or newer.

## Development

[Sukhavati-Labs](https://github.com/Sukhavati-Labs/go-miner)

### Build from Source

#### Linux/Darwin

- Clone source code to `$GOPATH/src/github.com/Sukhavati-Labs/go-miner`.
- Go to project directory `cd $GOPATH/src/github.com/Sukhavati-Labs/go-miner`.
- Run Makefile `make build`. An executable `minerd` would be generated in `./bin/`.

#### Windows

- Clone source code to `$GOPATH/src/github.com/Sukhavati-Labs/go-miner`.
- Open terminal in `$GOPATH/src/github.com/Sukhavati-Labs/go-miner`.
- Require environment variable as `GO111MODULE="on"`.
- Run `go build -o bin/minerd.exe`. An executable `minerd.exe` would be generated in `./bin/`.

### Contributing Code

#### Prerequisites

- Install [Golang](http://golang.org) 1.15 or newer.
- Install the specific version or [ProtoBuf](https://developers.google.com/protocol-buffers), and related `protoc-*`:
  ```
  # libprotoc
  libprotoc 3.14.0
  
  # github.com/golang/protobuf 1.3.2
  protoc-gen-go
  
  # github.com/gogo/protobuf 1.3.1
  protoc-gen-gogo
  protoc-gen-gofast
  
  # github.com/grpc-ecosystem/grpc-gateway 1.15.2
  protoc-gen-grpc-gateway
  protoc-gen-swagger
  ```

#### Modifying Code

- New codes should be compatible with Go 1.15 or newer.
- Run `gofmt` and `goimports` to lint go files.
- Run `make test` before building executables.

#### Reporting Bugs

Contact Sukhavati community via community@sukhavati.io, and we will get back to you soon.

## Documentation

### API

A documentation for API is provided [here](rpc/README.md).

### Transaction Scripts

A documentation for Transaction Scripts is provided [here](docs/script_en.md).

## License

`Sukhavati Miner` is licensed under the terms of the MIT license. See LICENSE for more information or see https://opensource.org/licenses/MIT.
