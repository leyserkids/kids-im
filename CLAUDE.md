# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a **monorepo** for OpenIM, an open-source instant messaging platform. It's a fork of the upstream OpenIM project with custom enhancements for **unified group/single chat read receipts**.

**Base Versions:**
- openim-protocol: v0.0.72-alpha.78
- openim-server: v3.8.3-patch.3
- openim-sdk-core: v3.8.3-patch.3

## Repository Structure

```
openim/
├── .github/workflows/    # CI/CD workflows (path-based triggers)
├── go.work               # Go workspace for local dependencies
├── protocol/             # Protobuf definitions & shared types (Go)
├── server/               # Backend microservices (Go)
├── sdk-core/             # Cross-platform SDK for iOS/Android/PC/WASM (Go)
├── sdk-js-wasm/          # TypeScript/JavaScript WebAssembly wrapper
└── docs/                 # Technical architecture documentation (Chinese)
```

## Go Workspace

This monorepo uses Go Workspaces (`go.work`) for local dependency resolution. The `protocol` module is automatically used by `server` and `sdk-core` without publishing.

```go
// go.work
go 1.22.7

use (
    ./protocol
    ./server
    ./sdk-core
)
```

**After any changes to protocol, no publishing is needed** - run `go work sync` and rebuild.

## Build Commands

### Full Build (from root)
```bash
go work sync                          # Sync workspace dependencies
cd protocol && mage                   # Generate protobuf code
cd server && mage build               # Build server
cd sdk-core && make build             # Build SDK
cd sdk-js-wasm && npm run build       # Build JS SDK
```

### protocol
```bash
cd protocol
mage                                  # Generate gRPC code and callers
```
Requires: `protoc-gen-go`, `protoc-gen-go-grpc`

### server
```bash
cd server
mage build     # Build all binaries to _output/bin/
mage start     # Start all services (requires docker-compose dependencies)
mage stop      # Stop all services
mage check     # Check service status
go test ./...  # Run tests
```

### sdk-core
```bash
cd sdk-core
make build           # Build for current platform (requires CGO)
make build-multiple  # Build for linux/amd64, linux/arm64
make build-wasm      # Build WebAssembly module
make ios             # Build iOS framework
make android         # Build Android AAR
make test            # Run unit tests
make lint            # Run golangci-lint
```

### sdk-js-wasm
```bash
cd sdk-js-wasm
npm install
npm run build        # Compile TS + bundle, outputs to lib/
npm run test         # Jest tests with coverage
npm run lint         # ESLint with fixes
npm run typecheck    # TypeScript type checking
```

## CI/CD (GitHub Actions)

Workflows use **path-based triggers**:
- `protocol/**` changes trigger: protocol.yml, server.yml, sdk-core.yml
- `server/**` changes trigger: server.yml
- `sdk-core/**` changes trigger: sdk-core.yml
- `sdk-js-wasm/**` changes trigger: sdk-js-wasm.yml

See `.github/workflows/` for details.

## Architecture

```
┌────────────────────────────────────────┐
│     Client Applications (SDK users)     │
└────────────────┬───────────────────────┘
                 │
┌────────────────▼───────────────────────┐
│     sdk-core / sdk-js-wasm             │
│  (WebSocket, local SQLite, encryption) │
└────────────────┬───────────────────────┘
                 │
┌────────────────▼───────────────────────┐
│          API Gateway (Gin)              │
│  REST: /user, /message, /group          │
│  WebSocket: /gateway (msggateway)       │
└────────────────┬───────────────────────┘
                 │ gRPC
┌────────────────▼───────────────────────┐
│           RPC Services                  │
│  msg | user | group | auth | relation   │
│  conversation | third | push            │
└────────────────┬───────────────────────┘
                 │
    ┌────────────┼────────────┐
    ▼            ▼            ▼
┌────────┐  ┌─────────┐  ┌────────┐
│ Redis  │  │ MongoDB │  │ Kafka  │
│(cache) │  │(persist)│  │(queue) │
└────────┘  └─────────┘  └────────┘
```

### Key Microservices (server/cmd/)
- **openim-api**: REST API gateway
- **openim-msggateway**: WebSocket connections for real-time messaging
- **openim-msgtransfer**: Kafka-based message routing
- **openim-push**: Offline push notifications
- **openim-crontask**: Scheduled cleanup tasks
- **openim-rpc/**: Individual RPC services (msg, user, group, auth, relation, conversation, third)

## Key Concepts

### Message Ordering (Seq)
Messages use a `Seq` (sequence number) for ordering and sync. Each conversation has its own Seq counter. Clients sync incrementally by requesting messages since their last known Seq.

### Read Receipts (This Fork's Enhancement)
This fork unifies single chat and group chat read receipt handling:
- **LocalReadCursor**: Stores each member's read position (max_read_seq) per conversation
- **LocalReadState**: Stores `allReadSeq` - the minimum read position among other members
- **Notification types**: `MarkAsReadTips` (2200) for self-sync, `GroupHasReadTips` (2201) for group broadcast

Key files for read receipt logic:
- `sdk-core/internal/conversation_msg/read_drawing.go`
- `sdk-core/internal/conversation_msg/sync.go`
- `server/internal/rpc/msg/as_read.go`
- `protocol/sdkws/sdkws.proto` (GroupHasReadTips)

### Storage
- **Server**: MongoDB (messages by conversation, ~100 msgs/document), Redis (cache/tokens), Kafka (message queue)
- **Client SDK**: SQLite for offline cache, IndexedDB for web via WASM

## Configuration

Server config files in `server/config/*.yml`:
- `share.yml`: Common settings (secret, token TTL)
- `mongodb.yml`, `redis.yml`, `kafka.yml`: Data layer connections
- `openim-api.yml`, `openim-msggateway.yml`, etc.: Per-service settings

## Technical Documentation

The `docs/` directory contains detailed design docs (in Chinese):
- `01-database-design.md`: MongoDB sharding, Seq mechanism
- `02-seq-design.md`: Message ordering via Seq numbers
- `04-message-order-dedup.md`: 5-layer ordering, 4-layer deduplication
- `09-read-receipt.md`: **Key doc for this fork's read receipt design**
