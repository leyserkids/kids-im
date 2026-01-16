# OpenIM 消息顺序与去重机制分析

## 概述

OpenIM 通过 **5 层保序机制** 和 **4 层去重防护** 确保消息在分布式环境下的顺序一致性和不重复。

---

## 一、核心��题

### 1.1 消息乱序

```
用户快速发送 3 条消息：
  "你好" → "在吗" → "？"

如果没有顺序保证，对方可能收到：
  "在吗" → "？" → "你好" ❌ 完全错乱
```

### 1.2 消息重复

```
客户端发送消息 → 网络超时 → 客户端重试
→ 服务端可能收到两次相同消息

结果：
  "你好"
  "你好" ❌ 重复显示
```

---

## 二、消息顺序保证机制（5 层）

### 第 0 层：客户端串行发送

**核心设计**：Channel 队列 + 单协程发送

```go
// 客户端架构（伪代码）
type LongConnMgr struct {
    sendChannel chan Message  // 消息队列（容量 10）
}

// 用户发送消息
func (m *LongConnMgr) SendMessage(content string) {
    msg := createMessage(content)
    m.sendChannel <- msg  // 放入队列，立即返回
}

// writePump：单协程串行发送
func (m *LongConnMgr) writePump() {
    for {
        msg := <-m.sendChannel  // 从队列中按顺序取消息
        m.websocket.Send(msg)   // 串行发送到 WebSocket
    }
}
```

**关键特性**：

| 特性 | 说明 |
|------|------|
| Channel 队列 | 消息按入队顺序排队 |
| 单协程发送 | writePump 独占 WebSocket，避免并发乱序 |
| 异步非阻塞 | 用户发送立即返回，不影响体验 |

---

### 第 1 层：WebSocket/TCP 传输保序

**核心原理**：TCP 字节流有序特性

- 单个 WebSocket 连接内消息天然有序
- 即使底层 IP 包乱序，TCP 也会重组后按顺序交付
- 只要客户端使用同一个连接，就不会乱序

---

### 第 2 层：Gateway 串行接收

**代码位置**：`internal/msggateway/client.go`

```go
func (c *Client) readMessageLoop() {
    for {
        // 单协程串行读取（不会并发）
        msgType, data := c.websocket.ReadMessage()
        c.handleMessage(data)  // 转发到 Msg RPC Server
    }
}
```

---

### 第 3 层：Kafka Hash 分区保序

**代码位置**：`pkg/common/storage/kafka/producer.go`

```go
func SendToKafka(conversationID string, message *Message) {
    kafkaMsg := &sarama.ProducerMessage{
        Topic: "toRedis",
        Key:   sarama.StringEncoder(conversationID),  // 会话ID作为分区键
        Value: serialize(message),
    }
    // Hash 分区器：相同 key 路由到同一分区
    producer.SendMessage(kafkaMsg)
}
```

**分区路由**：
```
hash("userA_userB") % 10 = 3 → Partition-3
Partition-3 内部保证有序存储
```

---

### 第 4 层：Batcher 分片保序

**代码位置**：`internal/msgtransfer/online_history_msg_handler.go`

```go
type Batcher struct {
    workers []*Worker  // 多个 Worker 协程
}

func (b *Batcher) sharding(conversationID string) int {
    hash := fnv.New32()
    hash.Write([]byte(conversationID))
    return int(hash.Sum32()) % len(b.workers)  // 固定路由到某个 Worker
}
```

**保序关键**：
- 同一 conversationID 的消息始终由同一 Worker 处理
- Worker 内部串行处理，不会并发

---

### 第 5 层：Seq 原子分配保序

**代码位置**：`pkg/common/storage/cache/redis/seq_conversation.go`

```lua
-- Seq 分配 Lua 脚本
local curr = tonumber(redis.call("HGET", key, "CURR"))
local new_curr = curr + size
redis.call("HSET", key, "CURR", new_curr)
return {0, curr}  -- 返回起始 seq
```

**消息发送时分配 Seq**：

```go
func BatchInsertMessages(conversationID string, messages []Message) {
    // 原子分配 Seq
    startSeq := seqCache.Malloc(conversationID, len(messages))

    // 为每条消息分配递增的 Seq
    currentSeq := startSeq
    for i := range messages {
        currentSeq++
        messages[i].Seq = currentSeq  // 严格递增
    }
}
```

---

### 完整保序链条

```
用户点击发送
  ↓ 第 0 层：客户端 Channel 串行发送
  ↓ 第 1 层：WebSocket/TCP 字节流有序
  ↓ 第 2 层：Gateway 单协程接收
  ↓ 第 3 层：Kafka Hash 分区有序
  ↓ 第 4 层：Batcher 固定 Worker 处理
  ↓ 第 5 层：Seq 原子递增分配
最终存储：顺序正确 ✅
```

---

## 三、消息去重保证机制（4 层）

### 第 1 层：ClientMsgID 客户端去重

```go
func GenerateClientMsgID() string {
    return fmt.Sprintf("%d-%s-%s",
        time.Now().UnixMilli(),  // 毫秒时间戳
        randomString(8),         // 随机字符串
        deviceID,                // 设备 ID
    )
}
```

**去重逻辑**：
- 客户端重试时使用相同 ClientMsgID
- SDK 收到消息检查本地数据库，如已存在则丢弃

---

### 第 2 层：ServerMsgID 服务端唯一标识

**代码位置**：`internal/rpc/msg/verify.go`

```go
func GenerateServerMsgID(senderID string) string {
    timestamp := time.Now().Format("2006-01-02 15:04:05")
    random := rand.Int31()
    raw := fmt.Sprintf("%s-%s-%d", timestamp, senderID, random)
    return md5Hash(raw)
}
```

---

### 第 3 层：Seq 唯一性

同一 conversationID 的 Seq 严格递增，绝不重复：

```
会话 "userA_userB" 的 Seq 序列：
  1, 2, 3, 4, 5, 6, 7, 8, 9, 10...

不可能出现：
  1, 2, 3, 5, 5, 6...  ❌ seq=5 重复
```

---

### 第 4 层：MongoDB 唯一索引 + 幂等更新

**代码位置**：`pkg/common/storage/database/mgo/msg.go`

```go
// 创建唯一索引
collection.CreateIndex(mongo.IndexModel{
    Keys:    bson.D{{Key: "doc_id", Value: 1}},
    Options: options.Index().SetUnique(true),
})
```

**幂等更新**：

```go
func InsertOrUpdate(doc *MsgDocument) error {
    _, err := collection.InsertOne(ctx, doc)
    if mongo.IsDuplicateKeyError(err) {
        // 文档已存在，执行更新（幂等）
        collection.UpdateOne(ctx,
            bson.M{"doc_id": doc.DocID},
            bson.M{"$set": doc},
        )
        return nil
    }
    return err
}
```

**效果**：
```
第一次处理：insert(doc_id="userA_userB:0") → 成功
第二次处理：insert(doc_id="userA_userB:0") → DuplicateKeyError → update → 成功（幂等）
结果：数据库中只有一份数据
```

---

## 四、关键代码位置

### 消息顺序相关

| 层级 | 机制 | 文件路径 |
|------|------|---------|
| 第 0 层 | 客户端 Channel | `openim-sdk-core/.../long_conn_mgr.go` |
| 第 2 层 | Gateway 串行接收 | `internal/msggateway/client.go` |
| 第 3 层 | Kafka 分区键 | `pkg/common/storage/kafka/producer.go` |
| 第 4 层 | Batcher 分片 | `internal/msgtransfer/online_history_msg_handler.go` |
| 第 5 层 | Seq Lua 脚本 | `pkg/common/storage/cache/redis/seq_conversation.go` |

### 消息去重相关

| 层级 | 机制 | 文件路径 |
|------|------|---------|
| 第 1 层 | ClientMsgID 去重 | `openim-sdk-core/.../message_check.go` |
| 第 2 层 | ServerMsgID 生成 | `internal/rpc/msg/verify.go` |
| 第 4 层 | MongoDB 唯一索引 | `pkg/common/storage/database/mgo/msg.go` |
| 第 4 层 | 幂等更新 | `pkg/common/storage/controller/msg.go` |

---

## 五、总结

### 核心设计理念

1. **源头保序**：客户端就开始保证顺序，不依赖服务端排序
2. **多层防护**：每一层都有独立的保序/去重机制
3. **原子操作**：关键操作（Seq 分配、数据库插入）都保证原子性
4. **幂等设计**：允许重复处理，但保证结果一致

### 设计精髓

```
消息顺序 = 5 层保障
├─ 第 0 层：客户端 Channel 串行发送
├─ 第 1 层：WebSocket/TCP 字节流有序
├─ 第 2 层：Gateway 单协程接收
├─ 第 3 层：Kafka Hash 分区有序
├─ 第 4 层：Batcher 固定 Worker 处理
└─ 第 5 层：Seq 原子递增分配

消息去重 = 4 层防护
├─ 第 1 层：ClientMsgID（客户端去重）
├─ 第 2 层：ServerMsgID（全局唯一标识）
├─ 第 3 层：Seq 唯一性（业务层保证）
└─ 第 4 层：MongoDB 唯一索引 + 幂等更新
```
