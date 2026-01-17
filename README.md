# kids-im

基于 [OpenIM](https://github.com/openimsdk) 的开源即时通讯平台，专注于统一已读回执等增强功能。

## 项目简介

kids-im 是 OpenIM 项目的 fork，保留了原有的高性能分布式架构，同时增加了针对业务场景的定制增强。

**基础版本：**
- openim-protocol: v0.0.72-alpha.78
- openim-server: v3.8.3-patch.3
- openim-sdk-core: v3.8.3-patch.3
- openim-sdk-js-wasm: v3.8.3-patch.3

## 本 Fork 的增强

### 统一单聊/群聊已读回执

重新设计了已读回执机制，统一处理单聊和群聊场景：

- **LocalReadCursor**: 存储每个成员在会话中的已读位置 (max_read_seq)
- **LocalReadState**: 存储 `allReadSeq` - 其他成员中的最小已读位置
- **通知类型**: `MarkAsReadTips` (2200) 用于自身同步，`GroupHasReadTips` (2201) 用于群组广播

详细设计文档见 `docs/09-read-receipt.md`。

## 仓库结构

```
kids-im/
├── protocol/         # Protobuf 定义与共享类型 (Go)
├── server/           # 后端微服务 (Go)
├── sdk-core/         # 跨平台 SDK for WASM (Go)
├── sdk-js-wasm/      # TypeScript/JavaScript WebAssembly 封装
└── docs/             # 技术架构文档
```

## 环境要求

**Go 版本：**
- go 1.22.7+

**Protocol 构建依赖：**
- protoc v5.26.0
- protoc-gen-go-grpc v1.6.0

## 快速开始

### 构建 Protocol
```bash
cd protocol && ./gen.sh
```

### 构建 Server
```bash
cd server && mage build
```

### 构建 SDK
```bash
cd sdk-core && make build-wasm
cd sdk-js-wasm && npm install && npm run build
```

### 运行 Server
```bash
cd server
mage start    # 启动所有服务（需要 docker-compose 依赖）
mage stop     # 停止所有服务
mage check    # 检查服务状态
```

## 架构概览

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

## 许可证

本项目基于 [Apache License 2.0](LICENSE) 开源。

```
Copyright 2024 kids-im Contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
```

## 致谢

本项目基于以下 OpenIM 开源项目（Apache 2.0 许可证）：

- [openimsdk/protocol](https://github.com/openimsdk/protocol) - Protobuf 协议定义
- [openimsdk/open-im-server](https://github.com/openimsdk/open-im-server) - 服务端实现
- [openimsdk/openim-sdk-core](https://github.com/openimsdk/openim-sdk-core) - 核心 SDK
- [openimsdk/openim-sdk-js-wasm](https://github.com/openimsdk/openim-sdk-js-wasm) - JavaScript/WASM SDK

感谢 OpenIM 社区的卓越工作。
