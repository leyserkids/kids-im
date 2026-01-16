# 消息保留策略设计

## 概述

OpenIM 提供两种消息保留机制：
1. **基于时间**：保留最近 N 天的消息（默认 365 天）
2. **基于数量**：每个会话最多保留 N 条消息（可选）

两种机制结合使用，通过 **minSeq 指针**实现逻辑删除，确保既能控制存储成本，又能保证用户体验。

---

## 一、minSeq 机制

### 1.1 工作原理

OpenIM 使用 `minSeq` 指针标记消息的可见边界，而非物理删除消息：

```
MongoDB 存储:
┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐
│ doc: "conv:0"    │  │ doc: "conv:1"    │  │ doc: "conv:2"    │
│ seq: 1-100       │  │ seq: 101-200     │  │ seq: 201-300     │
└──────────────────┘  └──────────────────┘  └──────────────────┘

seq_conversation 表:
{
  conversation_id: "conv",
  min_seq: 150,    // ← 150 之前的消息对客户端不可见
  max_seq: 300
}

客户端请求 seq=100 的消息:
→ 服务端检查: 100 < minSeq(150)? YES
→ 返回: {seq: 100, status: MsgStatusHasDeleted}
```

### 1.2 minSeq vs 物理删除

| 对比项 | minSeq 指针 | 物理删除 |
|--------|-------------|----------|
| 执行速度 | O(1) 单文档更新 | O(N) 删除多个文档 |
| 存储碎片 | 无 | 会产生碎片 |
| 可恢复性 | 可以（减小 minSeq） | 不可恢复 |
| 实现复杂度 | 简单 | 需要处理边界情况 |

---

## 二、基于时间的保留策略

### 2.1 配置项

```yaml
# config/openim-crontask.yml
cronTask:
  cronExecuteTime: "0 2 * * *"   # 每天凌晨 2 点执行
  retainChatRecords: 365          # 消息保留天数
  fileExpireTime: 180             # 文件保留天数
```

### 2.2 工作原理

```
当前时间: 2024-01-20 02:00:00
retainChatRecords: 365 天
删除截止时间: 2023-01-20 02:00:00

所有 send_time < 2023-01-20 02:00:00 的消息将被清理
```

---

## 三、基于数量的保留策略

### 3.1 配置项

```yaml
# config/openim-crontask.yml
cronTask:
  maxMessagesPerConversation: 0   # 每会话最大消息数 (0=不限制)
  minRetainCount: 200             # 最低保留条数
```

### 3.2 计算公式

```
finalMinSeq = min(
    max(minSeqByTime, minSeqByCount),  // 取时间和数量限制的较严格者
    maxSeq - minRetainCount + 1         // 但不能超过最低保留限制
)

其中:
- minSeqByTime = 超过 retainChatRecords 天的消息 seq
- minSeqByCount = maxSeq - maxMessagesPerConversation + 1
- minRetainCount = 最低保留条数保护
```

### 3.3 优先级

```
1. 最低保留数量 (minRetainCount) - 最高优先级，保护用户体验
2. 时间限制 (retainChatRecords) - 中优先级
3. 数量限制 (maxMessagesPerConversation) - 与时间限制取较严格者
```

---

## 四、清理流程

### 4.1 Cron 定时任务

```
┌───────────────────────────────────────────────────────────────┐
│                    Cron 定时任务 (每天 02:00)                   │
└───────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌───────────────────────────────────────────────────────────────┐
│ 1. 计算删除时间: deltime = now - retainChatRecords 天           │
└───────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌───────────────────────────────────────────────────────────────┐
│ 2. 循环调用 DestructMsgs RPC                                    │
│    - 每次删除 50 个文档（每个文档 100 条消息）                    │
│    - 最多迭代 10,000 次                                         │
└───────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌───────────────────────────────────────────────────────────────┐
│ 3. 对每个被删除的文档:                                          │
│    - 物理删除 MongoDB 文档                                      │
│    - 更新该会话的 minSeq 指针                                   │
└───────────────────────────────────────────────────────────────┘
```

### 4.2 伪代码

```go
func calculateNewMinSeq(ctx context.Context, conversationID string, config Config) int64 {
    maxSeq := db.GetMaxSeq(ctx, conversationID)
    currentMinSeq := db.GetMinSeq(ctx, conversationID)

    // 1. 基于时间的 minSeq
    deltime := time.Now().AddDate(0, 0, -config.RetainChatRecords)
    minSeqByTime := getSeqByTime(conversationID, deltime)

    // 2. 基于数量的 minSeq
    var minSeqByCount int64 = 0
    if config.MaxMessagesPerConversation > 0 {
        minSeqByCount = maxSeq - config.MaxMessagesPerConversation + 1
    }

    // 3. 取两者较大值
    newMinSeq := max(minSeqByTime, minSeqByCount)

    // 4. 但必须保证最低保留数量
    minSeqGuarantee := maxSeq - config.MinRetainCount + 1
    if newMinSeq > minSeqGuarantee {
        newMinSeq = minSeqGuarantee
    }

    // 5. minSeq 只能增大，不能减小
    if newMinSeq > currentMinSeq {
        return newMinSeq
    }
    return currentMinSeq
}
```

---

## 五、场景示例

### 场景 A：高活跃会话

```
配置:
  retainChatRecords: 365 天
  maxMessagesPerConversation: 1000
  minRetainCount: 200

会话状态:
  maxSeq: 50000
  一年前的 seq: 30000

计算:
  1. minSeqByTime = 30000
  2. minSeqByCount = 50000 - 1000 + 1 = 49001
  3. newMinSeq = max(30000, 49001) = 49001
  4. minSeqGuarantee = 50000 - 200 + 1 = 49801
  5. 49001 < 49801 → finalMinSeq = 49001

结果: 保留 seq 49001 ~ 50000，共 1000 条消息
```

### 场景 B：低活跃会话

```
配置:
  retainChatRecords: 365 天
  maxMessagesPerConversation: 1000
  minRetainCount: 200

会话状态:
  maxSeq: 150
  所有消息都在一年内

计算:
  1. minSeqByTime = 0 (没有超过一年的消息)
  2. minSeqByCount = 150 - 1000 + 1 = -849 → 0
  3. newMinSeq = max(0, 0) = 0

结果: 保留全部 150 条消息
```

### 场景 C：最低保留保护

```
配置:
  retainChatRecords: 30 天
  maxMessagesPerConversation: 0 (不限制)
  minRetainCount: 200

会话状态:
  maxSeq: 500
  30天前的 seq: 400

计算:
  1. minSeqByTime = 400
  2. minSeqByCount = 0
  3. newMinSeq = max(400, 0) = 400
  4. minSeqGuarantee = 500 - 200 + 1 = 301
  5. 400 > 301 → finalMinSeq = 301 (保护最低200条)

结果: 保留 seq 301 ~ 500，共 200 条消息
```

---

## 六、配置建议

| 场景 | retainChatRecords | maxMessagesPerConversation | minRetainCount |
|------|-------------------|---------------------------|----------------|
| 默认 | 365 | 0 (不限制) | 200 |
| 存储敏感 | 180 | 500 | 100 |
| 高活跃群 | 365 | 1000 | 200 |

---

## 七、关键代码位置

| 模块 | 文件路径 |
|------|----------|
| 配置定义 | `pkg/common/config/config.go` |
| Cron 任务 | `internal/tools/msg.go` |
| 清理 RPC | `internal/rpc/msg/clear.go` |
| minSeq 存储 | `pkg/common/storage/database/mgo/seq_conversation.go` |
| 消息控制器 | `pkg/common/storage/controller/msg.go` |
| 配置文件 | `config/openim-crontask.yml` |
