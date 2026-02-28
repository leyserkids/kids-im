# 定时清理任务实现细节

## 概述

`openim-crontask` 微服务负责定期清理过期数据，包含三个独立的定时任务：

| 任务 | 函数 | 清理对象 | 涉及存储 |
|------|------|---------|---------|
| 消息清理 | `deleteMsg()` | MongoDB `msg` collection 中的过期消息文档 | MongoDB |
| S3 文件清理 | `clearS3()` | 过期的上传文件及其元数据 | MongoDB + Redis + S3 |
| 用户消息清理 | `clearUserMsg()` | 用户设置了自动销毁的会话消息 | MongoDB |

三个任务按同一个 cron 表达式并发触发，互相独立。

### 配置

```yaml
# config/openim-crontask.yml
cronExecuteTime: 0 2 * * *       # 每天凌晨 2 点执行
retainChatRecords: 395            # 消息保留天数
fileExpireTime: 90                # 文件保留天数
deleteObjectType:                 # 要清理的 S3 对象类型
  - msg-picture
  - msg-file
  - msg-voice
  - msg-video
  - msg-video-snapshot
  - sdklog
```

所有配置项均可通过环境变量覆盖，格式为 `IMENV_OPENIM_CRONTASK_<KEY>`。

---

## 一、消息清理（deleteMsg）

### 1.1 调用链

```
tools/s3.go: clearS3()
  └─ tools/msg.go: deleteMsg()
       └─ rpc/msg/clear.go: DestructMsgs()
            └─ mgo/msg.go: GetRandBeforeMsg()   ← MongoDB 查询
            └─ mgo/msg.go: DeleteDoc()           ← 删除文档
            └─ 更新 seq collection 的 MinSeq
```

### 1.2 核心查询：GetRandBeforeMsg

**源码位置**：`pkg/common/storage/database/mgo/msg.go`

```go
func (m *MsgMgo) GetRandBeforeMsg(ctx context.Context, ts int64, limit int) ([]*model.MsgDocModel, error) {
    return mongoutil.Aggregate[*model.MsgDocModel](ctx, m.coll, []bson.M{
        {
            "$match": bson.M{
                "msgs": bson.M{
                    "$not": bson.M{
                        "$elemMatch": bson.M{
                            "msg.send_time": bson.M{"$gt": ts},
                        },
                    },
                },
            },
        },
        {"$project": bson.M{"_id": 0, "doc_id": 1, "msgs.msg.send_time": 1, "msgs.msg.seq": 1}},
        {"$sample": bson.M{"size": limit}},
    })
}
```

等价的 MongoDB 查询：

```js
db.msg.aggregate([
  // 第一步：匹配"文档内不存在任何 send_time > 截止时间的消息"
  { $match: { msgs: { $not: { $elemMatch: { "msg.send_time": { $gt: ts } } } } } },
  // 第二步：只取需要的字段
  { $project: { _id: 0, doc_id: 1, "msgs.msg.send_time": 1, "msgs.msg.seq": 1 } },
  // 第三步：随机采样
  { $sample: { size: 50 } }
])
```

### 1.3 删除条件的关键细节

**不是简单的"过期就删"，而是"整个文档内所有消息都过期才删"。**

每个 msg 文档最多存储 100 条消息（按 Seq 连续分配）。删除条件使用 `$not` + `$elemMatch` 组合，含义是：

> 文档中**不存在**任何 `send_time > 截止时间` 的消息 → 即文档中**所有消息**都已过期

这意味着：

```
文档 A (seq 1-100):
  ├─ msg[0].send_time = 2025-01-01  ← 过期
  ├─ msg[1].send_time = 2025-01-02  ← 过期
  └─ msg[99].send_time = 2025-03-01 ← 过期
  → 全部过期 ✅ 会被删除

文档 B (seq 101-200):
  ├─ msg[0].send_time = 2025-01-01  ← 过期
  ├─ msg[50].send_time = 2026-02-28 ← 未过期 !!!
  └─ msg[99] = null                 ← 空槽位
  → 存在未过期消息 ❌ 整个文档被跳过
```

**为什么文档内会混入新旧消息？** 因为文档的 Seq 范围是固定的（如 `0-99`, `100-199`），如果一个会话早期发了几条消息（占据了 `seq 0-5`），很久之后又发了新消息（占据了 `seq 6-10`），它们会落在同一个文档中，导致文档内新旧消息共存。

### 1.4 随机采样的影响

查询使用了 `$sample: { size: 50 }`，这是**随机采样**而非全量扫描。

循环退出条件：

```go
for i := 1; i <= 10000; i++ {
    resp, err := c.msgClient.DestructMsgs(ctx, &msg.DestructMsgsReq{
        Timestamp: deltime.UnixMilli(),
        Limit:     50,      // 每批随机取 50 个
    })
    if resp.Count < 50 {    // 取到的数量 < limit → 认为已清完
        break
    }
}
```

`$sample` 不保证能取到所有符合条件的文档。当返回数 < limit 时就停止循环，但这**不意味着已经全部清理干净**——只是随机采样没有取到更多了。因此：

- 单次 cron 执行可能无法删完所有过期文档
- 需要多天连续执行才能逐步清理干净
- 这是一种"尽力而为"的渐进式清理策略

### 1.5 删除后的操作

对每个被删除的文档，还会更新该会话的 `MinSeq`：

```go
// rpc/msg/clear.go
for _, doc := range docs {
    // 1. 物理删除 msg 文档
    m.MsgDatabase.DeleteDoc(ctx, doc.DocID)

    // 2. 从 docID 解析出 conversationID 和最大 seq
    conversationID, maxSeq := parseDocID(doc.DocID)

    // 3. 更新 seq collection，将 MinSeq 推进到已删文档之后
    m.MsgDatabase.SetMinSeq(ctx, conversationID, maxSeq + 1)
}
```

更新 MinSeq 后，客户端请求已删除 Seq 范围内的消息时，服务端会返回 `MsgStatusHasDeleted` 状态。

---

## 二、S3 文件清理（clearS3）

### 2.1 调用链

```
tools/s3.go: clearS3()
  └─ rpc/third/s3.go: DeleteOutdatedData()
       ├─ mgo/object.go: FindExpirationObject()   ← 查询过期对象
       └─ 对每个对象循环执行:
            ├─ ① DeleteSpecifiedData()              ← 删 MongoDB 记录
            ├─ ② DelS3Key()                         ← 删 Redis 缓存
            ├─ ③ GetKeyCount()                      ← 检查引用计数
            └─ ④ DeleteObject()                     ← 删 S3 实际文件（仅 count=0）
```

### 2.2 查询过期对象

**源码位置**：`pkg/common/storage/database/mgo/object.go`

```go
func (o *S3Mongo) FindExpirationObject(ctx context.Context, engine string, expiration time.Time,
    needDelType []string, count int64) ([]*model.Object, error) {
    return mongoutil.Find[*model.Object](ctx, o.coll, bson.M{
        "engine":      engine,
        "create_time": bson.M{"$lt": expiration},
        "group":       bson.M{"$in": needDelType},
    }, options.Find().SetLimit(count))
}
```

等价的 MongoDB 查询：

```js
db.s3.find({
  engine: "aws",
  create_time: { $lt: ISODate("2026-02-27T05:00:00Z") },
  group: { $in: ["msg-picture", "msg-file", "msg-voice", "msg-video", "msg-video-snapshot", "sdklog"] }
}).limit(100)
```

与 msg 不同，S3 查询使用的是 `find` + `limit`（顺序查询），不是 `$sample`（随机采样），所以**可以删干净**。

### 2.3 四步删除流程

**源码位置**：`internal/rpc/third/s3.go`

```go
for i, obj := range models {
    // ① 删除 MongoDB s3 collection 中的元数据记录
    t.s3dataBase.DeleteSpecifiedData(ctx, engine, []string{obj.Name})

    // ② 删除 Redis 中该对象的缓存 key
    t.s3dataBase.DelS3Key(ctx, engine, obj.Name)

    // ③ 查询是否还有其他记录引用同一个 S3 key
    count, _ := t.s3dataBase.GetKeyCount(ctx, engine, obj.Key)

    // ④ 只有当引用计数为 0 时，才删除 S3 上的实际文件
    if count == 0 {
        t.s3.DeleteObject(ctx, obj.Key)
    }
}
```

### 2.4 引用计数机制

OpenIM 的文件上传支持**去重**：相同 hash 的文件共享同一个 S3 存储 key，但在 `s3` collection 中有各自独立的 `name` 记录。

```
s3 collection:
┌──────────────────────────────────────────────────────────────┐
│ name: "user_A/msg_picture_abc.jpg"                           │
│ key:  "openim/data/hash/abc123"        ← 同一个 S3 key      │
│ create_time: 2025-12-01                ← 已过期              │
├──────────────────────────────────────────────────────────────┤
│ name: "user_B/msg_picture_def.jpg"                           │
│ key:  "openim/data/hash/abc123"        ← 同一个 S3 key      │
│ create_time: 2026-02-20                ← 未过期              │
└──────────────────────────────────────────────────────────────┘

删除 user_A 的记录后：
  GetKeyCount("openim/data/hash/abc123") → 1（user_B 还在引用）
  → 不删除 S3 实际文件
```

这确保了：即使一个元数据记录过期了，只要还有其他记录引用同一个文件，S3 上的实际文件就不会被误删。

### 2.5 日志中的 count 字段

日志中每条 `delete s3 object record` 的 `count` 值，就是删除当前记录后、同 key 剩余的引用数：

```
"count": 0   → S3 实际文件也会被删除
"count": 1   → 只删 MongoDB 记录和 Redis 缓存，保留 S3 文件
"count": 2   → 同上
```

---

## 三、用户消息清理（clearUserMsg）

### 3.1 调用链

```
tools/user_msg.go: clearUserMsg()
  └─ conversation-rpc: ClearUserConversationMsg()
       └─ 遍历设置了 MsgDestructTime 的会话
            └─ 为每个用户设置 MinSeq
```

### 3.2 触发条件

此任务处理的是**用户主动设置了消息自动销毁**的会话（如"阅后即焚"功能）。只有当会话的 `MsgDestructTime > 0` 时才生效。

如果没有任何会话设置了自动销毁，返回 `count: 0` 即完成。

---

## 四、三个任务的对比

| 维度 | deleteMsg | clearS3 | clearUserMsg |
|------|-----------|---------|-------------|
| **查询方式** | `$sample` 随机采样 | `find` 顺序查询 | 遍历会话列表 |
| **单次能否删干净** | 不一定 | 能 | 能 |
| **删除粒度** | 整个 msg 文档（~100条消息） | 单个文件记录 | 逻辑删除（更新 MinSeq） |
| **安全机制** | 文档内有新消息 → 跳过整个文档 | S3 key 有引用 → 保留实际文件 | 无特殊限制 |
| **每批数量** | 50 个文档 | 100 个对象 | 100 个会话 |
| **最大迭代次数** | 10,000 | 10,000 | 10,000 |
| **涉及存储** | MongoDB (msg + seq) | MongoDB + Redis + S3 | MongoDB (seq) |
| **退出条件** | 返回数 < 50 | 返回数 < 100 | 返回数 < 100 |

---

## 五、注意事项

### 5.1 msg 清理可能无法一次删干净

由于 `$sample` 随机采样和"整个文档必须全部过期"的双重限制，单次 cron 执行后仍可能残留旧数据。这是**符合设计预期**的行为，连续多天执行可以逐步清理。

可通过以下查询验证残留情况：

```js
// 查看"旧"文档中是否混有新消息
db.msg.aggregate([
  { $project: {
      doc_id: 1,
      first_msg_time: { $arrayElemAt: ["$msgs.msg.send_time", 0] },
      max_msg_time: { $max: "$msgs.msg.send_time" }
  }},
  { $match: { first_msg_time: { $gt: 0, $lt: <截止时间戳> } } },
  { $sort: { first_msg_time: 1 } }
])
```

如果 `max_msg_time` 大于截止时间，说明文档内混有新消息，未被清理是正确行为。

### 5.2 S3 删除后 MongoDB 空间不会立即释放

MongoDB 删除文档后，`storageSize` 不会立即缩小。WiredTiger 引擎会在后续写入时复用空间，或可手动执行 `db.s3.compact()` 回收。

### 5.3 环境变量覆盖

所有配置均可通过环境变量临时覆盖，便于临时调整清理策略：

```yaml
# docker-compose 示例
environment:
  - IMENV_OPENIM_CRONTASK_RETAINCHATRECORDS=2        # 只保留 2 天
  - IMENV_OPENIM_CRONTASK_FILEEXPIRETIME=1            # 文件 1 天过期
  - IMENV_OPENIM_CRONTASK_CRONEXECUTETIME=0 5 * * *   # 每天 05:00 执行
```

---

## 六、关键代码位置

| 模块 | 文件路径 |
|------|---------|
| cron 调度入口 | `internal/tools/cron_task.go` |
| 消息清理任务 | `internal/tools/msg.go` |
| S3 清理任务 | `internal/tools/s3.go` |
| 用户消息清理任务 | `internal/tools/user_msg.go` |
| 消息删除 RPC | `internal/rpc/msg/clear.go` |
| S3 删除 RPC | `internal/rpc/third/s3.go` |
| MongoDB msg 查询 | `pkg/common/storage/database/mgo/msg.go` → `GetRandBeforeMsg()` |
| MongoDB s3 查询 | `pkg/common/storage/database/mgo/object.go` → `FindExpirationObject()` |
| 配置文件 | `config/openim-crontask.yml` |
