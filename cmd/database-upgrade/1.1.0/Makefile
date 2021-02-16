build:
	@echo "make build: begin"
	@echo "building poc-db-upgrade to ./bin for current platform..."
	@env GO111MODULE=on go build -o ./bin/poc-db-upgrade-1.1.0
	@echo "make build: end"

clean:
	@echo "make clean: begin"
	@echo "cleaning .bin/ path..."
	@rm -rf ./bin/logs ./bin/poc-db-upgrade*
	@echo "make clean: end"