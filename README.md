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
- `network_name` -> `holesky` or `sepolia`.
- `client_name` -> `geth`, `nethermind`, `besu` or `reth`,

Check the tables below for all the possible combinations.

### Available snapshots

#### Holesky

Network | Client     | Snapshot                                                                                   | Block                                                                                                      | Client Version
--------| ------     | -----                                                                                      | ---                                                                                                        | ---
Holesky | Besu       | [📦 Download](https://snapshots.ethpandaops.io/holesky/besu/latest/snapshot.tar.zst)       | [ℹ️ Block](https://snapshots.ethpandaops.io/holesky/besu/latest/_snapshot_eth_getBlockByNumber.json)       | [ℹ️ Client](https://snapshots.ethpandaops.io/holesky/besu/latest/_snapshot_web3_clientVersion.json)
Holesky | Geth       | [📦 Download](https://snapshots.ethpandaops.io/holesky/geth/latest/snapshot.tar.zst)       | [ℹ️ Block](https://snapshots.ethpandaops.io/holesky/geth/latest/_snapshot_eth_getBlockByNumber.json)       | [ℹ️ Client](https://snapshots.ethpandaops.io/holesky/geth/latest/_snapshot_web3_clientVersion.json)
Holesky | Nethermind | [📦 Download](https://snapshots.ethpandaops.io/holesky/nethermind/latest/snapshot.tar.zst) | [ℹ️ Block](https://snapshots.ethpandaops.io/holesky/nethermind/latest/_snapshot_eth_getBlockByNumber.json) | [ℹ️ Client](https://snapshots.ethpandaops.io/holesky/nethermind/latest/_snapshot_web3_clientVersion.json)
Holesky | Reth       | [📦 Download](https://snapshots.ethpandaops.io/holesky/reth/latest/snapshot.tar.zst)       | [ℹ️ Block](https://snapshots.ethpandaops.io/holesky/reth/latest/_snapshot_eth_getBlockByNumber.json)       | [ℹ️ Client](https://snapshots.ethpandaops.io/holesky/reth/latest/_snapshot_web3_clientVersion.json)


#### Sepolia
Network | Client     | Snapshot                                                                                   | Block                                                                                                      | Client Version
--------| ------     | -----                                                                                      | ---                                                                                                        | ---
Sepolia | Besu       | [📦 Download](https://snapshots.ethpandaops.io/sepolia/besu/latest/snapshot.tar.zst)       | [ℹ️ Block](https://snapshots.ethpandaops.io/sepolia/besu/latest/_snapshot_eth_getBlockByNumber.json)       | [ℹ️ Client](https://snapshots.ethpandaops.io/sepolia/besu/latest/_snapshot_web3_clientVersion.json)
Sepolia | Geth       | [📦 Download](https://snapshots.ethpandaops.io/sepolia/geth/latest/snapshot.tar.zst)       | [ℹ️ Block](https://snapshots.ethpandaops.io/sepolia/geth/latest/_snapshot_eth_getBlockByNumber.json)       | [ℹ️ Client](https://snapshots.ethpandaops.io/sepolia/geth/latest/_snapshot_web3_clientVersion.json)
Sepolia | Nethermind | [📦 Download](https://snapshots.ethpandaops.io/sepolia/nethermind/latest/snapshot.tar.zst) | [ℹ️ Block](https://snapshots.ethpandaops.io/sepolia/nethermind/latest/_snapshot_eth_getBlockByNumber.json) | [ℹ️ Client](https://snapshots.ethpandaops.io/sepolia/nethermind/latest/_snapshot_web3_clientVersion.json)
Sepolia | Reth       | [📦 Download](https://snapshots.ethpandaops.io/sepolia/reth/latest/snapshot.tar.zst)       | [ℹ️ Block](https://snapshots.ethpandaops.io/sepolia/reth/latest/_snapshot_eth_getBlockByNumber.json)       | [ℹ️ Client](https://snapshots.ethpandaops.io/sepolia/reth/latest/_snapshot_web3_clientVersion.json)

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
