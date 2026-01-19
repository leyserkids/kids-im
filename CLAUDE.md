# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a **monorepo** for `kids-im`, an open-source instant messaging platform. It's a fork of the upstream OpenIM project with custom enhancements for **unified group/single chat read receipts** and more.

For base versions, repository structure, build commands, and architecture overview, see [README.md](README.md).

## CI/CD (GitHub Actions)

The monorepo uses a unified workflow structure:

- **`ci.yml`**: Unified CI workflow with change detection
  - Uses `dorny/paths-filter` to detect which modules changed
  - Runs protocol verification, Go builds (matrix), TypeScript builds, integration tests, Docker tests, and CodeQL
  - Jobs run conditionally based on detected changes
- **`release-server.yml`**: Docker image release for server components
- **`release-wasm.yml`**: WASM SDK release workflow

See `.github/workflows/` for details.

## Key Microservices (server/cmd/)

- **openim-api**: REST API gateway
- **openim-msggateway**: WebSocket connections for real-time messaging
- **openim-msgtransfer**: Kafka-based message routing
- **openim-push**: Offline push notifications
- **openim-crontask**: Scheduled cleanup tasks
- **openim-rpc/**: Individual RPC services (msg, user, group, auth, relation, conversation, third)

## Key Concepts

### Message Ordering (Seq)
Messages use a `Seq` (sequence number) for ordering and sync. Each conversation has its own Seq counter. Clients sync incrementally by requesting messages since their last known Seq.

### Read Receipts
Unified single chat and group chat read receipt handling via `ReadCursor` (per-member read position) and `allReadSeq` (minimum read position among other members).

### Storage
- **Server**: MongoDB (messages by conversation, ~100 msgs/document), Redis (cache/tokens), Kafka (message queue)
- **Client SDK**: SQLite for offline cache, IndexedDB for web via WASM

## Configuration

Server config files in `server/config/*.yml`:
- `share.yml`: Common settings (secret, token TTL)
- `mongodb.yml`, `redis.yml`, `kafka.yml`: Data layer connections
- `openim-api.yml`, `openim-msggateway.yml`, etc.: Per-service settings

## Related Projects

- **Frontend**: The IM frontend repository path can be obtained via the environment variable `PROJECT_PATH_FUJI_FRONTEND`. The IM module is located at `apps/fuji-im` within that repository.

## Technical Documentation

The `docs/` directory contains detailed design docs (in Chinese). See [docs/README.md](docs/README.md) for the full index and recommended reading order.
