## 📖 API Documentation# GoFast Server 🚀

[![Release](https://img.shields.io/github/v/release/armandParser/gofast-server)](https://github.com/armandParser/gofast-server/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/armandParser/gofast-server)](https://golang.org/)
[![License](https://img.shields.io/github/license/armandParser/gofast-server)](LICENSE)

A high-performance, distributed, in-memory cache system built in Go that rivals Redis in speed and functionality.

## ✨ Features

- **🚀 High Performance**: 100k+ operations/second with sub-millisecond latency
- **⚡ Redis-Compatible**: Familiar commands and data structures
- **📊 Multiple Data Types**: Strings, Lists, Sets, Hashes
- **🔄 Pipeline Support**: Batch operations for maximum throughput
- **⏰ TTL Support**: Automatic expiration of keys
- **🔍 Pattern Matching**: KEYS and SCAN operations with wildcard support
- **💾 Persistence**: Optional disk persistence with snapshots
- **📈 Built-in Metrics**: Performance monitoring and statistics

## 📦 Quick Installation

### Download Pre-built Binaries

#### Linux (AMD64)
```bash
wget https://github.com/armandParser/gofast-server/releases/latest/download/gofast-server-linux-amd64.tar.gz
tar -xzf gofast-server-linux-amd64.tar.gz
chmod +x gofast-server-linux-amd64
./gofast-server-linux-amd64 --host=0.0.0.0 --port=6379
```

#### macOS (Intel)
```bash
wget https://github.com/armandParser/gofast-server/releases/latest/download/gofast-server-darwin-amd64.tar.gz
tar -xzf gofast-server-darwin-amd64.tar.gz
chmod +x gofast-server-darwin-amd64
./gofast-server-darwin-amd64 --host=0.0.0.0 --port=6379
```

#### macOS (Apple Silicon)
```bash
wget https://github.com/armandParser/gofast-server/releases/latest/download/gofast-server-darwin-arm64.tar.gz
tar -xzf gofast-server-darwin-arm64.tar.gz
chmod +x gofast-server-darwin-arm64
./gofast-server-darwin-arm64 --host=0.0.0.0 --port=6379
```

#### Windows
1. Download `gofast-server-windows-amd64.zip` from [releases](https://github.com/armandParser/gofast-server/releases)
2. Extract the ZIP file
3. Run `gofast-server-windows-amd64.exe --host=0.0.0.0 --port=6379`

### Build from Source
```bash
git clone https://github.com/armandParser/gofast-server.git
cd gofast-server
make build
./bin/gofast-server --help
```

## 🚀 Usage

### Basic Commands
```bash
# Start server (default: localhost:6379)
./gofast-server

# Custom host and port
./gofast-server --host=0.0.0.0 --port=8080

# High-performance setup
./gofast-server \
  --host=0.0.0.0 \
  --port=6379 \
  --max-memory=8GB \
  --max-clients=50000 \
  --log-level=info

# With persistence
./gofast-server \
  --enable-persist \
  --data-dir=/var/lib/gofast \
  --save-interval=300s

# Show help
./gofast-server --help
```

### Configuration File
```bash
# Copy example config
cp gofast.example.yaml gofast.yaml

# Edit configuration
vim gofast.yaml

# Run with config file
./gofast-server
```

### Environment Variables
```bash
export GOFAST_HOST=0.0.0.0
export GOFAST_PORT=6379
export GOFAST_MAX_MEMORY=4GB
export GOFAST_LOG_LEVEL=info
./gofast-server
```

## 📊 Performance Benchmarks

| Operation | Throughput | P99 Latency |
|-----------|------------|-------------|
| SET       | 120k ops/s | 0.8ms       |
| GET       | 150k ops/s | 0.6ms       |
| MGET      | 200k ops/s | 1.2ms       |
| Pipeline  | 300k ops/s | 2.1ms       |

*Tested on: Intel i7-9700K, 32GB RAM, NVMe SSD*

## 🛠️ Development

### Prerequisites
- Go 1.21 or later
- Make (optional, but recommended)

### Building
```bash
# Build for current platform
make build

# Build for all platforms
make build-all

# Run tests
make test

# Run with race detection
make test-race

# Format and lint
make fmt
make vet

# Show all commands
make help
```

### Project Structure
```
gofast-server/
├── *.go                 # Core server code
├── Makefile            # Build automation
├── gofast.example.yaml # Example configuration
├── go.mod             # Go modules
└── README.md          # This file
```

## 📖 API Documentation

### Supported Commands

#### String Operations
- `SET key value [TTL]` - Set key to value with optional TTL
- `GET key` - Get value of key
- `MGET key1 key2 ...` - Get multiple keys
- `MSET key1 val1 key2 val2 ...` - Set multiple keys
- `INCR key` - Increment integer value
- `DECR key` - Decrement integer value
- `GETSET key newvalue` - Set new value and return old

#### Key Management
- `DEL key` - Delete key
- `EXISTS key` - Check if key exists
- `EXPIRE key seconds` - Set key expiration
- `TTL key` - Get key time to live
- `KEYS pattern` - Find keys matching pattern
- `SCAN cursor [MATCH pattern]` - Iterate over keys

#### List Operations
- `LPUSH key value` - Push to list head
- `RPUSH key value` - Push to list tail
- `LPOP key` - Pop from list head
- `RPOP key` - Pop from list tail
- `LLEN key` - Get list length
- `LINDEX key index` - Get element by index
- `LRANGE key start end` - Get range of elements

#### Set Operations
- `SADD key member` - Add member to set
- `SREM key member` - Remove member from set
- `SMEMBERS key` - Get all set members
- `SCARD key` - Get set cardinality
- `SISMEMBER key member` - Test set membership

#### Hash Operations
- `HSET key field value` - Set hash field
- `HGET key field` - Get hash field
- `HDEL key field` - Delete hash field
- `HGETALL key` - Get all hash fields
- `HLEN key` - Get hash length
- `HEXISTS key field` - Check if hash field exists

#### Advanced
- `PIPELINE commands...` - Execute multiple commands in batch

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## ⭐ Star History

If you find this project useful, please consider giving it a star! ⭐