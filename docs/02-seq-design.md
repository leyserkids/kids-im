# OpenIM Seq 序列号设计分析

## 概述

Seq（序列号）是 OpenIM 消息系统的核心机制，用于保证消息全局有序、支持增量同步、实现消息分片存储。本文档深入分析 OpenIM 的 Seq 设计方案。

---

## 一、Seq 要解决的问题

### 1.1 消息全局有序性

在分布式 IM 系统中，多个客户端同时发送消息，需要确保：
- **同一会话中所有消息全局有序**：后发的消息 Seq 必须大于先发的
- **避免 Seq 冲突**：多个服务器节点同时分配 Seq 时不能重复

```
用户A和用户B同时给群聊发消息：
- 服务器1处理A的消息 → 需要分配 Seq 1001
- 服务器2处理B的消息 → 不能也分配 Seq 1001，必须是 1002
```

### 1.2 消息增量同步

客户端需要根据 Seq 拉取消息：
- **客户端记录本地最大 Seq**：如 localMaxSeq = 1000
- **向服务器请求新消息**：拉取 Seq > 1000 的所有消息
- **填补消息空隙**：发现 Seq 不连续时，请求缺失的消息

### 1.3 消息存储分片

MongoDB 需要将海量消息分散存储：
- **按 Seq 范围分片**：每 100 条消息存为一个文档
- **快速定位文档**：通过 Seq 计算文档 ID
- **降低单文档大小**：避免超过 MongoDB 16MB 限制

```
Seq 1-100   → 存入文档 conversation_123:0
Seq 101-200 → 存入文档 conversation_123:1
Seq 201-300 → 存入文档 conversation_123:2
```

### 1.4 用户级消息访问控制

不同用户对同一会话的消息访问权限不同：
- **新加入群组的用户**：只能看到加入后的消息
- **被踢出的用户**：只能看到踢出前的消息
- **清空聊天记录**：更新用户的 minSeq

---

## 二、核心设计：两层 Seq 体系

### 2.1 会话级 Seq（Conversation Seq）

**用途**：管理会话的全局消息序列号

**存储结构**：
```
MongoDB: seq_conversation 集合
{
  conversation_id: "conversation_123",
  max_seq: 10000,    // 当前会话最大 Seq
  min_seq: 1         // 当前会话最�� Seq（历史消息清理）
}

Redis: MALLOC_SEQ:conversation_123 Hash
{
  CURR: 10000,      // 当前已分配到的 Seq
  LAST: 10099,      // 本批次最后可用的 Seq
  TIME: 1703001234, // 分配时间戳
  LOCK: 123456      // 分布式锁值
}
```

### 2.2 用户级 Seq（User Seq）

**用途**：记录用户在特定会话中的消息访问边界

**存储结构**：
```
MongoDB: seq_user 集合
{
  user_id: "user_456",
  conversation_id: "conversation_123",
  min_seq: 600,      // 用户可访问的最小 Seq
  max_seq: 1000,     // 用户可访问的最大 Seq
  read_seq: 850      // 用户已读到的 Seq
}
```

---

## 三、Seq 分配流程

### 3.1 批量预分配策略

为了减少数据库访问，OpenIM 使用批量预分配：

```go
// Seq 分配主函数（伪代码）
func AllocateSeq(conversationID string, count int64) (int64, error) {
    key := "MALLOC_SEQ:" + conversationID

    // 1. 尝试从 Redis 缓存分配
    result := redis.Eval(LuaScriptMalloc, key, count)

    switch result.State {
    case 0:  // 缓存命中，直接分配成功
        return result.CurrSeq, nil

    case 1:  // 缓存未命中，需要从 MongoDB 加载
        mongoSeq := mongo.GetMaxSeq(conversationID)
        allocSize := calculateAllocSize(count)  // 群聊100，单聊50
        newMaxSeq := mongoSeq + allocSize
        mongo.UpdateMaxSeq(conversationID, newMaxSeq)
        redis.SetSeq(key, mongoSeq, newMaxSeq)
        return mongoSeq, nil

    case 2:  // 已被其他进程锁定
        time.Sleep(250 * time.Millisecond)
        return AllocateSeq(conversationID, count)  // 重试

    case 3:  // 缓存 Seq 用尽，需要追加分配
        allocSize := calculateAllocSize(count, result.LastSeq - result.CurrSeq)
        newMaxSeq := result.LastSeq + allocSize
        mongo.UpdateMaxSeq(conversationID, newMaxSeq)
        redis.SetSeq(key, result.CurrSeq, newMaxSeq)
        return result.CurrSeq, nil
    }
}
```

### 3.2 Redis Lua 脚本（原子操作）

**为什么用 Lua**：
- 保证 Redis 操作的原子性
- 一次网络往返完成复杂逻辑
- 避免并发竞态条件

```lua
-- 核心逻辑（简化版）
local key = KEYS[1]
local size = tonumber(ARGV[1])

-- 检查缓存是否存在
if redis.call("EXISTS", key) == 0 then
    local lock = math.random(0, 999999999)
    redis.call("HSET", key, "LOCK", lock)
    return {1, lock}  -- state=1: 需要从 MongoDB 加载
end

-- 检查是否被锁定
if redis.call("HEXISTS", key, "LOCK") == 1 then
    return {2}  -- state=2: 已被其他进程锁定
end

-- 获取当前值
local curr = tonumber(redis.call("HGET", key, "CURR"))
local last = tonumber(redis.call("HGET", key, "LAST"))

-- 检查是否足够分配
if curr + size > last then
    local lock = math.random(0, 999999999)
    redis.call("HSET", key, "LOCK", lock)
    redis.call("HSET", key, "CURR", last)
    return {3, curr, last, lock}  -- state=3: 需要扩容
end

-- 分配成功
redis.call("HSET", key, "CURR", curr + size)
return {0, curr, last}  -- state=0: 成功分配
```

---

## 四、消息发送与拉取流程

### 4.1 消息发送流程

```go
func BatchInsertMessages(conversationID string, messages []Message) (int64, error) {
    // 1. 批量分配 Seq
    startSeq := seqCache.Malloc(conversationID, len(messages))

    // 2. 为每条消息分配 Seq
    userReadMap := make(map[string]int64)
    for i := range messages {
        messages[i].Seq = startSeq + int64(i) + 1
        userReadMap[messages[i].SenderID] = messages[i].Seq
    }

    // 3. 写入 Redis 缓存
    cache.SetMessagesBySeqs(conversationID, messages)

    // 4. 更新用户已读 Seq
    for userID, seq := range userReadMap {
        cache.SetUserReadSeq(conversationID, userID, seq)
    }

    // 5. 发送到 Kafka 异步持久化
    kafka.Send(ToMongoTopic, messages)

    return messages[len(messages)-1].Seq, nil
}
```

### 4.2 消息拉取流程

```go
func PullMessages(conversationID, userID string, startSeq, endSeq int64) ([]Message, error) {
    // 1. 获取会话级边界
    convMinSeq := seqCache.GetConversationMinSeq(conversationID)
    convMaxSeq := seqCache.GetConversationMaxSeq(conversationID)

    // 2. 获取用户级边界
    userMinSeq := seqCache.GetUserMinSeq(conversationID, userID)
    userMaxSeq := seqCache.GetUserMaxSeq(conversationID, userID)

    // 3. 计算有效边界
    effectiveMin := max(convMinSeq, userMinSeq)
    effectiveMax := min(convMaxSeq, userMaxSeq)

    // 4. 校正请求范围
    startSeq = max(startSeq, effectiveMin)
    endSeq = min(endSeq, effectiveMax)

    // 5. 优先从 Redis 读取
    messages := cache.GetMessagesBySeqRange(conversationID, startSeq, endSeq)

    // 6. 缓存未命中则从 MongoDB 补全
    if len(messages) < int(endSeq - startSeq + 1) {
        missingSeqs := findMissingSeqs(startSeq, endSeq, messages)
        dbMessages := mongo.GetMessagesBySeqs(conversationID, missingSeqs)
        messages = merge(messages, dbMessages)
    }

    return messages, nil
}
```

---

## 五、协作组件架构

```
┌─────────────────────────────────────────────────────────┐
│                     RPC 层（对外接口）                      │
├─────────────────────────────────────────────────────────┤
│ • GetConversationMaxSeq    • PullMessageBySeqs          │
│ • GetMaxSeqs              • SetUserConversationMaxSeq   │
│ • GetHasReadSeqs          • SetUserConversationMinSeq   │
└──────────────────┬──────────────────────────────────────┘
                   │
┌──────────────────▼──────────────────────────────────────┐
│              Controller 层（业务逻辑）                      │
├─────────────────────────────────────────────────────────┤
│ • BatchInsertChat2Cache  （消息入库 + Seq分配）           │
│ • GetMsgBySeqsRange      （按 Seq 范围查询）             │
│ • SetHasReadSeqs         （更新已读 Seq）                │
└──────────────────┬──────────────────────────────────────┘
                   │
        ┌──────────┴──────────┐
        │                     │
┌───────▼────────┐   ┌────────▼─────────┐
│  Cache 层       │   │  Database 层     │
│  (Redis)       │   │  (MongoDB)       │
├────────────────┤   ├──────────────────┤
│ • Seq 缓存      │   │ • Seq 持久化     │
│ • 消息缓存      │   │ • 消息持久化     │
│ • 用户 Seq      │   │ • 用户 Seq       │
└────────────────┘   └──────────────────┘
```

---

## 六、方案的取舍

### 6.1 优势

| 优势 | 说明 |
|------|------|
| **高性能** | 批量预分配减少 DB 访问，90% 消息读取命中缓存 |
| **强一致性** | MongoDB 单点递增保证全局有序，分布式锁避免并发冲突 |
| **灵活访问控制** | 用户级 Seq 边界支持群组成员权限管理 |

**性能对比**：
```
传统方案（每条消息访问 DB）：1000 条消息 = 1000 次 DB 查询
OpenIM 方案（批量预分配）：1000 条消息 = 10 次 DB 查询（100条一批）
性能提升：100倍
```

### 6.2 劣势与代价

| 劣势 | 影响 | 降级方案 |
|------|------|----------|
| **高复杂度** | 两层 Seq 体系，双写维护成本高 | - |
| **预分配浪费** | Seq 可能不连续，客户端需处理空洞 | - |
| **Redis 依赖强** | Redis 宕机影响性能 | 回源 MongoDB |
| **单点瓶颈** | 高并发时 MongoDB Seq 分配可能成为瓶颈 | Snowflake |

---

## 七、关键代码位置

| 功能 | 文件路径 | 说明 |
|------|----------|------|
| Seq 分配 | `pkg/common/storage/cache/redis/seq_conversation.go` | Lua 脚本、malloc 函数 |
| 消息入库 | `pkg/common/storage/controller/msg_transfer.go` | 批量分配 Seq |
| 消息拉取 | `pkg/common/storage/controller/msg.go` | 按 Seq 范围查询 |
| RPC 接口 | `internal/rpc/msg/seq.go`、`sync_msg.go` | 对外服务 |
| 数据模型 | `pkg/common/storage/model/seq.go` | Seq 结构定义 |

---

## 八、总结

### Seq 设计精髓

1. **批量预分配**：减少数据库访问，提升性能
2. **双层 Seq 体系**：会话级（全局有序）+ 用户级（访问控制）
3. **Redis + MongoDB**：缓存热数据，持久化冷数据
4. **Lua 脚本原子化**：避免并发竞态

### 设计选择指南

| 规模 | 推荐方案 |
|------|----------|
| 小规模（<1万用户） | 单层 Seq + Redis INCR |
| 中规模（1-10万用户） | 双层 Seq + Lua 脚本 |
| 大规模（>10万用户） | 完整 OpenIM 方案 + 消息队列 |
