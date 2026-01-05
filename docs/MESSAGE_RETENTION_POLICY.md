# 消息保留策略设计文档

## 概述

本文档描述 OpenIM Kids 的消息保留策略，包括：
1. **现有机制**：基于时间的消息保留（默认 365 天）
2. **新增机制**：基于数量的最低保留限制（如至少保留 200 条）

两种机制结合使用，确保既能控制存储成本，又能保证用户体验。

---

## 1. 现有机制：基于时间的消息保留

### 1.1 配置项

| 配置项 | 文件位置 | 默认值 | 说明 |
|--------|----------|--------|------|
| `retainChatRecords` | `config/openim-crontask.yml` | 365 | 消息保留天数 |
| `cronExecuteTime` | `config/openim-crontask.yml` | `0 2 * * *` | 清理任务执行时间（每天凌晨2点） |
| `fileExpireTime` | `config/openim-crontask.yml` | 180 | 文件保留天数 |

### 1.2 工作原理

```
当前时间: 2024-01-20 02:00:00
retainChatRecords: 365 天
删除截止时间: 2023-01-20 02:00:00

所有 send_time < 2023-01-20 02:00:00 的消息将被清理
```

### 1.3 清理流程

```
┌───��─────────────────────────────────────────────────────────────┐
│                    Cron 定时任务 (每天 02:00)                    │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 1. 计算删除时间: deltime = now - retainChatRecords 天            │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 2. 循环调用 DestructMsgs RPC                                     │
│    - 每次删除 50 个文档（每个文档 100 条消息）                     │
│    - 最多迭代 10,000 次                                          │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 3. 对每个被删除的文档:                                           │
│    - 物理删除 MongoDB 文档                                       │
│    - 更新该会话的 minSeq 指针                                    │
└─────────────────────────────────────────────────────────────────┘
```

### 1.4 关键代码位置

| 功能 | 文件路径 |
|------|----------|
| 配置定义 | `pkg/common/config/config.go:117-122` |
| Cron 任务 | `internal/tools/msg.go:deleteMsg()` |
| RPC 实现 | `internal/rpc/msg/clear.go:DestructMsgs()` |
| MongoDB 操作 | `pkg/common/storage/database/mgo/msg.go:GetRandBeforeMsg()` |

---

## 2. 新增机制：基于数量的最低保留限制

### 2.1 需求背景

在某些场景下，仅基于时间的清理策略可能导致问题：

| 场景 | 问题 |
|------|------|
| 低活跃会话 | 一年内只有 50 条消息，全部被保留，没问题 |
| 高活跃会话 | 一年内有 10 万条消息，占用大量存储 |
| 新建会话 | 刚建立的会话消息较少，需要保护 |

**目标**：在时间限制的基础上，增加数量限制，但同时保证每个会话至少保留 N 条消息。

### 2.2 设计方案

#### 方案概述

```
清理策略 = max(基于时间的 minSeq, 基于数量的 minSeq)
          BUT
保留数量 >= minRetainCount (最低保留条数)
```

#### 配置项设计

```yaml
# config/openim-crontask.yml
cronTask:
  cronExecuteTime: "0 2 * * *"
  retainChatRecords: 365        # 现有：保留天数
  maxMessagesPerConversation: 0 # 新增：每会话最大消息数 (0=不限制)
  minRetainCount: 200           # 新增：最低保留条数
```

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `maxMessagesPerConversation` | int | 0 | 每个会话最多保留消息数，0 表示不限制 |
| `minRetainCount` | int | 200 | 每个会话最少保留消息数 |

### 2.3 清理逻辑

```go
// 伪代码：计算会话的新 minSeq
func calculateNewMinSeq(ctx context.Context, conversationID string, config Config) int64 {
    maxSeq, _ := db.GetMaxSeq(ctx, conversationID)
    currentMinSeq, _ := db.GetMinSeq(ctx, conversationID)

    // 当前消息总数
    totalMessages := maxSeq - currentMinSeq + 1

    // 1. 基于时间的 minSeq（现有逻辑）
    deltime := time.Now().AddDate(0, 0, -config.RetainChatRecords)
    minSeqByTime := getSeqByTime(conversationID, deltime)

    // 2. 基于数量的 minSeq（新增逻辑）
    var minSeqByCount int64 = 0
    if config.MaxMessagesPerConversation > 0 {
        minSeqByCount = maxSeq - config.MaxMessagesPerConversation + 1
    }

    // 3. 取两者较大值（更严格的限制）
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

### 2.4 场景示例

#### 场景 A：高活跃会话

```
配置:
  retainChatRecords: 365 天
  maxMessagesPerConversation: 1000
  minRetainCount: 200

会话状态:
  maxSeq: 50000
  currentMinSeq: 1
  一年前的 seq: 30000

计算过程:
  1. minSeqByTime = 30000 (一年前)
  2. minSeqByCount = 50000 - 1000 + 1 = 49001
  3. newMinSeq = max(30000, 49001) = 49001
  4. minSeqGuarantee = 50000 - 200 + 1 = 49801
  5. 49001 < 49801, 所以 finalMinSeq = 49001

结果: 保留 seq 49001 ~ 50000，共 1000 条消息
```

#### 场景 B：低活跃会话

```
配置:
  retainChatRecords: 365 天
  maxMessagesPerConversation: 1000
  minRetainCount: 200

会话状态:
  maxSeq: 150
  currentMinSeq: 1
  所有消息都在一年内

计算过程:
  1. minSeqByTime = 0 (没有超过一年的消息)
  2. minSeqByCount = 150 - 1000 + 1 = -849 → 0 (负数取0)
  3. newMinSeq = max(0, 0) = 0
  4. minSeqGuarantee = 150 - 200 + 1 = -49 → 0

结果: minSeq 保持为 1，保留全部 150 条消息
```

#### 场景 C：中等活跃会话 + 严格时间限制

```
配置:
  retainChatRecords: 30 天
  maxMessagesPerConversation: 0 (不限制)
  minRetainCount: 200

会话状态:
  maxSeq: 500
  currentMinSeq: 1
  30天前的 seq: 400

计算过程:
  1. minSeqByTime = 400 (30天前)
  2. minSeqByCount = 0 (不限制)
  3. newMinSeq = max(400, 0) = 400
  4. minSeqGuarantee = 500 - 200 + 1 = 301
  5. 400 > 301, 所以 finalMinSeq = 301 (保护最低200条)

结果: 保留 seq 301 ~ 500，共 200 条消息
       (而不是只保留 100 条)
```

### 2.5 流程图

```
┌─────────────────────────────────────────────────────────────────┐
│                    Cron 定时任务 (每天 02:00)                    │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                   遍历所有会话 (分批处理)                         │
└─────────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┼───────────────┐
              ▼               ▼               ▼
         [会话 A]        [会话 B]        [会话 C]
              │               │               │
              ▼               ▼               ▼
┌─────────────────────────────────────────────────────────────────┐
│ 对每个会话:                                                      │
│ 1. 获取 maxSeq, currentMinSeq                                   │
│ 2. 计算 minSeqByTime (基于时间)                                  │
│ 3. 计算 minSeqByCount (基于数量限制)                             │
│ 4. newMinSeq = max(minSeqByTime, minSeqByCount)                │
│ 5. 应用最低保留保护: if newMinSeq > maxSeq - minRetainCount      │
│                      then newMinSeq = maxSeq - minRetainCount   │
│ 6. 更新 minSeq (仅当 newMinSeq > currentMinSeq)                 │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│            物理删除 minSeq 之前的消息文档 (可选)                   │
└─────────────────────────────────────────────────────────────────┘
```

---

## 3. 实现方案

### 3.1 需要修改的文件

| 文件 | 修改内容 |
|------|----------|
| `pkg/common/config/config.go` | 添加 `MaxMessagesPerConversation` 和 `MinRetainCount` 配置项 |
| `config/openim-crontask.yml` | 添加新配置的默认值 |
| `internal/tools/msg.go` | 修改 `deleteMsg()` 逻辑 |
| `pkg/common/storage/controller/msg.go` | 可能需要添加辅助方法 |

### 3.2 配置结构修改

```go
// pkg/common/config/config.go
type CronTask struct {
    CronExecuteTime            string   `mapstructure:"cronExecuteTime"`
    RetainChatRecords          int      `mapstructure:"retainChatRecords"`
    FileExpireTime             int      `mapstructure:"fileExpireTime"`
    DeleteObjectType           []string `mapstructure:"deleteObjectType"`
    // 新增
    MaxMessagesPerConversation int      `mapstructure:"maxMessagesPerConversation"`
    MinRetainCount             int      `mapstructure:"minRetainCount"`
}
```

### 3.3 核心实现思路

```go
// internal/tools/msg.go
func (c *cronServer) deleteMsg() {
    // ... 现有的基于时间的删除逻辑 ...

    // 新增：基于数量的清理
    if c.config.CronTask.MaxMessagesPerConversation > 0 {
        c.cleanupByMessageCount()
    }
}

func (c *cronServer) cleanupByMessageCount() {
    // 分批获取所有会话
    conversations := getAllConversations()

    for _, conv := range conversations {
        maxSeq := getMaxSeq(conv.ID)
        currentMinSeq := getMinSeq(conv.ID)

        // 计算新的 minSeq
        newMinSeq := c.calculateNewMinSeq(conv.ID, maxSeq, currentMinSeq)

        if newMinSeq > currentMinSeq {
            setMinSeq(conv.ID, newMinSeq)
        }
    }
}
```

---

## 4. minSeq 机制说明

### 4.1 为什么用 minSeq 而不是物理删除？

| 对比项 | minSeq 指针 | 物理删除 |
|--------|-------------|----------|
| 执行速度 | O(1) 单文档更新 | O(N) 删除多个文档 |
| 存储碎片 | 无 | 会产生碎片 |
| 可恢复性 | 可以（减小 minSeq） | 不可恢复 |
| 实现复杂度 | 简单 | 需要处理边界情况 |

### 4.2 minSeq 工作原理

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

### 4.3 后续物理删除（可选）

minSeq 之前的消息虽然不可访问，但仍占用存储空间。可以通过现有的 `DestructMsgs` 机制在后台异步删除：

```go
// 物理删除 minSeq 之前的消息文档
func (c *cronServer) physicalDeleteOldDocs() {
    // 现有逻辑已支持，通过 GetRandBeforeMsg 获取旧文档并删除
}
```

---

## 5. 配置建议

### 5.1 推荐配置（教育场景，25000 用户）

```yaml
cronTask:
  cronExecuteTime: "0 2 * * *"
  retainChatRecords: 365          # 保留一年
  maxMessagesPerConversation: 500 # 每会话最多 500 条
  minRetainCount: 200             # 至少保留 200 条
  fileExpireTime: 180             # 文件保留半年
```

### 5.2 配置策略说明

| 用户类型 | 消息量 | 效果 |
|----------|--------|------|
| 普通学生 | < 200 条/年 | 全部保留 |
| 活跃班级群 | 1000+ 条/年 | 保留最近 500 条 |
| 低活跃会话 | 50 条/年 | 全部保留 |

---

## 6. 总结

### 清理策略公式

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

### 优先级

```
1. 最低保留数量 (minRetainCount) - 最高优先级，保护用户体验
2. 时间限制 (retainChatRecords) - 中优先级
3. 数量限制 (maxMessagesPerConversation) - 与时间限制取较严格者
```

---

## 附录：相关代码位置

| 模块 | 文件路径 |
|------|----------|
| 配置定义 | `openim-server/pkg/common/config/config.go` |
| Cron 任务 | `openim-server/internal/tools/msg.go` |
| 清理 RPC | `openim-server/internal/rpc/msg/clear.go` |
| minSeq 存储 | `openim-server/pkg/common/storage/database/mgo/seq_conversation.go` |
| 消息控制器 | `openim-server/pkg/common/storage/controller/msg.go` |
| 配置文件 | `openim-server/config/openim-crontask.yml` |
