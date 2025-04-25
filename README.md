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
Erigon     | [📦 Download](https://snapshots.ethpandaops.io/mainnet/erigon/latest/snapshot.tar.zst)     | [ℹ️ Block](https://snapshots.ethpandaops.io/mainnet/erigon/latest/_snapshot_eth_getBlockByNumber.json)     | [ℹ️ Client](https://snapshots.ethpandaops.io/mainnet/erigon/latest/_snapshot_web3_clientVersion.json)     | `--prune.mode=full`
Geth       | [📦 Download](https://snapshots.ethpandaops.io/mainnet/geth/latest/snapshot.tar.zst)       | [ℹ️ Block](https://snapshots.ethpandaops.io/mainnet/geth/latest/_snapshot_eth_getBlockByNumber.json)       | [ℹ️ Client](https://snapshots.ethpandaops.io/mainnet/geth/latest/_snapshot_web3_clientVersion.json)       | `--state.scheme=path --cache.preimages`
Nethermind | [📦 Download](https://snapshots.ethpandaops.io/mainnet/nethermind/latest/snapshot.tar.zst) | [ℹ️ Block](https://snapshots.ethpandaops.io/mainnet/nethermind/latest/_snapshot_eth_getBlockByNumber.json) | [ℹ️ Client](https://snapshots.ethpandaops.io/mainnet/nethermind/latest/_snapshot_web3_clientVersion.json) |
Reth       | [📦 Download](https://snapshots.ethpandaops.io/mainnet/reth/latest/snapshot.tar.zst)       | [ℹ️ Block](https://snapshots.ethpandaops.io/mainnet/reth/latest/_snapshot_eth_getBlockByNumber.json)       | [ℹ️ Client](https://snapshots.ethpandaops.io/mainnet/reth/latest/_snapshot_web3_clientVersion.json)       | `--full`

### Hoodi

Client     | Snapshot                                                                                   | Block                                                                                                      | Client Version                                                                                            | Args
------     | -----                                                                                      | ---                                                                                                        | ---                                                                                                       | ---
Besu       | [📦 Download](https://snapshots.ethpandaops.io/hoodi/besu/latest/snapshot.tar.zst)       | [ℹ️ Block](https://snapshots.ethpandaops.io/hoodi/besu/latest/_snapshot_eth_getBlockByNumber.json)       | [ℹ️ Client](https://snapshots.ethpandaops.io/hoodi/besu/latest/_snapshot_web3_clientVersion.json)       | `--data-storage-format=BONSAI`
Erigon     | [📦 Download](https://snapshots.ethpandaops.io/hoodi/erigon/latest/snapshot.tar.zst)     | [ℹ️ Block](https://snapshots.ethpandaops.io/hoodi/erigon/latest/_snapshot_eth_getBlockByNumber.json)     | [ℹ️ Client](https://snapshots.ethpandaops.io/hoodi/erigon/latest/_snapshot_web3_clientVersion.json)     | `--prune.mode=full`
Geth       | [📦 Download](https://snapshots.ethpandaops.io/hoodi/geth/latest/snapshot.tar.zst)       | [ℹ️ Block](https://snapshots.ethpandaops.io/hoodi/geth/latest/_snapshot_eth_getBlockByNumber.json)       | [ℹ️ Client](https://snapshots.ethpandaops.io/hoodi/geth/latest/_snapshot_web3_clientVersion.json)       | `--state.scheme=path --cache.preimages`
Nethermind | [📦 Download](https://snapshots.ethpandaops.io/hoodi/nethermind/latest/snapshot.tar.zst) | [ℹ️ Block](https://snapshots.ethpandaops.io/hoodi/nethermind/latest/_snapshot_eth_getBlockByNumber.json) | [ℹ️ Client](https://snapshots.ethpandaops.io/hoodi/nethermind/latest/_snapshot_web3_clientVersion.json) |
Reth       | [📦 Download](https://snapshots.ethpandaops.io/hoodi/reth/latest/snapshot.tar.zst)       | [ℹ️ Block](https://snapshots.ethpandaops.io/hoodi/reth/latest/_snapshot_eth_getBlockByNumber.json)       | [ℹ️ Client](https://snapshots.ethpandaops.io/hoodi/reth/latest/_snapshot_web3_clientVersion.json)       | `--full`


### Holesky

Client     | Snapshot                                                                                   | Block                                                                                                      | Client Version                                                                                            | Args
------     | -----                                                                                      | ---                                                                                                        | ---                                                                                                       | ---
Besu       | [📦 Download](https://snapshots.ethpandaops.io/holesky/besu/latest/snapshot.tar.zst)       | [ℹ️ Block](https://snapshots.ethpandaops.io/holesky/besu/latest/_snapshot_eth_getBlockByNumber.json)       | [ℹ️ Client](https://snapshots.ethpandaops.io/holesky/besu/latest/_snapshot_web3_clientVersion.json)       | `--data-storage-format=BONSAI`
Erigon     | [📦 Download](https://snapshots.ethpandaops.io/holesky/erigon/latest/snapshot.tar.zst)     | [ℹ️ Block](https://snapshots.ethpandaops.io/holesky/erigon/latest/_snapshot_eth_getBlockByNumber.json)     | [ℹ️ Client](https://snapshots.ethpandaops.io/holesky/erigon/latest/_snapshot_web3_clientVersion.json)     | `--prune.mode=full`
Geth       | [📦 Download](https://snapshots.ethpandaops.io/holesky/geth/latest/snapshot.tar.zst)       | [ℹ️ Block](https://snapshots.ethpandaops.io/holesky/geth/latest/_snapshot_eth_getBlockByNumber.json)       | [ℹ️ Client](https://snapshots.ethpandaops.io/holesky/geth/latest/_snapshot_web3_clientVersion.json)       | `--state.scheme=path --cache.preimages`
Nethermind | [📦 Download](https://snapshots.ethpandaops.io/holesky/nethermind/latest/snapshot.tar.zst) | [ℹ️ Block](https://snapshots.ethpandaops.io/holesky/nethermind/latest/_snapshot_eth_getBlockByNumber.json) | [ℹ️ Client](https://snapshots.ethpandaops.io/holesky/nethermind/latest/_snapshot_web3_clientVersion.json) |
Reth       | [📦 Download](https://snapshots.ethpandaops.io/holesky/reth/latest/snapshot.tar.zst)       | [ℹ️ Block](https://snapshots.ethpandaops.io/holesky/reth/latest/_snapshot_eth_getBlockByNumber.json)       | [ℹ️ Client](https://snapshots.ethpandaops.io/holesky/reth/latest/_snapshot_web3_clientVersion.json)       | `--full`

### Sepolia

Client     | Snapshot                                                                                   | Block                                                                                                      | Client Version                                                                                            | Args
------     | -----                                                                                      | ---                                                                                                        | ---                                                                                                       | ---
Besu       | [📦 Download](https://snapshots.ethpandaops.io/sepolia/besu/latest/snapshot.tar.zst)       | [ℹ️ Block](https://snapshots.ethpandaops.io/sepolia/besu/latest/_snapshot_eth_getBlockByNumber.json)       | [ℹ️ Client](https://snapshots.ethpandaops.io/sepolia/besu/latest/_snapshot_web3_clientVersion.json)       | `--data-storage-format=BONSAI`
Erigon     | [📦 Download](https://snapshots.ethpandaops.io/sepolia/erigon/latest/snapshot.tar.zst)     | [ℹ️ Block](https://snapshots.ethpandaops.io/sepolia/erigon/latest/_snapshot_eth_getBlockByNumber.json)     | [ℹ️ Client](https://snapshots.ethpandaops.io/sepolia/erigon/latest/_snapshot_web3_clientVersion.json)     | `--prune.mode=full`
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
```

## Configuration Options

Check a full example config file [here](config.example.yaml).

### Snapshot Cleanup

The snapshotter now includes an automatic cleanup feature that can delete old snapshots. This helps manage storage space by keeping only the most recent snapshots. The feature can be configured in the `config.yaml` file:

```yaml
global:
  snapshots:
    cleanup:
      enabled: true           # Enable or disable the cleanup feature
      keep_count: 3           # Number of most recent snapshots to keep
      check_interval_hours: 24 # How often to check for snapshots to clean up (in hours)
```

When enabled, the cleanup routine will:
1. Run at the specified interval
2. Keep the specified number of most recent successful snapshots
3. Delete older snapshots from storage
4. Mark deleted snapshots in the database

## API Authentication

To protect sensitive endpoints like `persist` and `unpersist`, the snapshotter supports token-based authentication. These endpoints allow you to mark snapshots as persisted, ensuring they won't be deleted by the cleanup routine.

### Configuration

Add an API token in your config file:

```yaml
server:
  listen_addr: 0.0.0.0:5001
  auth:
    api_token: "your-secure-api-token-here"
```

If no token is configured, authentication will be disabled, and a warning will be logged at startup.

### Making Authenticated Requests

To make authenticated requests, include the token in the `Authorization` header:

```
Authorization: Bearer your-secure-api-token-here
```

Example using curl:

```bash
# Mark a snapshot run as persisted
curl -X POST "http://localhost:5001/api/v1/runs/123/persist" \
  -H "Authorization: Bearer your-secure-api-token-here"

# Mark a snapshot run as not persisted
curl -X POST "http://localhost:5001/api/v1/runs/123/unpersist" \
  -H "Authorization: Bearer your-secure-api-token-here"

# Mark a specific target snapshot as persisted
curl -X POST "http://localhost:5001/api/v1/targets/456/persist" \
  -H "Authorization: Bearer your-secure-api-token-here"

# Mark a specific target snapshot as not persisted
curl -X POST "http://localhost:5001/api/v1/targets/456/unpersist" \
  -H "Authorization: Bearer your-secure-api-token-here"
```

### Protected Endpoints

The following endpoints require authentication:

- `POST /api/v1/runs/{id}/persist` - Mark a snapshot run as persisted (won't be deleted)
- `POST /api/v1/runs/{id}/unpersist` - Mark a snapshot run as not persisted (can be deleted)
- `POST /api/v1/targets/{id}/persist` - Mark a specific target snapshot as persisted (won't be deleted)
- `POST /api/v1/targets/{id}/unpersist` - Mark a specific target snapshot as not persisted (can be deleted)

Other endpoints remain publicly accessible:

- `GET /api/v1/runs` - List all snapshot runs
- `GET /api/v1/runs/{id}` - Get details about a specific snapshot run
- `GET /api/v1/targets/{id}` - Get details about a specific target snapshot
- `GET /api/v1/status` - Get snapshotter status

### Persistence Levels

The snapshotter supports two levels of persistence:

1. **Run-level persistence**: When you persist a snapshot run, all targets within that run are protected from cleanup.
2. **Target-level persistence**: You can persist individual target snapshots, protecting only specific client/node data.

This two-level approach gives you fine-grained control over which snapshots to retain.
