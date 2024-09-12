# Solana Log Loader

This application downloads Solana logs from an RPC endpoint and stores program logs in files.

## Usage

```
# download without backfill
solana-slot-loader -t=20 --rpc=[rpc_endpoint] --ws=[ws_endpoint] -o=./data_files

# download with backfill
solana-slot-loader -t=100 --rpc=[rpc_endpoint] -ws=[ws_endpoint] --backfill --backfill-range=25000
```

## Issues

- too many requests for default `mainnet` or `devnet` rpc endpoints
- very large number of files; could collapse into slot ranges