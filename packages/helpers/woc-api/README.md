### Requirements

Requires mongodb and redis for caching. 
Can be disable from settings_local.conf for local dev.


### Development

Written in Golang

### Mongo

mongo

To create the mongo schema, user and collections

use bsv

db.createCollection('blocks')

db.createCollection('blocktxids')

db.createCollection('transactions')


Create Indexes

db.transactions.createIndex( { txid: 1 }, { unique: true } )


db.blocktxids.createIndex( { blockhash: 1 } )

db.blocktxids.createIndex( { blockhash: 1, startindex: 1 }, { unique: true } )


db.blocks.createIndex( { hash: 1 }, { unique: true } )

db.blocks.createIndex( { height: 1 } )


// change the user and password here and in the config.json file

db.createUser({user:'bsvuser', pwd:'bsvpwd99', roles: [{role: "readWrite", db:"bsv"}]})


### Elasticsearch

Elasticsearch should be installed

Configuration in settings.conf

es_url=http://127.0.0.1:9200
es_indexname=or_index


### Clear Mongo cache 

> use bsv
switched to db bsv
> show collections
blocks
blocktxids
transactions
> db.blocks.drop()
true
> db.blocktxids.drop()
true
> db.transactions.drop()
true

sudo systemctl restart mongodb.service

after that receate collections from above sections.


---

## Dependencies

The service requires connection to the following services:

**Required:**
- bitcoind node (in config `BSV_*` settings)
- MongoDB (in config `mongo*` settings) - can be disabled for local dev
- Redis (in config `redis_*` settings) - can be disabled for local dev

**Optional:**
- ElectrumX (in config `electrumUrl` settings) - alternative blockchain query interface
- Elasticsearch (in config `es_*` settings) - for OP_RETURN search functionality

**gRPC Services:**
- bstore (in config `bstoreAddress`) - block storage service, primary data source
- woc-stats (in config `woc_stats_address`) - block/miner statistics
- woc-exchange-rate (in config `woc_exchange_rate_address`) - exchange rate data
- p2p-service (in config `p2p_service_address`) - P2P network information
- utxo-store (in config `utxoStoreAddress`) - UTXO set management
- utxos-mempool (in config `utxosMempoolAddress`) - mempool UTXO tracking
- token-service (in config `tokenServiceAddress`) - BSV-21 and 1Sat token data
- token-mempool (in config `tokenMempoolAddress`) - token mempool tracking
- account-manager (in config `accountManagerAddress`) - API key authentication and rate limiting

This service is part of the WhatsOnChain ecosystem and can be run within entire stack locally or connected to live services.

## Ports

| Port | Purpose | Server Type |
|------|---------|-------------|
| 8084 | Main HTTP API | Gorilla mux |
| 8085 | Fiber HTTP API | Fiber (internal) |

## How to debug live and common issues

Boxes list and diagram are here: [WhatsOnChain - Flow Diagram](https://teranode.atlassian.net/wiki/spaces/WoC/pages/16056411/WhatsOnChain+-+Flow+Diagram)

1) Check status: `systemctl status woc-api`

2) Check live logs: `journalctl -u woc-api -f`

3) Check errors from last 2 hours: `journalctl -u woc-api --since "2 hours ago" --grep error`

4) Check if there is sufficient free space on the box: `df -h` - values very close or `100%` are not correct and may impact the service. Free storage to ensure there are no problems for service/database.

5) Port (default `8084`) can be forwarded to local machine and used for check responses by:

- autossh (recommended): `autossh -M 12345 woc-api-box -L 8084:localhost:8084` - port `12345` is any free port on your local machine, autossh uses it for monitoring
- ssh: `ssh -L 8084:localhost:8084 woc-api-box`

In both examples local port `8084` has to be free (no app should listen on this)

If this port can't be forwarded or does not work on remote host (`woc-api-box`) it usually means a problem with `woc-api`

6) Most of the problems can be solved by restart `systemctl restart woc-api` but before restart make sure restart is essential. Restart is usually needed when the service hangs/not respond. In case of any doubts, ask the service owner.

### Common Issues and Resolutions

**Service won't start - gRPC connection failures:**

The service connects to multiple gRPC services on startup. If any critical service (bstore, woc-stats, woc-exchange-rate, p2p-service) is unavailable, the service will fail to start.

**Resolution:**
```bash
# Check which gRPC service is failing from logs
journalctl -u woc-api -n 100 | grep "failed to connect"

# Verify the service addresses in config
grep "_address" /path/to/settings_local.conf

# Check if the gRPC service is running
systemctl status bstore
systemctl status woc-stats
systemctl status woc-exchange-rate
```

**MongoDB connection issues:**

If MongoDB is enabled but not accessible, the service will have degraded performance or fail health checks.

**Resolution:**
```bash
# Check MongoDB status
systemctl status mongodb

# Test MongoDB connection
mongo --host <mongoHost> --port <mongoPort> -u <mongoUsername> -p <mongoPassword> --authenticationDatabase <mongoDatabase>

# Disable MongoDB caching for local dev if needed
# In settings_local.conf, set: mongoCache=false
```

**Redis connection issues:**

Similar to MongoDB, Redis issues will cause caching failures.

**Resolution:**
```bash
# Check Redis status
systemctl status redis

# Test Redis connection
redis-cli -h <redis_host> -p <redis_port> ping

# Disable Redis for local dev if needed
# In settings_local.conf, set: redis_cache_enabled=false
```

**Bitcoin node connectivity issues:**

The service requires connection to a Bitcoin SV node (or TAAL proxy).

**Resolution:**
```bash
# Test Bitcoin node RPC
curl --user <BSV_username>:<BSV_password> --data-binary '{"jsonrpc":"1.0","id":"test","method":"getblockchaininfo","params":[]}' -H 'content-type: text/plain;' http://<BSV_host>:<BSV_port>/

# Check if TAAL proxy is enabled and configured
grep "taalBitcoinProxyEnabled" /path/to/settings_local.conf
```

**Health check failing:**

The service has extensive health checks for all dependencies.

**Resolution:**
```bash
# Check health endpoint
curl http://localhost:8084/woc/v1/bsv/main/health

# The response shows status of each component:
# - bitcoinNode
# - electrumX (if enabled)
# - mongoDB (if enabled)
# - bstore
# - sockets (if enabled)
# - tokenMempool (if enabled)
# - tokenService (if enabled)
# - utxoStore (if enabled)
# - utxosMempool (if enabled)
# - wocExchangeRate (if enabled)
# - wocStats (if enabled)
# - p2pService (if enabled)
# - wocChainStats (if enabled)
```

**High memory usage:**

Large blockchain queries can cause memory spikes.

**Resolution:**
```bash
# Check current memory usage
free -h
ps aux | grep woc-api

# Review recent API calls in logs
journalctl -u woc-api --since "1 hour ago" | grep -E "POST|GET"

# Consider adjusting limits in settings:
# - maxTxHexLength (default 100000 bytes)
# - bstoreGrpcMaxCallRecvMsgSize_mb (default 500MB)
```

### API Key Issues

If API key authentication is enabled (`apiKeyCheckEnabled=true`):

**Resolution:**
```bash
# Check if account-manager is accessible
grpcurl -plaintext <accountManagerAddress> list

# Verify API key cache
ls -la /path/to/lastKnownAPIKeys.json

# Check API key activity logs (if enabled)
journalctl -u woc-api | grep "apikey"

# Temporarily disable API key check for debugging
# In settings_local.conf: apiKeyCheckEnabled=false
```

## Healthcheck

The service provides comprehensive health checks for all dependencies.

**HTTP Health Check:**

```bash
# Check overall health
curl http://localhost:8084/woc/v1/bsv/main/health

# Example response:
{
  "status": "ok",
  "components": {
    "bitcoinNode": "up",
    "bstore": "up",
    "mongoDB": "up",
    "redis": "up",
    "wocStats": "up",
    "wocExchangeRate": "up",
    "p2pService": "up",
    "utxoStore": "up"
  }
}
```

**Health Check via SSH Tunnel:**

1) Forward the health check port:
   ```bash
   ssh -L 8084:localhost:8084 woc-api-box
   ```

2) Query health endpoint:
   ```bash
   curl http://localhost:8084/woc/v1/bsv/main/health | jq
   ```

**Component-Specific Health Checks:**

The health check system monitors:
- Bitcoin node connectivity and sync status
- bstore accessibility and sync status (checks block height delay)
- MongoDB connectivity (if enabled)
- Redis connectivity (if enabled)
- ElectrumX connectivity (if enabled)
- All gRPC services (woc-stats, woc-exchange-rate, p2p-service, utxo-store, utxos-mempool, token-service, token-mempool)
- WebSocket service (if enabled)

**Health Check Configuration:**

Health checks can be individually enabled/disabled in `settings_local.conf`:

```ini
check_bitcoinNode=true
check_mongoDB=true
check_bStoreReadable=true
check_bStoreSynced=true
check_electrumX=false
check_sockets=true
check_tokenMempool=true
check_tokenService=true
check_utxoStore=true
check_utxosMempool=true
check_wocExchangeRate=true
check_wocStats=true
check_p2pService=true
check_wocChainStats=true
```

**Sync Checker Delays:**

The service allows configuration of acceptable delays for syncing services:

```ini
# Minutes delay before alerting
nodeRetryDelayInMins=1
bstoreSyncCheckerDelayInMins=20
utxoStorSyncCheckerDelayInMins=5
```

## Security

**gRPC Services:**

Most gRPC services run without authentication (securityLevel=0 by default). These services should NOT be exposed to the Internet.

**API Key Authentication:**

When enabled, the service uses account-manager for API key validation:

```ini
apiKeyCheckEnabled=true         # Validate API keys
apiKeyRateLimitEnabled=true     # Enforce rate limits
apiKeySaveActivity=true         # Track API usage

apiKeyDefaultRateLimitPerSec=3
apiKeyDefaultRateLimitPerDay=100000

# Account manager requires mTLS in production
accountManagerSecurityLevel=1
accountManagerCertFile=/path/to/client.crt
accountManagerKeyFile=/path/to/client.key
accountManagerCaCertFile=/path/to/cafile.pem
```

**Bitcoin Node Authentication:**

Bitcoin RPC requires basic authentication:

```ini
BSV_username=bitcoin
BSV_password=<secure-password>
```

**Network Security Recommendations:**

- Run gRPC services on private network only
- Only expose HTTP API ports (8084) publicly
- Use reverse proxy (nginx) with TLS termination for public access
- Keep MongoDB, Redis, PostgreSQL, Elasticsearch on private network
- Use firewall rules to restrict access to gRPC services
- Enable API key authentication for production deployments
- Use mTLS for account-manager communication in production

**Secrets Management:**

- All credentials stored in `settings_local.conf` (should be in `.gitignore`)
- Never commit `settings_local.conf` with real credentials
- Use environment-specific overrides (`.live`, `.test`, etc.)
- Rotate Bitcoin RPC passwords regularly
- Keep TLS certificates secure and rotate before expiry

## Configuration

### Configuration Files

The service uses a hierarchical configuration system:

- **`settings.conf`** - Empty template/defaults
- **`settings_local.conf`** - Local environment configuration 
- **`configs/configs.go`** - Configuration struct and loader

### Environment-Specific Overrides

Configuration supports environment-specific overrides using suffixes:

```ini
# settings_local.conf example
BSV_host=localhost
BSV_host.stage=172.20.16.40
BSV_host.live.hz=172.20.16.69
BSV_host.live.ovh=172.21.21.12

# The service loads based on SETTINGS_CONTEXT environment variable
# Or uses default values without suffix
```

Common suffixes:
- `.test` - Test environment
- `.live.hz` - Production HZ datacenter
- `.live.ovh` - Production OVH datacenter

### Key Configuration Sections

**Server Configuration:**
```ini
host=
port=8084
fiber_port=8085
log_level=debug
prettify_log=false
writeTimeout=200s
readTimeout=120s
idleTimeout=60s
```

**Network Configuration:**
```ini
isMainnet=true
network=mainnet
genesisBlock=000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f
```

**Data Source Configuration:**
```ini
# Bitcoin Node
BSV_host=localhost
BSV_port=8332
BSV_username=bitcoin
BSV_password=bitcoin
BSV_timeout=5000

# MongoDB (optional, can be disabled)
mongoCache=true
mongoHost=localhost
mongoPort=27017
mongoDatabase=bsv
mongoUsername=bsvuser
mongoPassword=bsvpwd99

# Redis (optional, can be disabled)
redis_cache_enabled=true
redis_host=localhost
redis_port=6379
redis_db=3

# PostgreSQL
db_host=localhost
db_port=5432
db_name=bsv
db_user=bsv
db_password=bsv

# Elasticsearch
es_url=http://localhost:9200
es_indexname=woc_index
opReturnSearch=true

# ElectrumX
electrumUrl=localhost:50001
```

**gRPC Services Configuration:**
```ini
# bstore (required)
bstoreEnabled=true
bstoreAddress=localhost:7021
bstoreSecurityLevel=0
bstoreGrpcMaxCallRecvMsgSize_mb=500

# woc-stats (optional)
woc_stats_enabled=true
woc_stats_address=localhost:7020

# woc-exchange-rate (optional)
woc_exchange_rate_enabled=true
woc_exchange_rate_address=localhost:7021

# p2p-service (optional)
p2p_service_enabled=true
p2p_service_address=localhost:7040

# utxo-store (optional)
utxoStoreEnabled=true
utxoStoreAddress=localhost:7031

# utxos-mempool (optional)
utxosMempoolEnabled=true
utxosMempoolAddress=localhost:7213

# token-service (optional)
tokenServiceAddress=localhost:7006
tokenServiceSecurityLevel=0

# token-mempool (optional)
tokenMempoolAddress=localhost:7007
tokenMempoolSecurityLevel=0

# account-manager (for API keys)
accountManagerAddress=localhost:7001
accountManagerSecurityLevel=0
apiKeyCheckEnabled=false
apiKeyRateLimitEnabled=false
```

**Feature Flags:**
```ini
# Caching
mongoCache=true
precacheNewBlocks=false

# OP_RETURN Search
opReturnSearch=true
nonFinalMempoolSearchEnabled=true

# TAAL Bitcoin Proxy
taalBitcoinProxyEnabled=false
taalTapiURL=https://api.taal.com/api/v1/bitcoin
taalTapiKey=<your-key>

# Merkle Proof Service
woc_merkle_service_enabled=true
woc_merkle_service_address=http://localhost:8890

# Block Headers Saving
block_headers_save_enabled=false
block_headers_save_latest_enabled=false
block_headers_path=../data/block_headers

# Homepage Stats Caching
home_page_stats_cache_enabled=false
home_page_stats_cache_expiry=86400
```

**API Key & Offline Mode Feature Toggles:**

| Setting                  | Default                   | Description                                                                                                                                                           |
|--------------------------|---------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `offlineMode`            | `false`                   | When `true`, disables all account-manager gRPC connections. API keys are read only from `lastKnownAPIKeys.json` cache file. Activity store is automatically disabled. |
| `apiKeyCheckEnabled`     | `true`                    | When `true`, validates API keys on every request. Requires account-manager connection (unless `offlineMode=true`).                                                    |
| `apiKeyRateLimitEnabled` | `true`                    | When `true`, enforces per-second and per-day rate limits. Requires Redis.                                                                                             |
| `apiKeySaveActivity`     | `true`                    | When `true`, logs API activity to account-manager. Automatically set to `false` when `offlineMode=true`.                                                              |
| `apiKeyCacheDestination` | `./lastKnownAPIKeys.json` | Path to the API key cache file used by offline mode.                                                                                                                  |

**Behavior Matrix:**

| Mode                         | API Key Source           | On-demand Lookup          | Activity Logging | Rate Limiting               |
|------------------------------|--------------------------|---------------------------|------------------|-----------------------------|
| Normal (`offlineMode=false`) | gRPC stream + cache file | Yes (gRPC fallback)       | If enabled       | If enabled                  |
| Offline (`offlineMode=true`) | Cache file only          | No (returns unauthorized) | Disabled         | If enabled (requires Redis) |

**API Key Cache File Format:**

The `lastKnownAPIKeys.json` file uses protobuf JSON serialization with snake_case field names. The map key format is `{network}_{apikey_value}`:

```json
{
  "mainnet_fff6f33959a139a45206d054e07fd0d4": {
    "apikey_id": 2452,
    "network": "mainnet",
    "value": "fff6f33959a139a45206d054e07fd0d4",
    "account_id": 992,
    "standard_fee_id": 54,
    "data_fee_id": 55,
    "apikey": "mainnet_fff6f33959a139a45206d054e07fd0d4",
    "is_account_active": true,
    "woc_access": true,
    "woc_rate_limit": 100,
    "woc_daily_rate_limit": 100000,
    "Fees": [
      {
        "id": 54,
        "fee_type": "standard",
        "mining_fee": { "satoshis": 1, "bytes": 1000 },
        "relay_fee": { "satoshis": 1, "bytes": 1000 }
      },
      {
        "id": 55,
        "fee_type": "data",
        "mining_fee": { "satoshis": 1, "bytes": 1000 },
        "relay_fee": { "satoshis": 1, "bytes": 1000 }
      }
    ],
    "package": {}
  }
}
```

| Field                  | Type    | Description                                      |
|------------------------|---------|--------------------------------------------------|
| `apikey_id`            | int     | Unique identifier for the API key                |
| `account_id`           | int     | Account ID (must not be 1, which is public)      |
| `is_account_active`    | bool    | Must be `true` for the key to be valid           |
| `network`              | string  | `"mainnet"` or `"testnet"`                       |
| `value`                | string  | The API key value (without network prefix)       |
| `apikey`               | string  | Full API key with network prefix                 |
| `woc_access`           | bool    | Whether WoC access is enabled                    |
| `woc_rate_limit`       | int     | Requests per second limit                        |
| `woc_daily_rate_limit` | int     | Requests per day limit                           |
| `standard_fee_id`      | int     | ID for standard transaction fees                 |
| `data_fee_id`          | int     | ID for data transaction fees                     |
| `Fees`                 | array   | Fee configurations for mining and relay          |
| `package`              | object  | Package/subscription details                     |
| `revoked`              | object  | Optional timestamp; if present, key is revoked   |
| `expiry`               | object  | Optional timestamp; if present and past, expired |

### Minimal Local Development Configuration

For local development with minimal dependencies:

```ini
# settings_local.conf
host=
port=8084
fiber_port=8085
log_level=debug
prettify_log=true

# Disable caching
mongoCache=false
redis_cache_enabled=false

# Bitcoin Node (required)
BSV_host=localhost
BSV_port=18332
BSV_username=bitcoin
BSV_password=bitcoin

# bstore (required)
bstoreEnabled=true
bstoreAddress=localhost:7021

# Disable optional services
woc_stats_enabled=false
woc_exchange_rate_enabled=false
p2p_service_enabled=false
utxoStoreEnabled=false
utxosMempoolEnabled=false
opReturnSearch=false

# Disable health checks for missing services
check_mongoDB=false
check_electrumX=false
check_sockets=false
check_tokenMempool=false
check_tokenService=false
check_utxoStore=false
check_utxosMempool=false
check_wocExchangeRate=false
check_wocStats=false
check_p2pService=false
check_wocChainStats=false

# Disable API key authentication
apiKeyCheckEnabled=false
apiKeyRateLimitEnabled=false
```

## Operations

### Running the Service

**Development:**
```bash
# Run with pretty logging
make run

# Build binary
make build

# Run tests
make unit_test

# Lint code
make lint
```

**Production:**
```bash
# Build with version info
VERSION=1.2.3 make build

# Run the service
./artifacts/svc

# Or via systemd
systemctl start woc-api
systemctl enable woc-api
```

**Docker:**
```bash
# Build image
docker build -t woc-api:latest .

# Run container
docker run -d \
  -p 8084:8084 \
  -v /path/to/settings_local.conf:/app/settings_local.conf \
  woc-api:latest
```

### Monitoring

**Logs:**
- Structured JSON logging via `zap`
- Log levels: debug, info, warn, error
- All logs include timestamp, service name, version, commit

**Production Logging:**
```bash
# Follow logs
journalctl -u woc-api -f

# Search for errors
journalctl -u woc-api --since "1 hour ago" | grep ERROR

# JSON log parsing
journalctl -u woc-api -o cat | jq 'select(.level=="error")'
```

**Key Metrics to Monitor:**
- Health check status for all components
- Bitcoin node sync status and block height
- bstore sync delay (`bstoreSyncCheckerDelayInMins`)
- API response times
- Error rates per endpoint
- Memory usage (can spike on large queries)
- MongoDB/Redis cache hit rates
- API key rate limit violations (if enabled)

### Deployment Considerations

**Resource Requirements:**
- Memory: 2-8 GB depending on query load
- CPU: 2-4 cores recommended
- Disk: Minimal (unless MongoDB caching enabled, then depends on cache size)
- Network: High bandwidth for blockchain data queries

**Scaling:**
- Service is stateless (except optional MongoDB/Redis cache)
- Can be horizontally scaled behind load balancer
- Share MongoDB and Redis instances across multiple instances
- Each instance connects to same backend services (bstore, woc-stats, etc.)

**High Availability:**
- Run multiple instances behind load balancer
- Monitor health endpoint for automated failover
- Ensure backend services (bstore, Bitcoin node) have redundancy
- Consider geographic distribution for global latency reduction

## Links

- **Documentation:** [WhatsOnChain Flow Diagram](https://teranode.atlassian.net/wiki/spaces/WoC/pages/16056411/WhatsOnChain+-+Flow+Diagram)
- **API Documentation:** Check `/woc/v1/bsv/main/` endpoints for API reference
- **Repository:** `github.com/teranode-group/woc-api` 
