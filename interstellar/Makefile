APP := interstellard
BUILD_DIR := bin

.PHONY: build build-rocksdb test-config docker-build docker-build-rocksdb

build:
	CGO_ENABLED=1 go build -trimpath -buildvcs=false -o $(BUILD_DIR)/$(APP) ./cmd/interstellard

# RocksDB is optional and requires librocksdb development headers and libraries
# on the build host. The selected runtime backend must be set separately in
# both config.toml (db_backend) and app.toml (app-db-backend).
build-rocksdb:
	CGO_ENABLED=1 go build -tags rocksdb -trimpath -buildvcs=false -o $(BUILD_DIR)/$(APP)-rocksdb ./cmd/interstellard

test-config:
	go test ./config

docker-build:
	docker build -f Dockerfile -t interstellar/$(APP):dev ..

docker-build-rocksdb:
	docker build --build-arg DB_BACKEND=rocksdb -f Dockerfile -t interstellar/$(APP):rocksdb ..
