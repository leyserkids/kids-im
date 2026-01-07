# 消息已读设计

## 概述

OpenIM 实现类似 Microsoft Teams 的消息已读状态显示：
- **无图标**：消息已发送但未被（所有人）已读
- **蓝色眼睛 👁️**：消息已被（所有人）已读

核心设计原则：
1. **单聊/群聊统一处理**：使用相同的表结构、同步逻辑和事件
2. **使用 seq 比较**：O(1) 时间复杂度判断已读状态
3. **事件驱动**：使用 `OnConversationReadStateChanged` 事件通知前端更新 UI

---

## 一、数据库设计

### 1.1 LocalReadCursor - 存储每个成员的已读位置

```sql
CREATE TABLE local_read_cursor (
    conversation_id CHAR(128),
    user_id CHAR(64),
    max_read_seq INTEGER,
    PRIMARY KEY (conversation_id, user_id)
);
```

| 字段 | 说明 |
|------|------|
| conversation_id | 会话 ID |
| user_id | 用户 ID |
| max_read_seq | 该用户已读的最大消息序号 |

**使用方式**：
- **单聊**：存储对方的已读位置
- **群聊**：存储每个群成员的已读位置

### 1.2 LocalReadState - 存储会话的全局已读状态

```sql
CREATE TABLE local_read_state (
    conversation_id CHAR(128),
    all_read_seq INTEGER DEFAULT 0,
    PRIMARY KEY (conversation_id)
);
```

| 字段 | 说明 |
|------|------|
| conversation_id | 会话 ID |
| all_read_seq | **其他人**都已读到的位置（cursor 中排除自己后的最小值） |

**语义**：
- **单聊**：`all_read_seq` = 对方已读到的位置
- **群聊**：`all_read_seq` = 所有**其他**群成员中读得最少的位置

---

## 二、通知类型

### 2.1 MarkAsReadTips (contentType: 2200)

**发送场景**：用户标记消息为已读时

**通知目标**：
- **单聊**：发送给对方（告诉对方"我读了你的消息"）
- **群聊**：发送给自己（同步自己在其他设备的已读状态）

### 2.2 GroupHasReadTips (contentType: 2201)

**发送场景**：群成员标记消息为已读时

**通知目标**：发送给**所有其他**群成员（不包括已读操作的发起者）

**通知内容**：
```go
type GroupHasReadTips struct {
    GroupID        string
    ConversationID string
    UserID         string  // 谁读了消息
    HasReadSeq     int64   // 读到了哪个位置
}
```

---

## 三、allReadSeq 计算逻辑

**关键点**：计算 `allReadSeq` 时必须**排除自己**

```go
// 计算时排除当前登录用户
allSeq, err := c.db.GetAllReadSeqFromCursors(ctx, conversationID, c.loginUserID)
```

```sql
-- SQL 实现
SELECT MIN(max_read_seq) FROM local_read_cursor
WHERE conversation_id = ? AND user_id != ?  -- 排除自己
```

**原因**：`allReadSeq` 表示"其他人都读到了哪里"，自己发的消息自己肯定看过。

---

## 四、同步策略

### 4.1 同步时机

| 时机 | 触发点 | 同步范围 |
|------|--------|----------|
| 连接成功/重连 | `MsgSyncEnd` | 所有会话 |
| 应用唤醒 | `syncData` | 所有会话 |
| 成员退出群聊 | `MemberQuitNotification` (1504) | 当前群 |
| 成员被踢出 | `MemberKickedNotification` (1508) | 当前群 |
| 新成员加入 | `MemberEnterNotification` (1510) | 当前群 |
| 新成员被邀请 | `MemberInvitedNotification` (1509) | 当前群 |
| 群解散 | `GroupDismissedNotification` (1511) | 当前群 |

### 4.2 连接成功后全量同步

```go
func (c *Conversation) syncAllReadCursors(ctx context.Context) {
    allConversations, _ := c.db.GetAllConversations(ctx)

    var conversationIDs []string
    for _, conv := range allConversations {
        if conv.ConversationType == constant.SingleChatType ||
           conv.ConversationType == constant.ReadGroupChatType {
            conversationIDs = append(conversationIDs, conv.ConversationID)
        }
    }

    c.SyncReadCursors(ctx, conversationIDs)
    c.notifySubscribedConversationsReadStateChanged(ctx)
}
```

### 4.3 成员变动处理

**成员退出/被踢**：删除 cursor，重算 allReadSeq（可能增加）

```go
func (c *Conversation) handleMemberLeftForReadCursor(ctx context.Context, conversationID string, userIDs []string) {
    for _, userID := range userIDs {
        c.db.DeleteReadCursor(ctx, conversationID, userID)
    }
    c.updateReadStateAfterSync(ctx, conversationID)
}
```

**新成员加入**：从服务器同步获取真实阅读位置

```go
func (c *Conversation) handleMemberEnterForReadCursorInternal(ctx context.Context, conversationID string) {
    // 从服务器同步获取真实的阅读位置，而不是创建 maxReadSeq=0 的 cursor
    c.SyncReadCursors(ctx, []string{conversationID})
}
```

---

## 五、前端接口设计

### 5.1 订阅会话已读状态

```go
// Go SDK
func (c *Conversation) SubscribeConversationReadState(ctx context.Context, conversationID string) (allReadSeq int64, err error)

// JS SDK
subscribeConversationReadState(conversationID: string): Promise<number>
```

**行为**：
1. 将 conversationID 加入订阅集合
2. 返回本地数据库中的当前 allReadSeq（无数据时返回 0）
3. 后续变化通过 `OnConversationReadStateChanged` 回调通知

### 5.2 取消订阅

```go
// Go SDK
func (c *Conversation) UnsubscribeConversationReadState(ctx context.Context, conversationID string) error

// JS SDK
unsubscribeConversationReadState(conversationID: string): Promise<void>
```

### 5.3 已读状态变化回调

```json
// OnConversationReadStateChanged
{
  "conversationID": "sg_xxx",
  "allReadSeq": 123
}
```

**触发条件**：
- 收到其他成员的已读回执
- 首次进入会话同步完成后
- 重连后同步完成

---

## 六、工作流程

### 6.1 单聊已读回执流程

```
1. 用户B 打开会话，阅读消息
   ↓
2. 客户端调用 markConversationMessageAsRead
   ↓
3. 服务端发送 MarkAsReadTips (2200) 给用户A
   ↓
4. 用户A 的 SDK 收到通知
   ↓
5. 更新 cursor 和 state
   ↓
6. 如果 allReadSeq 变化，触发 OnConversationReadStateChanged
   ↓
7. 前端更新 UI（显示蓝色眼睛 👁️）
```

### 6.2 群聊已读回执流程

```
1. 用户B 打开群会话，阅读消息
   ↓
2. 客户端调用 markConversationMessageAsRead
   ↓
3. 服务端：
   - 发送 MarkAsReadTips (2200) 给用户B自己（多端同步）
   - 发送 GroupHasReadTips (2201) 给所有其他成员
   ↓
4. 其他成员的 SDK 收到 GroupHasReadTips
   ↓
5. 更新 cursor 和 state
   ↓
6. 如果 allReadSeq 变化，触发 OnConversationReadStateChanged
   ↓
7. 前端更新 UI（显示蓝色眼睛 👁️）
```

### 6.3 前端判断逻辑

```typescript
// 只显示自己发出的消息的已读状态
const isSender = currentUserID === message.sendID;
const isRead = allReadSeq > 0 && message.seq > 0 && message.seq <= allReadSeq;

if (isSender && isRead) {
    // 显示蓝色眼睛 👁️
}
```

---

## 七、关键代码位置

### SDK (openim-sdk-core)

| 文件 | 说明 |
|------|------|
| `pkg/db/model_struct/data_model_struct.go` | 表结构定义 |
| `pkg/db/read_cursor_model.go` | 数据库操作 |
| `internal/conversation_msg/read_drawing.go` | 已读回执处理 |
| `internal/conversation_msg/sync.go` | 同步逻辑 |
| `sdk_callback/callback.go` | 回调接口定义 |

### 服务端 (openim-server)

| 文件 | 说明 |
|------|------|
| `internal/rpc/msg/as_read.go` | 已读标记请求处理 |
| `internal/rpc/conversation/conversation.go` | GetConversationReadCursors API |

### 协议 (openim-protocol)

| 文件 | 说明 |
|------|------|
| `sdkws/sdkws.proto` | GroupHasReadTips 消息定义 |
| `constant/constant.go` | GroupHasReadReceipt (2201) 常量 |

---

## 八、注意事项

1. **计算 allReadSeq 时排除自己**：必须传入当前登录用户 ID
2. **新成员影响**：新成员加入会使 allReadSeq 可能归零
3. **事件触发条件**：只有 allReadSeq 变化时才触发回调
4. **清理策略**：退出群/删除会话时需删除相关 cursor 和 state
5. **群聊通知分离**：MarkAsReadTips 只发给自己，GroupHasReadTips 广播给他人
