# Makefile for Sukhavati Miner Client

# consts
TEST_REPORT=test.report

# make commands
build:
	@echo "make build: begin"
	@echo "building miner to ./bin for current platform..."
	@env GO111MODULE=on go build -o bin/minerd
	@echo "make build: end"

test:
	@echo "make test: begin"
	@env GO111MODULE=on go test ./... 2>&1 | tee $(TEST_REPORT)
	@echo "make test: end"
