# OpenIM 数据库设计分析

## 概述

本文档分析 OpenIM 的数据库设计，包括服务端 MongoDB 和客户端本地 SQLite 数据库。通过深入分析其设计模式，理解 IM 系统数据存储面临的核心挑��及解决方案。

---

## 一、服务端 MongoDB 设计

### 1.1 核心 Collections

```
msg                    # 消息存储（核心）
seq                    # 会话序列号
seq_user               # 用户序列号
conversation           # 会话信息
conversation_version   # 会话版本（增量同步）
group                  # 群组信息
group_member           # 群组成员
group_member_version   # 群组成员版本
user                   # 用户信息
friend                 # 好友关系
black                  # 黑名单
```

### 1.2 消息存储设计（核心）

#### 数据结构

```go
// 消息文档模型 - 按会话分组存储
type MsgDocModel struct {
    DocID string          `bson:"doc_id"`    // 文档ID: conversationID:index
    Msg   []*MsgInfoModel `bson:"msgs"`      // 消息数组，固定100条
}

// 消息信息
type MsgInfoModel struct {
    Msg     *MsgDataModel `bson:"msg"`       // 消息内容
    Revoke  *RevokeModel  `bson:"revoke"`    // 撤回信息
    DelList []string      `bson:"del_list"`  // 删除用户列表
    IsRead  bool          `bson:"is_read"`   // 是否已读
}

// 消息数据
type MsgDataModel struct {
    SendID           string `bson:"send_id"`
    RecvID           string `bson:"recv_id"`
    GroupID          string `bson:"group_id"`
    ClientMsgID      string `bson:"client_msg_id"`
    ServerMsgID      string `bson:"server_msg_id"`
    SessionType      int32  `bson:"session_type"`
    ContentType      int32  `bson:"content_type"`
    Content          string `bson:"content"`
    Seq              int64  `bson:"seq"`           // 序列号
    SendTime         int64  `bson:"send_time"`
    CreateTime       int64  `bson:"create_time"`
    Status           int32  `bson:"status"`
}
```

#### 存储策略：按会话分表 + 按 Seq 分片

**关键设计点：**

1. **每个会话独立存储**：通过 `conversationID` 区分不同会话
2. **每 100 条消息一个文档**：`singleGocMsgNum = 100`
3. **DocID 格式**：`conversationID:index`
   - 例如：`si_user1_user2:0` (第 1-100 条消息)
   - 例如：`si_user1_user2:1` (第 101-200 条消息)

**分片算法：**

```go
// 根据 seq 计算文档索引
func GetDocIndex(seq int64) int64 {
    return (seq - 1) / 100  // seq=1-100 -> index=0
}

// 根据 seq 计算在文档中的位置
func GetMsgIndex(seq int64) int64 {
    return (seq - 1) % 100  // seq=1 -> index=0
}

// 生成 DocID
func GetDocID(conversationID string, seq int64) string {
    seqSuffix := (seq - 1) / 100
    return conversationID + ":" + strconv.FormatInt(seqSuffix, 10)
}
```

### 1.3 ConversationID 生成规则

```
si_user1_user2   # 单聊（按字母排序）
g_groupID        # 普通群聊
sg_groupID       # 超级群聊
n_user1_user2    # 通知消息
sn_xxx           # 系统通知
```

### 1.4 Seq 序列号设计

```go
type SeqConversation struct {
    ConversationID string `bson:"conversation_id"`
    MaxSeq         int64  `bson:"max_seq"`  // 最大序列号
    MinSeq         int64  `bson:"min_seq"`  // 最小序列号
}

// 原子递增分配 seq
func Malloc(ctx context.Context, conversationID string, size int64) (int64, error) {
    update := map[string]any{
        "$inc": map[string]any{"max_seq": size},  // 原子递增
    }
    // 返回递增前的值作为起始 seq
}
```

**特点：**
- 每个会话独立的 seq 序列
- 使用 MongoDB `$inc` 原子操作保证并发安全
- 支持批量分配（一次分配多个 seq）

### 1.5 索引设计

| Collection | 索引 | 类型 |
|------------|------|------|
| msg | `{doc_id: 1}` | 唯一索引 |
| seq | `{conversation_id: 1}` | 普通索引 |
| conversation | `{owner_user_id: 1, conversation_id: 1}` | 复合唯一索引 |
| group | `{group_id: 1}` | 唯一索引 |
| user | `{user_id: 1}` | 唯一索引 |

---

## 二、客户端本地数据库设计

### 2.1 技术栈

- **数据库**：SQLite
- **ORM**：GORM
- **文件命名**：`OpenIM_{版本号}_{用户ID}.db`
- **连接池**：MaxOpenConns=3, MaxIdleConns=2

### 2.2 核心表结构

#### 消息表 (LocalChatLog) - 动态表

**每个会话创建独立的消息表**，表名格式：`chat_log_{conversationID}`

```go
type LocalChatLog struct {
    ClientMsgID      string  // 主键，客户端消息ID
    ServerMsgID      string  // 服务端消息ID
    SendID           string  // 发送者ID
    RecvID           string  // 接收者ID (索引)
    SenderPlatformID int32
    SenderNickname   string
    SenderFaceURL    string
    SessionType      int32   // 会话类型
    ContentType      int32   // 内容类型 (索引)
    Content          string  // 消息内容 (最大1000字符)
    IsRead           bool
    Status           int32
    Seq              int64   // 序列号 (索引)
    SendTime         int64   // 发送时间 (索引)
    CreateTime       int64
    AttachedInfo     string
    Ex               string  // 扩展字段
    LocalEx          string  // 本地扩展
}
```

**索引设计：**
- `PRIMARY KEY (client_msg_id)`
- `index_seq_{conversationID}` on `seq`
- `index_send_time_{conversationID}` on `send_time`
- `index_recv_id` on `recv_id`
- `content_type_alone` on `content_type`

#### 会话表 (LocalConversation)

```go
type LocalConversation struct {
    ConversationID        string  // 主键
    ConversationType      int32
    UserID                string
    GroupID               string
    ShowName              string
    FaceURL               string
    RecvMsgOpt            int32   // 接收消息选项
    UnreadCount           int32   // 未读数
    GroupAtType           int32   // @类型
    LatestMsg             string  // 最新消息
    LatestMsgSendTime     int64   // 最新消息时间 (索引)
    DraftText             string  // 草稿
    IsPinned              bool    // 是否置顶
    MaxSeq                int64   // 最大序列号
    MinSeq                int64   // 最小序列号
}
```

**会话排序规则：**
```sql
ORDER BY
  CASE WHEN is_pinned=1 THEN 0 ELSE 1 END,
  MAX(latest_msg_send_time, draft_text_time) DESC
```

### 2.3 同步相关表

#### 版本同步表 (LocalVersionSync)

```go
type LocalVersionSync struct {
    Table      string      // 主键1，表名
    EntityID   string      // 主键2，实体ID
    VersionID  string      // 版本ID
    Version    uint64      // 版本号
    CreateTime int64
    UIDList    StringArray // ID列表 (JSON数组)
}
```

#### 已读状态表

```go
// 读取游标表
type LocalReadCursor struct {
    ConversationID string  // 主键1
    UserID         string  // 主键2
    MaxReadSeq     int64   // 最大已读序列号
}

// 读取状态表 - 存储"全员已读"位置
type LocalReadState struct {
    ConversationID string  // 主键
    AllReadSeq     int64   // 所有成员都已读的序列号
}
```

---

## 三、核心问题与解决方案

### 3.1 消息存储

| 问题 | 解决方案 |
|------|----------|
| 海量消息存储 | 按会话分表 + 按 Seq 分片（100条/文档） |
| 消息顺序一致性 | Seq 序列号机制（MongoDB $inc 原子操作） |
| 消息去重 | ClientMsgID 唯一标识 |

### 3.2 消息同步

| 问题 | 解决方案 |
|------|----------|
| 离线消息同步 | 基于 Seq 的增量同步 |
| 多端同步 | 版本号 + 增量同步 |

### 3.3 已读状态

**群聊已读状态设计（双表）**：

```
LocalReadCursor (详细记录)
├── conversation_id: group_123
├── user_id: user_A, max_read_seq: 150
├── user_id: user_B, max_read_seq: 120
└── user_id: user_C, max_read_seq: 180

LocalReadState (缓存计算结果)
└── conversation_id: group_123, all_read_seq: 120
```

---

## 四、设计模式总结

### 服务端设计模式

| 模式 | 描述 | 解决的问题 |
|------|------|-----------|
| 按会话分表 | 每个会话独立存储 | 避免单表过大 |
| 固定大小分片 | 每 100 条消息一个文档 | 便于管理和查询 |
| Seq 原子递增 | MongoDB $inc 操作 | 消息顺序一致性 |
| 软删除 | del_list 记录删除用户 | 支持多端删除状态 |
| 版本控制 | version 字段 | 增量同步 |

### 客户端设计模式

| 模式 | 描述 | 解决的问题 |
|------|------|-----------|
| 按会话分表 | 每个会话独立消息表 | 数据隔离、便于清理 |
| 双表已读 | ReadCursor + ReadState | O(1) 全员已读查询 |
| 版本同步 | LocalVersionSync | 增量同步追踪 |

---

## 五、关键代码位置

### 服��端 (openim-server)

```
pkg/common/storage/model/
├── msg.go              # 消息模型定义
├── seq.go              # 序列号模型
├── conversation.go     # 会话模型
└── user.go             # 用户模型

pkg/common/storage/database/mgo/
├── msg.go              # 消息 MongoDB 操作
├── seq_conversation.go # Seq 分配逻辑
└── conversation.go     # 会话操作

pkg/msgprocessor/
└── conversation.go     # ConversationID 生成规则
```

### 客户端 SDK (openim-sdk-core)

```
pkg/db/
├── db_init.go              # 数据库初始化
├── chat_log_model.go       # 消息表操作
├── conversation_model.go   # 会话表操作
└── version_sync.go         # 版本同步

pkg/db/model_struct/
├── local_chat_logs.go      # 消息表结构
└── data_model_struct.go    # 其他表结构
```
