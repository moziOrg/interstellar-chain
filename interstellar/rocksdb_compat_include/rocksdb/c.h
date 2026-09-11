#ifndef INTERSTELLAR_ROCKSDB_COMPAT_C_H
#define INTERSTELLAR_ROCKSDB_COMPAT_C_H

// Resolve the actual system header after this compatibility include directory.
#include_next <rocksdb/c.h>

/*
 * grocksdb v1.10.7 retains bindings for these two deprecated C API calls.
 * They were removed by modern RocksDB releases, where SST-size checking at
 * open is handled internally. Interstellar does not call either binding, so
 * provide no-op definitions solely to keep the binding source compatible.
 */
static inline void rocksdb_options_set_skip_checking_sst_file_sizes_on_db_open(
    rocksdb_options_t *opt, unsigned char val) {
  (void)opt;
  (void)val;
}

static inline unsigned char
rocksdb_options_get_skip_checking_sst_file_sizes_on_db_open(
    rocksdb_options_t *opt) {
  (void)opt;
  return 0;
}

/*
 * RocksDB removed the in-range callback from this factory. Preserve the
 * legacy grocksdb call shape and forward the callbacks still accepted by the
 * current API. The project does not construct custom SliceTransforms.
 */
static inline rocksdb_slicetransform_t*
interstellar_rocksdb_slicetransform_create(
    void *state, void (*destructor)(void*),
    char* (*transform)(void*, const char*, size_t, size_t*),
    unsigned char (*in_domain)(void*, const char*, size_t),
    unsigned char (*in_range)(void*, const char*, size_t),
    const char* (*name)(void*)) {
  (void)in_range;
  return rocksdb_slicetransform_create(
      state, destructor, transform, in_domain, name);
}

#define rocksdb_slicetransform_create \
  interstellar_rocksdb_slicetransform_create

#endif
