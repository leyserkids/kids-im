# OpenIM REST API 与 WebSocket 设计分析

## 概述

OpenIM 采用 **REST API + WebSocket 双通道架构** 进行数据交互：
- **REST API**：处理低频的 CRUD 操作、管理操作、批量数据
- **WebSocket**：处理高频的实时消息通信

---

## 一、REST API 端点概览

REST API 使用 Gin 框架，统一采用 POST 方法（少数 GET 端点除外）。

### 1.1 主要模块

| 模块 | 前缀 | 主要功能 |
|------|------|----------|
| 用户管理 | `/user` | 注册、信息更新、在线状态、订阅 |
| 好友管理 | `/friend` | 添加/删除好友、黑名单、备注 |
| 群组管理 | `/group` | 创建/解散群、成员管理、禁言 |
| 认证 | `/auth` | Token 获取/解析、强制登出 |
| 消息 | `/msg` | 发送、���回、搜索、已读标记 |
| 会话 | `/conversation` | 会话列表、设置、增量同步 |
| 第三方服务 | `/third` | 文件上传、推送 Token、日志 |
| 统计 | `/statistics` | 用户/群组注册和活跃统计 |

### 1.2 核心 API 示例

**消息相关**：
```
POST /msg/send_msg              # 发送消息
POST /msg/pull_msg_by_seq       # 按序列号拉取消息
POST /msg/revoke_msg            # 撤回消息
POST /msg/mark_msgs_as_read     # 标记已读
POST /msg/search_msg            # 搜索消息
```

**认证相关**：
```
POST /auth/get_user_token       # 获取用户 Token
POST /auth/parse_token          # 解析 Token
POST /auth/force_logout         # 强制登出
```

**会话相关**：
```
POST /conversation/get_sorted_conversation_list  # 获取排序会话列表
POST /conversation/get_incremental_conversations # 增量同步会话
```

---

## 二、WebSocket 通信设计

### 2.1 连接握手

```
WebSocket URL: ws://host:port/?sendID=xxx&platformID=1&token=xxx
```

**URL 参数**：
| 参数 | 说明 |
|------|------|
| sendID | 用户 ID |
| platformID | 平台 ID |
| token | 认证令牌 |
| compression | 是否启用 gzip 压缩 |
| isBackground | 是否后台运行 |

### 2.2 消息类型 (ReqIdentifier)

**客户端请求类型 (1000-1999)**：

| ID | 常量名 | 功能 |
|----|--------|------|
| 1001 | `WSGetNewestSeq` | 获取最新序列号 |
| 1002 | `WSPullMsgBySeqList` | 按序列号拉取消息 |
| 1003 | `WSSendMsg` | 发送消息 |
| 1005 | `WSPullMsg` | 拉取消息 |
| 1006 | `WSGetConvMaxReadSeq` | 获取会话已读序列号 |

**客户端控制类型 (2000+)**：

| ID | 常量名 | 功能 |
|----|--------|------|
| 2003 | `WsLogoutMsg` | 用户登出 |
| 2004 | `WsSetBackgroundStatus` | 设置后台状态 |
| 2005 | `WsSubUserOnlineStatus` | 订阅用户在线状态 |

**服务器推送类型**：

| ID | 常量名 | 功能 |
|----|--------|------|
| 2001 | `WSPushMsg` | 推送消息 |
| 2002 | `WSKickOnlineMsg` | 踢用户下线 |
| 3001 | `WSDataError` | 数据错误通知 |

### 2.3 消息协议

```protobuf
// 请求消息
message Req {
  int32 ReqIdentifier = 1;   // 消息类型标识
  string Token = 2;          // 认证令牌
  string SendID = 3;         // 发送者 ID
  string OperationID = 4;    // 操作 ID
  bytes Data = 6;            // Protobuf 序列化数据
}

// 响应消息
message Resp {
  int32 ReqIdentifier = 1;   // 请求类型标识
  string OperationID = 2;    // 对应操作 ID
  int32 ErrCode = 4;         // 错误码
  string ErrMsg = 5;         // 错误消息
  bytes Data = 6;            // 响应数据
}
```

---

## 三、数据流分析

### 3.1 REST API 数据流

```
客户端 HTTP POST
      ↓
API 路由 (router.go)
      ↓
Handler (msg.go, user.go, group.go...)
      ↓
RPC 调用后端服务
      ↓
返回 JSON 响应
```

### 3.2 WebSocket 消息发送流程

```
客户端发送二进制消息
      ↓
WebSocket 服务器 (ws_server.go)
      ↓ 验证令牌 + Protobuf 反序列化
消息处理器 (message_handler.go)
      ↓
RPC 调用 msg 服务
      ↓
响应序列化 + 返回客户端
```

### 3.3 消息推送流程

```
后端服务发起推送
      ↓
消息网关 (hub_server.go)
      ↓
查询在线用户连接
      ↓
Client.PushMessage()
      ↓ Protobuf 序列化
WebSocket 发送 (WSPushMsg = 2001)
      ↓
客户端接收
```

---

## 四、双通道设计对比

### 4.1 功能划分

| 数据类型 | REST | WebSocket | 说明 |
|----------|:----:|:---------:|------|
| 用户注册/登录 | ✅ | ❌ | 一次性操作 |
| 用户/好友/群组管理 | ✅ | ❌ | CRUD 操作 |
| 会话列表 | ✅ | ❌ | 初始化加载 |
| 消息发送 | ✅ | ✅ | REST 用于服务端，WS 用于客户端 |
| 消息接收/推送 | ❌ | ✅ | 实时性要求 |
| 消息历史拉取 | ✅ | ✅ | REST 批量，WS 增量 |
| 消息搜索 | ✅ | ❌ | 复杂查询 |
| 文件上传 | ✅ | ❌ | HTTP 更适合 |

### 4.2 特性对比

| 特性 | REST API | WebSocket |
|------|----------|-----------|
| 连接模式 | 短连接，请求-响应 | 长连接，双向通信 |
| 状态 | 无状态 | 有状态 |
| 延迟 | 较高（每次握手） | 低（复用连接） |
| 推送能力 | 不支持 | 支持服务端推送 |
| 扩展性 | 易于水平扩展 | 需要连接状态管理 |
| 适用场景 | CRUD、批量处理 | 实时消息、状态同步 |

---

## 五、设计取舍

### 5.1 消息发送同时支持两种方式

```
REST API (POST /msg/send_msg)  → 服务端/管理后台发消息
WebSocket (WSSendMsg = 1003)   → 客户端实时发消息
```

**原因**：
- REST 适用于服务端集成、批量发送
- WebSocket 适用于客户端实时通信，延迟更低

### 5.2 消息接收只用 WebSocket

```
用户在线 → WebSocket 推送 (WSPushMsg = 2001)
用户离线 → 存储 + 离线推送 (APNs/FCM)
```

**原因**：
- 实时性要求高，HTTP 轮询不可接受
- 服务端可主动推送
- 节省带宽和电量

### 5.3 管理操作只用 REST

**原因**：
- 操作频率低
- 无状态设计利于水平扩展
- 便于权限校验和审计
- HTTP 缓存可复用

### 5.4 序列号机制的双通道设计

```
REST:  POST /msg/newest_seq       → 初始同步（登录时）
WS:    WSGetNewestSeq (1001)      → 实时同步（运行时）
WS:    WSPullMsgBySeqList (1002)  → 断点续传
```

---

## 六、性能与可靠性

### 6.1 并发处理

- WebSocket 连接池管理（`sync.Pool`）
- 消息推送使用 `MemoryQueue` 异步处理
- 支持并发请求限制配置

### 6.2 心跳机制

- Web 平台：服务端主动 Ping
- 移动端：客户端主动 Ping
- 读写超时控制
- Pong 响应验证

### 6.3 消息可靠性

- 序列号机制保证有序
- 支持断点续传
- 离线消息补偿
- 消息确认机制

---

## 七、关键代码位置

| 文件路径 | 功能 |
|----------|------|
| `internal/api/router.go` | REST API 路由定义 |
| `internal/api/msg.go` | 消息 REST API 处理 |
| `internal/msggateway/ws_server.go` | WebSocket 服务器 |
| `internal/msggateway/client.go` | 客户端连接管理 |
| `internal/msggateway/message_handler.go` | 消息处理器 |
| `internal/msggateway/hub_server.go` | 消息推送中心 |
| `internal/msggateway/constant.go` | 消息类型常量 |

---

## 八、总结

OpenIM 的双通道设计原则：

| 原则 | 选择 | 应用场景 |
|------|------|----------|
| 实时性优先 | WebSocket | 消息收发、在线状态推送 |
| 可靠性优先 | REST + Seq | 历史消息拉取、离线补偿 |
| 管理操作 | REST | 用户、群组、好友 CRUD |
| 双通道互补 | REST + WS | 消息发送同时支持两种方式 |

通过这种设计，REST API 处理低频管理操作和批量数据，WebSocket 专注于高频实时通信，两者相辅相成。
