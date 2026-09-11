APP := interstellard
BUILD_DIR := bin

.PHONY: build build-rocksdb test-config docker-build docker-build-rocksdb

build:
	CGO_ENABLED=1 go build -trimpath -buildvcs=false -o $(BUILD_DIR)/$(APP) ./cmd/interstellard

# RocksDB is optional and requires compatible librocksdb development headers
# and libraries on the build host. RocksDB is an application backend only:
# config.toml db_backend must remain goleveldb or pebbledb; set
# app.toml app-db-backend=rocksdb when running this binary.
build-rocksdb:
	CGO_ENABLED=1 CGO_CFLAGS="-I$(CURDIR)/rocksdb_compat_include" go build -tags rocksdb -trimpath -buildvcs=false -o $(BUILD_DIR)/$(APP)-rocksdb ./cmd/interstellard

test-config:
	go test ./config

docker-build:
	docker build -f Dockerfile -t interstellar/$(APP):dev ..

docker-build-rocksdb:
	docker build --build-arg DB_BACKEND=rocksdb -f Dockerfile -t interstellar/$(APP):rocksdb ..
