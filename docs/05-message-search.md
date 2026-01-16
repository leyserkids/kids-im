# OpenIM 消息搜索设计分析

## 概述

OpenIM 提供**双通道消息搜索**能力：
- **服务端搜索**：通过 REST API 搜索 MongoDB 中的全量历史消息
- **客户端本地搜索**：通过 SDK 搜索 SQLite 中已同步的本地消息

---

## 一、服务端消息搜索

### 1.1 架构

```
HTTP API (/msg/search_msg)
       ↓
RPC Service (SearchMessage)
       ↓
Message Controller
       ↓
MongoDB 聚合管道查询
```

### 1.2 API 接口

```protobuf
message SearchMessageReq {
  string sendID = 1;           // 发送者ID
  string recvID = 2;           // 接收者ID（或群组ID）
  int32 contentType = 3;       // 消息类型
  string sendTime = 4;         // 发送日期（格式：YYYY-MM-DD）
  int32 sessionType = 5;       // 会话类型
  RequestPagination pagination = 6;
}
```

### 1.3 查询过滤器

```go
func SearchMessage(ctx context.Context, req *SearchMessageReq) {
    filter := bson.M{}

    // RecvID 条件：同时匹配单聊和群聊
    if req.RecvID != "" {
        filter["$or"] = bson.A{
            bson.M{"msgs.msg.recv_id": req.RecvID},
            bson.M{"msgs.msg.group_id": req.RecvID},
        }
    }

    // SendID 条件
    if req.SendID != "" {
        filter["msgs.msg.send_id"] = req.SendID
    }

    // ContentType 条件
    if req.ContentType != 0 {
        filter["msgs.msg.content_type"] = req.ContentType
    }

    // SendTime 条件：时间范围查询
    if req.SendTime != "" {
        sendTime, _ := time.Parse(time.DateOnly, req.SendTime)
        filter["$and"] = bson.A{
            bson.M{"msgs.msg.send_time": bson.M{"$gte": sendTime.UnixMilli()}},
            bson.M{"msgs.msg.send_time": bson.M{"$lt": sendTime.Add(24*time.Hour).UnixMilli()}},
        }
    }
}
```

### 1.4 聚合管道

```go
pipeline := bson.A{
    // 1. 游标定位（避免 skip）
    bson.M{"$match": bson.M{"_id": bson.M{"$gt": nextID}}},
    // 2. 文档排序
    bson.M{"$sort": bson.M{"_id": 1}},
    // 3. 粗过滤：只查询 si_ 和 sg_ 开头的 doc_id
    bson.M{"$match": bson.M{"doc_id": bson.M{"$regex": "^(sg_|si_)"}}},
    // 4. 精细过滤
    bson.M{"$match": filter},
    // 5. 限制文档数
    bson.M{"$limit": 50},
    // 6. 展开消息数组
    bson.M{"$unwind": "$msgs"},
    // 7. 再次过滤
    bson.M{"$match": filter},
}
```

### 1.5 防止全表扫描

| 机制 | 实现方式 |
|------|----------|
| 分片存储 | 每 100 条消息一个文档 |
| 游标分页 | `_id > nextID` |
| 双层过滤 | 先过滤文档，再过滤消息 |
| 限制批次 | 每次最多 50 个文档 |

---

## 二、客户端本地搜索

### 2.1 数据库设计

**消息表**（每个会话独立一张表）：

```sql
CREATE TABLE chat_logs_[conversationID] (
    client_msg_id CHAR(64) PRIMARY KEY,
    seq           INTEGER DEFAULT 0,
    send_time     INTEGER,
    content       VARCHAR(1000),
    content_type  SMALLINT,
    status        SMALLINT,
    send_id       CHAR(64),
    recv_id       CHAR(64)
);

CREATE INDEX index_seq ON chat_logs_xxx (seq);
CREATE INDEX index_send_time ON chat_logs_xxx (send_time);
```

### 2.2 搜索接口

```go
type MessageModel interface {
    // 按关键词搜索
    SearchMessageByKeyword(
        ctx context.Context,
        contentType []int,
        keywordList []string,
        keywordListMatchType int,   // 0=OR, 1=AND
        conversationID string,
        startTime, endTime int64,
        offset, count int,
    ) ([]*LocalChatLog, error)

    // 按内容类型搜索
    SearchMessageByContentType(
        ctx context.Context,
        contentType []int,
        conversationID string,
        startTime, endTime int64,
        offset, count int,
    ) ([]*LocalChatLog, error)
}
```

### 2.3 SQL 查询生成

```go
func SearchMessageByKeyword(...) ([]*LocalChatLog, error) {
    // 构建关键词子条件
    var subCondition string
    connectStr := " OR "
    if keywordListMatchType == 1 {
        connectStr = " AND "
    }

    for i, keyword := range keywordList {
        if i == 0 {
            subCondition += " AND ("
        }
        subCondition += fmt.Sprintf("content LIKE '%%%s%%'", keyword)
        if i < len(keywordList)-1 {
            subCondition += connectStr
        } else {
            subCondition += ")"
        }
    }

    condition := fmt.Sprintf(
        "send_time BETWEEN %d AND %d AND status <= %d AND content_type IN ?",
        startTime, endTime, constant.MsgStatusSendFailed,
    )
    condition += subCondition

    return db.Table(tableName).Where(condition, contentType).Order("send_time DESC").Find(&result)
}
```

**生成的 SQL 示例**：

```sql
-- 关键词 OR 搜索
SELECT * FROM chat_logs_si_user1_user2
WHERE send_time BETWEEN 1703001600000 AND 1703088000000
  AND status <= 3
  AND content_type IN (101, 102, 103)
  AND (content LIKE '%hello%' OR content LIKE '%world%')
ORDER BY send_time DESC
LIMIT 20 OFFSET 0;
```

### 2.4 多会话并发搜索

```go
func SearchAllConversations(ctx context.Context, ...) []*LocalChatLog {
    conversationIDList := db.GetAllConversationIDList(ctx)

    g, _ := errgroup.WithContext(ctx)
    g.SetLimit(searchMessageGoroutineLimit)

    var mu sync.Mutex
    var allResults []*LocalChatLog

    for _, conversationID := range conversationIDList {
        convID := conversationID
        g.Go(func() error {
            results := db.SearchMessageByKeyword(ctx, convID, ...)
            mu.Lock()
            allResults = append(allResults, results...)
            mu.Unlock()
            return nil
        })
    }

    g.Wait()
    return allResults
}
```

### 2.5 消息内容解析

```go
func SearchLocalMessages(ctx context.Context, ...) {
    for _, msg := range list {
        switch msg.ContentType {
        case constant.Text:
            var text TextElem
            json.Unmarshal([]byte(msg.Content), &text)
            if matchKeywords(text.Content, keywordList) {
                results = append(results, msg)
            }

        case constant.File:
            var file FileElem
            json.Unmarshal([]byte(msg.Content), &file)
            if matchKeywords(file.FileName, keywordList) {
                results = append(results, msg)
            }

        case constant.Merger:
            var merger MergerElem
            json.Unmarshal([]byte(msg.Content), &merger)
            if searchInMergerMessage(merger, keywordList) {
                results = append(results, msg)
            }
        }
    }
}
```

---

## 三、本地搜索 vs 服务端搜索

| 特性 | 本地搜索 | 服务端搜索 |
|------|----------|------------|
| 延迟 | 毫秒级 | 百毫秒级 |
| 离线可用 | ✅ | ❌ |
| 数据范围 | 已同步的消息 | 全部历史消息 |
| 服务器压力 | 无 | 有 |
| 搜索方式 | LIKE 模糊匹配 | 可集成 Elasticsearch |

---

## 四、混合搜索策略

```
用户发起搜索
      ↓
1. 优先本地搜索（已同步消息）
   - 响应快，用户体验好
   - 适合最近消息搜索
      ↓
2. 判断是否需要服务端搜索
   - 时间范围超出本地同步范围
   - 本地结果不足
   - 用户明确要求搜索历史
      ↓
3. 调用服务端搜索（补充结果）
   - 合并去重
   - 按时间排序
```

---

## 五、性能优化建议

### 本地搜索优化

```sql
-- 添加全文搜索索引（FTS）
CREATE VIRTUAL TABLE chat_logs_fts USING fts5(content);

-- 添加复合索引
CREATE INDEX idx_time_type ON chat_logs (send_time, content_type);
```

### 服务端搜索优化

```go
// 添加 MongoDB 复合索引
{Keys: bson.D{
    {Key: "doc_id", Value: 1},
    {Key: "msgs.msg.send_id", Value: 1},
    {Key: "msgs.msg.send_time", Value: 1},
}}

// 考虑引入 Elasticsearch 做全文搜索
// 热点查询 Redis 缓存
```

---

## 六、关键代码位置

### 服务端

| 文件路径 | 功能 |
|----------|------|
| `internal/api/msg.go` | HTTP API 入口 |
| `internal/rpc/msg/sync_msg.go` | RPC 服务实现 |
| `pkg/common/storage/database/mgo/msg.go` | MongoDB 实现 |

### 客户端 SDK

| 文件路径 | 功能 |
|----------|------|
| `pkg/db/chat_log_model.go` | 消息搜索 |
| `internal/conversation_msg/conversation.go` | 搜索实现 |
| `internal/interaction/msg_sync.go` | 消息同步 |

---

## 七、总结

OpenIM 消息搜索采用**双通道设计**：

1. **服务端搜索**：分片存储 + 游标分页 + 双层过滤，适合全量历史消息搜索
2. **客户端本地搜索**：SQLite + 会话分表，适合实时搜索和离线场景
3. **最佳实践**：优先本地搜索，服务端搜索作为补充
