#!/bin/bash

# Set the baseURL and network as configurable variables
network=${1:-sepolia}
baseURL=${2:-https://snapshots.ethpandaops.io}

# Array to store block numbers
block_numbers=()

# List of clients
clients=("geth" "besu" "nethermind" "reth")

echo "Verifying snapshot block numbers for network: $network"

# Loop through each client and fetch block numbers
for client in "${clients[@]}"; do
  block_number=$(printf '%d\n' $(curl -s "$baseURL/$network/$client/latest/_snapshot_eth_getBlockByNumber.json" | jq -r '.result.number'))
  block_numbers+=("$block_number")
  echo "Block: $block_number ($client)"
done

# Verify if all block numbers are the same
first_block_number="${block_numbers[0]}"
all_same=true

for i in "${!clients[@]}"; do
  if [[ "${block_numbers[$i]}" != "$first_block_number" ]]; then
    echo "Block number mismatch detected for client: ${clients[$i]}"
    all_same=false
  fi
done

if [ "$all_same" = true ]; then
  echo "✅ All block numbers are the same: https://${network}.etherscan.io/block/${first_block_number}"
else
  echo "❌ Some block numbers are different."
  exit 1
fi
