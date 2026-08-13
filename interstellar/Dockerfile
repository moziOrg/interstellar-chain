# Build from the repository root:
# docker build -f interstellar/Dockerfile -t interstellar/interstellard:dev .
FROM golang:1.25.9-bookworm AS builder

ARG DB_BACKEND=goleveldb

RUN apt-get update \
    && apt-get install -y --no-install-recommends build-essential librocksdb-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src/interstellar-chain
COPY . .

WORKDIR /src/interstellar-chain/interstellar
RUN if [ "$DB_BACKEND" = "rocksdb" ]; then \
      CGO_ENABLED=1 go build -tags rocksdb -trimpath -buildvcs=false -o /out/interstellard ./cmd/interstellard; \
    else \
      CGO_ENABLED=1 go build -trimpath -buildvcs=false -o /out/interstellard ./cmd/interstellard; \
    fi

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends librocksdb7.8 \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --system --create-home --home-dir /home/interstellar interstellar
COPY --from=builder /out/interstellard /usr/local/bin/interstellard

USER interstellar
WORKDIR /home/interstellar
EXPOSE 26656 26657 8545 8546 9090 1317
ENTRYPOINT ["interstellard"]
