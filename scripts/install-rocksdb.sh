#!/usr/bin/env bash
# Installs the RocksDB revision used by this project for grocksdb v1.10.7.
#
# Usage:
#   sudo bash scripts/install-rocksdb.sh
#
# Optional overrides:
#   ROCKSDB_REF=<git-commit-or-tag> ROCKSDB_PREFIX=/usr/local \
#     sudo bash scripts/install-rocksdb.sh

set -euo pipefail

readonly ROCKSDB_REPOSITORY="https://github.com/facebook/rocksdb"
readonly ROCKSDB_REF_DEFAULT="323d915dbcaed4a7a1d8bf73389c389c08f69e03"
readonly ROCKSDB_REF="${ROCKSDB_REF:-$ROCKSDB_REF_DEFAULT}"
readonly ROCKSDB_PREFIX="${ROCKSDB_PREFIX:-/usr/local}"

if [[ "${EUID}" -ne 0 ]]; then
  echo "Run this script with sudo or as root." >&2
  exit 1
fi

if [[ "${ROCKSDB_PREFIX}" != /* || "${ROCKSDB_PREFIX}" == "/" ]]; then
  echo "ROCKSDB_PREFIX must be an absolute path other than /." >&2
  exit 1
fi

work_dir="$(mktemp -d /tmp/rocksdb-install.XXXXXX)"
trap 'rm -rf "${work_dir}"' EXIT

source_dir="${work_dir}/source"
build_dir="${work_dir}/build"
backup_dir="${work_dir}/previous-install"

echo "Installing RocksDB ${ROCKSDB_REF} into ${ROCKSDB_PREFIX}"

export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y --no-install-recommends \
  build-essential \
  ca-certificates \
  cmake \
  curl \
  libbz2-dev \
  libgflags-dev \
  liblz4-dev \
  libsnappy-dev \
  libzstd-dev \
  pkg-config \
  zlib1g-dev

mkdir -p "${source_dir}" "${build_dir}" "${backup_dir}"
curl --fail --location --retry 5 \
  "${ROCKSDB_REPOSITORY}/archive/${ROCKSDB_REF}.tar.gz" \
  --output "${work_dir}/rocksdb.tar.gz"
tar -xzf "${work_dir}/rocksdb.tar.gz" --strip-components=1 -C "${source_dir}"

if ! grep -q 'typedef struct rocksdb_slice_t' "${source_dir}/include/rocksdb/c.h"; then
  echo "Selected RocksDB source does not provide rocksdb_slice_t." >&2
  exit 1
fi

cmake -S "${source_dir}" -B "${build_dir}" \
  -DCMAKE_BUILD_TYPE=Release \
  -DROCKSDB_BUILD_SHARED=ON \
  -DWITH_TESTS=OFF \
  -DWITH_TOOLS=OFF \
  -DWITH_BENCHMARK_TOOLS=OFF
cmake --build "${build_dir}" --target rocksdb-shared --parallel "$(nproc)"

# Build before replacing an existing /usr/local installation, then retain it in
# the temporary backup until the replacement and verification have completed.
mkdir -p "${ROCKSDB_PREFIX}/include" "${ROCKSDB_PREFIX}/lib/pkgconfig"
if [[ -d "${ROCKSDB_PREFIX}/include/rocksdb" ]]; then
  mv "${ROCKSDB_PREFIX}/include/rocksdb" "${backup_dir}/include-rocksdb"
fi

shopt -s nullglob
previous_libraries=("${ROCKSDB_PREFIX}/lib"/librocksdb.so*)
shopt -u nullglob
if (( ${#previous_libraries[@]} )); then
  mkdir -p "${backup_dir}/lib"
  mv "${previous_libraries[@]}" "${backup_dir}/lib/"
fi
if [[ -f "${ROCKSDB_PREFIX}/lib/pkgconfig/rocksdb.pc" ]]; then
  mkdir -p "${backup_dir}/pkgconfig"
  mv "${ROCKSDB_PREFIX}/lib/pkgconfig/rocksdb.pc" "${backup_dir}/pkgconfig/"
fi

cp -a "${source_dir}/include/rocksdb" "${ROCKSDB_PREFIX}/include/"
cp -a "${build_dir}"/librocksdb.so* "${ROCKSDB_PREFIX}/lib/"
install -m 0644 "${build_dir}/rocksdb.pc" \
  "${ROCKSDB_PREFIX}/lib/pkgconfig/rocksdb.pc"

ldconfig

PKG_CONFIG_PATH="${ROCKSDB_PREFIX}/lib/pkgconfig${PKG_CONFIG_PATH:+:${PKG_CONFIG_PATH}}" \
  pkg-config --modversion rocksdb
grep -q 'typedef struct rocksdb_slice_t' "${ROCKSDB_PREFIX}/include/rocksdb/c.h"
ldconfig -p | grep -q 'librocksdb.so'

echo "RocksDB installation completed successfully."
