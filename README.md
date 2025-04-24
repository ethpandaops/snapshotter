# snapshotter

Used to create data snapshots from ethereum nodes managed by EthPandaOps.

## How to use the snapshots

We currently provide data directory snapshots for Ethereum execution clients on the Sepolia and Holesky test networks. These snapshots are updated automatically every 2-3 days.

The data snapshots are packaged into a [tar](https://man7.org/linux/man-pages/man1/tar.1.html), compressed using [zstandard](https://github.com/facebook/zstd).

### URL naming conventions

What | URL
---  | ----
Snapshot | `https://snapshots.ethpandaops.io/{{ network_name }}/{{ client_name }}/latest/snapshot.tar.zst`
Block info | `https://snapshots.ethpandaops.io/{{ network_name }}/{{ client_name }}/latest/_snapshot_eth_getBlockByNumber.json`
Client info | `https://snapshots.ethpandaops.io/{{ network_name }}/{{ client_name }}/latest/_snapshot_web3_clientVersion.json`

Possible values:
- `network_name` -> `holesky`, `sepolia`, `mainnet`.
- `client_name` -> `geth`, `nethermind`, `besu`, `erigon`, `reth`

Check the tables below for all the possible combinations.

## Available snapshots

### Mainnet

Client     | Snapshot                                                                                   | Block                                                                                                      | Client Version                                                                                            | Args
------     | -----                                                                                      | ---                                                                                                        | ---                                                                                                       | ---
Besu       | [📦 Download](https://snapshots.ethpandaops.io/mainnet/besu/latest/snapshot.tar.zst)       | [ℹ️ Block](https://snapshots.ethpandaops.io/mainnet/besu/latest/_snapshot_eth_getBlockByNumber.json)       | [ℹ️ Client](https://snapshots.ethpandaops.io/mainnet/besu/latest/_snapshot_web3_clientVersion.json)       | `--data-storage-format=BONSAI`
Erigon     | [📦 Download](https://snapshots.ethpandaops.io/mainnet/erigon/latest/snapshot.tar.zst)     | [ℹ️ Block](https://snapshots.ethpandaops.io/mainnet/erigon/latest/_snapshot_eth_getBlockByNumber.json)     | [ℹ️ Client](https://snapshots.ethpandaops.io/mainnet/erigon/latest/_snapshot_web3_clientVersion.json)     | `--prune=hrtc `
Geth       | [📦 Download](https://snapshots.ethpandaops.io/mainnet/geth/latest/snapshot.tar.zst)       | [ℹ️ Block](https://snapshots.ethpandaops.io/mainnet/geth/latest/_snapshot_eth_getBlockByNumber.json)       | [ℹ️ Client](https://snapshots.ethpandaops.io/mainnet/geth/latest/_snapshot_web3_clientVersion.json)       | `--state.scheme=path --cache.preimages`
Nethermind | [📦 Download](https://snapshots.ethpandaops.io/mainnet/nethermind/latest/snapshot.tar.zst) | [ℹ️ Block](https://snapshots.ethpandaops.io/mainnet/nethermind/latest/_snapshot_eth_getBlockByNumber.json) | [ℹ️ Client](https://snapshots.ethpandaops.io/mainnet/nethermind/latest/_snapshot_web3_clientVersion.json) |
Reth       | [📦 Download](https://snapshots.ethpandaops.io/mainnet/reth/latest/snapshot.tar.zst)       | [ℹ️ Block](https://snapshots.ethpandaops.io/mainnet/reth/latest/_snapshot_eth_getBlockByNumber.json)       | [ℹ️ Client](https://snapshots.ethpandaops.io/mainnet/reth/latest/_snapshot_web3_clientVersion.json)       | `--full`

### Hoodi

Client     | Snapshot                                                                                   | Block                                                                                                      | Client Version                                                                                            | Args
------     | -----                                                                                      | ---                                                                                                        | ---                                                                                                       | ---
Besu       | [📦 Download](https://snapshots.ethpandaops.io/hoodi/besu/latest/snapshot.tar.zst)       | [ℹ️ Block](https://snapshots.ethpandaops.io/hoodi/besu/latest/_snapshot_eth_getBlockByNumber.json)       | [ℹ️ Client](https://snapshots.ethpandaops.io/hoodi/besu/latest/_snapshot_web3_clientVersion.json)       | `--data-storage-format=BONSAI`
Erigon     | [📦 Download](https://snapshots.ethpandaops.io/hoodi/erigon/latest/snapshot.tar.zst)     | [ℹ️ Block](https://snapshots.ethpandaops.io/hoodi/erigon/latest/_snapshot_eth_getBlockByNumber.json)     | [ℹ️ Client](https://snapshots.ethpandaops.io/hoodi/erigon/latest/_snapshot_web3_clientVersion.json)     | `--prune=hrtc `
Geth       | [📦 Download](https://snapshots.ethpandaops.io/hoodi/geth/latest/snapshot.tar.zst)       | [ℹ️ Block](https://snapshots.ethpandaops.io/hoodi/geth/latest/_snapshot_eth_getBlockByNumber.json)       | [ℹ️ Client](https://snapshots.ethpandaops.io/hoodi/geth/latest/_snapshot_web3_clientVersion.json)       | `--state.scheme=path --cache.preimages`
Nethermind | [📦 Download](https://snapshots.ethpandaops.io/hoodi/nethermind/latest/snapshot.tar.zst) | [ℹ️ Block](https://snapshots.ethpandaops.io/hoodi/nethermind/latest/_snapshot_eth_getBlockByNumber.json) | [ℹ️ Client](https://snapshots.ethpandaops.io/hoodi/nethermind/latest/_snapshot_web3_clientVersion.json) |
Reth       | [📦 Download](https://snapshots.ethpandaops.io/hoodi/reth/latest/snapshot.tar.zst)       | [ℹ️ Block](https://snapshots.ethpandaops.io/hoodi/reth/latest/_snapshot_eth_getBlockByNumber.json)       | [ℹ️ Client](https://snapshots.ethpandaops.io/hoodi/reth/latest/_snapshot_web3_clientVersion.json)       | `--full`


### Holesky

Client     | Snapshot                                                                                   | Block                                                                                                      | Client Version                                                                                            | Args
------     | -----                                                                                      | ---                                                                                                        | ---                                                                                                       | ---
Besu       | [📦 Download](https://snapshots.ethpandaops.io/holesky/besu/latest/snapshot.tar.zst)       | [ℹ️ Block](https://snapshots.ethpandaops.io/holesky/besu/latest/_snapshot_eth_getBlockByNumber.json)       | [ℹ️ Client](https://snapshots.ethpandaops.io/holesky/besu/latest/_snapshot_web3_clientVersion.json)       | `--data-storage-format=BONSAI`
Erigon     | [📦 Download](https://snapshots.ethpandaops.io/holesky/erigon/latest/snapshot.tar.zst)     | [ℹ️ Block](https://snapshots.ethpandaops.io/holesky/erigon/latest/_snapshot_eth_getBlockByNumber.json)     | [ℹ️ Client](https://snapshots.ethpandaops.io/holesky/erigon/latest/_snapshot_web3_clientVersion.json)     | `--prune=hrtc `
Geth       | [📦 Download](https://snapshots.ethpandaops.io/holesky/geth/latest/snapshot.tar.zst)       | [ℹ️ Block](https://snapshots.ethpandaops.io/holesky/geth/latest/_snapshot_eth_getBlockByNumber.json)       | [ℹ️ Client](https://snapshots.ethpandaops.io/holesky/geth/latest/_snapshot_web3_clientVersion.json)       | `--state.scheme=path --cache.preimages`
Nethermind | [📦 Download](https://snapshots.ethpandaops.io/holesky/nethermind/latest/snapshot.tar.zst) | [ℹ️ Block](https://snapshots.ethpandaops.io/holesky/nethermind/latest/_snapshot_eth_getBlockByNumber.json) | [ℹ️ Client](https://snapshots.ethpandaops.io/holesky/nethermind/latest/_snapshot_web3_clientVersion.json) |
Reth       | [📦 Download](https://snapshots.ethpandaops.io/holesky/reth/latest/snapshot.tar.zst)       | [ℹ️ Block](https://snapshots.ethpandaops.io/holesky/reth/latest/_snapshot_eth_getBlockByNumber.json)       | [ℹ️ Client](https://snapshots.ethpandaops.io/holesky/reth/latest/_snapshot_web3_clientVersion.json)       | `--full`

### Sepolia

Client     | Snapshot                                                                                   | Block                                                                                                      | Client Version                                                                                            | Args
------     | -----                                                                                      | ---                                                                                                        | ---                                                                                                       | ---
Besu       | [📦 Download](https://snapshots.ethpandaops.io/sepolia/besu/latest/snapshot.tar.zst)       | [ℹ️ Block](https://snapshots.ethpandaops.io/sepolia/besu/latest/_snapshot_eth_getBlockByNumber.json)       | [ℹ️ Client](https://snapshots.ethpandaops.io/sepolia/besu/latest/_snapshot_web3_clientVersion.json)       | `--data-storage-format=BONSAI`
Erigon     | [📦 Download](https://snapshots.ethpandaops.io/sepolia/erigon/latest/snapshot.tar.zst)     | [ℹ️ Block](https://snapshots.ethpandaops.io/sepolia/erigon/latest/_snapshot_eth_getBlockByNumber.json)     | [ℹ️ Client](https://snapshots.ethpandaops.io/sepolia/erigon/latest/_snapshot_web3_clientVersion.json)     | `--prune=hrtc `
Geth       | [📦 Download](https://snapshots.ethpandaops.io/sepolia/geth/latest/snapshot.tar.zst)       | [ℹ️ Block](https://snapshots.ethpandaops.io/sepolia/geth/latest/_snapshot_eth_getBlockByNumber.json)       | [ℹ️ Client](https://snapshots.ethpandaops.io/sepolia/geth/latest/_snapshot_web3_clientVersion.json)       | `--state.scheme=path --cache.preimages`
Nethermind | [📦 Download](https://snapshots.ethpandaops.io/sepolia/nethermind/latest/snapshot.tar.zst) | [ℹ️ Block](https://snapshots.ethpandaops.io/sepolia/nethermind/latest/_snapshot_eth_getBlockByNumber.json) | [ℹ️ Client](https://snapshots.ethpandaops.io/sepolia/nethermind/latest/_snapshot_web3_clientVersion.json) |
Reth       | [📦 Download](https://snapshots.ethpandaops.io/sepolia/reth/latest/snapshot.tar.zst)       | [ℹ️ Block](https://snapshots.ethpandaops.io/sepolia/reth/latest/_snapshot_eth_getBlockByNumber.json)       | [ℹ️ Client](https://snapshots.ethpandaops.io/sepolia/reth/latest/_snapshot_web3_clientVersion.json)       | `--full`

### Example: Getting a Sepolia Geth data dir snapshot

Verify when the latest snapshot was taken:

```sh
# Check the latest block:
curl -s https://snapshots.ethpandaops.io/sepolia/geth/latest/_snapshot_eth_getBlockByNumber.json

# Or just get the block number:
printf '%d\n' $(curl -s https://snapshots.ethpandaops.io/sepolia/geth/latest/_snapshot_eth_getBlockByNumber.json | jq -r '.result.number')

# Or just the time when it was taken
printf '%d\n' $(curl -s https://snapshots.ethpandaops.io/sepolia/geth/latest/_snapshot_eth_getBlockByNumber.json | jq -r '.result.timestamp') | date
```

Then, also check which client version was used during the snapshot:

```sh
# Get client version. Can be important, depending on the version that you want to run.
curl -s https://snapshots.ethpandaops.io/sepolia/geth/latest/_snapshot_web3_clientVersion.json | jq -r '.result'
```

If you're happy with the version and the timestamp of the most recent snapshot, you can download it like:

```sh
# Download the whole snapshot
curl -O https://snapshots.ethpandaops.io/sepolia/geth/latest/snapshot.tar.zst

# Or... download and untar at the same time. Safes you disk space, so you don't have to store the full compressed file.
curl -s -L https://snapshots.ethpandaops.io/sepolia/geth/latest/snapshot.tar.zst | tar -I zstd -xvf - -C $PATH_TO_YOUR_GETH_DATA_DIR

# Or.. use a docker container with all the tools you need (curl, zstd, tar) and untar it on the fly
docker run --rm -it -v $PATH_TO_YOUR_GETH_DATA_DIR:/data --entrypoint "/bin/sh" alpine -c "apk add --no-cache curl tar zstd && curl -s -L https://snapshots.ethpandaops.io/sepolia/geth/latest/snapshot.tar.zst | tar -I zstd -xvf - -C /data"
