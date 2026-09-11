# Build from the repository root:
# docker build -f interstellar/Dockerfile -t interstellar/interstellard:dev .
ARG GO_IMAGE=golang:1.25.9-bookworm
ARG RUNTIME_IMAGE=debian:bookworm-slim

FROM ${GO_IMAGE} AS builder

ARG DB_BACKEND=goleveldb

RUN apt-get update \
    && apt-get install -y --no-install-recommends build-essential ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src/interstellar-chain
COPY . .

RUN if [ "$DB_BACKEND" = "rocksdb" ]; then \
      ROCKSDB_PREFIX=/usr/local bash scripts/install-rocksdb.sh; \
    elif [ "$DB_BACKEND" != "goleveldb" ]; then \
      echo "unsupported DB_BACKEND: $DB_BACKEND (expected goleveldb or rocksdb)" >&2; exit 1; \
    fi

WORKDIR /src/interstellar-chain/interstellar
RUN mkdir -p /out/runtime-libs \
    && if [ "$DB_BACKEND" = "rocksdb" ]; then \
      PKG_CONFIG_PATH=/usr/local/lib/pkgconfig \
      CGO_CFLAGS=-I/usr/local/include \
      CGO_LDFLAGS=-L/usr/local/lib \
      CGO_ENABLED=1 go build -tags rocksdb -trimpath -buildvcs=false -o /out/interstellard ./cmd/interstellard \
      && cp -a /usr/local/lib/librocksdb.so* /out/runtime-libs/; \
    else \
      CGO_ENABLED=1 go build -trimpath -buildvcs=false -o /out/interstellard ./cmd/interstellard; \
    fi

FROM ${RUNTIME_IMAGE}

RUN apt-get update \
    && apt-get install -y --no-install-recommends libbz2-1.0 libgflags2.2 liblz4-1 libsnappy1v5 libzstd1 zlib1g \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 10001 interstellar \
    && useradd --system --uid 10001 --gid interstellar --create-home --home-dir /home/interstellar interstellar
COPY --from=builder /out/interstellard /usr/local/bin/interstellard
COPY --from=builder /out/runtime-libs/ /usr/local/lib/
RUN ldconfig

USER interstellar
WORKDIR /data
EXPOSE 26656 26657 8545 8546 9090 1317
ENTRYPOINT ["interstellard"]
