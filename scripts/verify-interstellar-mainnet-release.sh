#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
bin="${1:-${repo_root}/interstellar/bin/interstellard}"

if ! command -v jq >/dev/null 2>&1; then
  echo "release check requires jq" >&2
  exit 1
fi
if [[ ! -x "$bin" ]]; then
  echo "Interstellar binary is missing or not executable: $bin" >&2
  exit 1
fi

check_home="$(mktemp -d "${TMPDIR:-/tmp}/interstellar-mainnet-release.XXXXXX")"
trap 'rm -rf -- "$check_home"' EXIT

"$bin" init mainnet-release-check --chain-id intl-main --home "$check_home" >/dev/null 2>&1

jq -e '
  .chain_id == "intl-main"
  and (.consensus.params.block.max_gas | tonumber) == 100000000
  and (.app_state.staking.params.max_validators | tonumber) == 5
  and .app_state.evm.params.evm_denom == "ahuge"
  and .app_state.feemarket.params.min_gas_price == "1000000000.000000000000000000"
  and .app_state.mint.minter.inflation == "0.000000000000000000"
' "$check_home/config/genesis.json" >/dev/null

grep -Eq '^evm-chain-id = 1677$' "$check_home/config/app.toml"

echo "Mainnet genesis defaults verified: chain=intl-main evm=1677 max_gas=100000000 max_validators=5"
