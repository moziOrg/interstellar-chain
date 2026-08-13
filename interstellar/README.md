# Interstellar Chain

`interstellar` is the Interstellar application module. It is intentionally separated from
the root `github.com/cosmos/evm` library and from the upstream `evmd` reference
application. This keeps chain-specific consensus and genesis changes reviewable
while allowing upstream Cosmos EVM releases to be merged at the library layer.

## Baseline identity

| Setting | Value |
| --- | --- |
| Binary | `interstellard` |
| Mainnet Cosmos chain ID | `intl-mainnet` |
| Testnet Cosmos chain ID | `intl-testnet-1` |
| Devnet Cosmos chain ID | `intl-dev-1` |
| Mainnet EVM chain ID | `1677` |
| Testnet EVM chain ID | `1678` |
| Native base denom | `ahuge` |
| Native symbol | `HUGE` |
| Decimal places | `18` |
| Bech32 account prefix | `hg` |
| HD coin type | `60` |

The same account bytes are represented as `hg1...` in Cosmos SDK interfaces and
`0x...` in EVM JSON-RPC, wallets, contracts, and exchange integrations.

## Build

The Cosmos go-ethereum fork requires CGO for secp256k1 public-key recovery.
Use a Linux environment with a C compiler, or build the supplied container from
the repository root:

```sh
docker build -f interstellar/Dockerfile -t interstellar/interstellard:dev .
```

The genesis helpers support local and integration testnets. Interstellar replaces
the reference inflation with a capped 170,000,000 HUGE schedule, while mainnet
startup enforces the 17,000,000 HUGE initial circulation. A public-network
genesis still requires an independently reviewed allocation file, validator
set, consensus gas limit, and operational security review before launch.

## Node storage profiles

`--node-mode` selects application defaults for a newly initialized node. It is
not a consensus setting, and each operator must keep its local configuration
consistent with the role it operates.

| Profile | Application state | Blocks and transactions | Intended role |
| --- | --- | --- | --- |
| `val` | Custom pruning: retain 362,880 recent versions, every 10 blocks | `min-retain-blocks = 0`; retain blocks unless an audited retention policy is set | Validator and state-sync source |
| `rpc` | Same bounded state window as `val` | EVM indexer, REST, gRPC and JSON-RPC enabled | Public read/write RPC |
| `archive` | `pruning = nothing`, preserves all application-state versions | `min-retain-blocks = 0` and transaction index must remain enabled | Historical queries, explorer backfill, recovery source |

All profiles create state-sync snapshots every 10,000 blocks and retain three.
Snapshots shorten recovery time; they are not a substitute for independently
verified backups. `min-retain-blocks`, block pruning, and transaction indexing
are separate from application-state pruning and should only be changed after
checking the unbonding period and the state-sync snapshot window.

## Database backend

The default binary uses GoLevelDB. A RocksDB binary is optional:

```sh
make build-rocksdb
```

For a RocksDB node, start the RocksDB-built binary and set both values before
initializing or restoring data:

```toml
# config/config.toml
db_backend = "rocksdb"

# config/app.toml
app-db-backend = "rocksdb"
```

Database backends cannot be switched in place. A node changing backend must use
a new data directory, a verified snapshot restore, state sync, or full sync.

## State repair

`interstellard repair-state --rollback-count 1 --yes --home <HOME>` performs a
matched rollback of CometBFT state and the Cosmos application multistore. It is
only for a known state-machine/app-hash mismatch and must be executed while the
node is stopped. It cannot repair checksum errors, missing database files, or
physical media corruption; those cases require verified snapshot restoration or
sync from a healthy peer.
