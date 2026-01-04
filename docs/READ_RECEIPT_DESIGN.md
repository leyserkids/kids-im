# 已读回执方案设计

## 概述

本方案实现类似 Microsoft Teams 的消息已读状态显示：
- **无图标**：消息已发送但未被（所有人）已读
- **蓝色眼睛 👁️**：消息已被（所有人）已读
- **感叹号 ⚠️**：消息发送失败（由 `MessageSendStatus` 组件显示）

> 注：早期方案曾考虑使用蓝色对勾表示"已发送未读"，但实际使用中发现没有必要——发送成功是默认状态，只需关注"是否已读"和"是否失败"即可。

## 核心原则

1. **只显示自己发出的消息的已读状态** - 别人发的消息不显示已读图标
2. **单聊/群聊统一处理** - 使用相同的表结构、同步逻辑和事件
3. **使用 seq 比较** - O(1) 时间复杂度判断已读状态
4. **事件驱动** - 使用 `OnConversationReadStateChanged` 事件通知前端更新 UI

---

## 数据库表设计

### 1. LocalReadCursor - 存储每个成员的已读位置

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
| conversation_id | 会话ID |
| user_id | 用户ID |
| max_read_seq | 该用户已读的最大消息序号 |

**使用方式**：
- **单聊**：存储对方的已读位置
- **群聊**：存储每个群成员的已读位置（包括自己，但计算 allReadSeq 时排除自己）

### 2. LocalReadState - 存储会话的全局已读状态

```sql
CREATE TABLE local_read_state (
    conversation_id CHAR(128),
    all_read_seq INTEGER DEFAULT 0,
    PRIMARY KEY (conversation_id)
);
```

| 字段 | 说明 |
|------|------|
| conversation_id | 会话ID |
| all_read_seq | **其他人**都已读到的位置（cursor 中排除自己后的最小值） |

**语义**：
- **单聊**：`all_read_seq` = 对方已读到的位置
- **群聊**：`all_read_seq` = 所有**其他**群成员中读得最少的位置

**用途**：快速判断某条消息是否被所有人已读，无需遍历所有 cursor。

### 存储对比

| 会话类型 | local_read_cursor | local_read_state |
|---------|-------------------|------------------|
| 单聊 | 1-2 条记录 | 1 条记录 |
| 群聊 | N 条记录（N = 群成员数） | 1 条记录 |

---

## 通知类型

### MarkAsReadTips (contentType: 2200)

由 `sendMarkAsReadNotification` 发送，用于通知已读状态变化。

**发送场景**：
- `SetConversationHasReadSeq`：用户设置已读位置
- `MarkMsgsAsRead`：用户标记特定消息为已读
- `MarkConversationAsRead`：用户标记整个会话为已读

**通知目标**：
- **单聊**：发送给对方（告诉对方"我读了你的消息"）
- **群聊**：发送给自己（同步自己在其他设备的已读状态）

### GroupHasReadTips (contentType: 2201)

由 `broadcastGroupHasReadReceipt` 发送，用于广播群成员的已读状态。

**发送场景**：
- `MarkConversationAsRead` 且会话类型为 `ReadGroupChatType`

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

## 工作流程

### 单聊已读回执流程

```
1. 用户B 打开会话，阅读消息
   ↓
2. 客户端调用 markConversationMessageAsRead
   ↓
3. 服务端发送 MarkAsReadTips (2200) 给用户A
   ↓
4. 用户A 的 SDK 收到通知，调用 doReadDrawing()
   ↓
5. 更新本地消息的已读状态，触发 OnRecvC2CReadReceipt
   ↓
6. 调用 updateReadCursorAndReadState() 更新 cursor 和 state
   ↓
7. 如果 allReadSeq 变化，触发 OnConversationReadStateChanged
   ↓
8. 前端更新 UI（消息显示蓝色眼睛 👁️ 图标）
```

### 群聊已读回执流程

```
1. 用户B 打开群会话，阅读消息
   ↓
2. 客户端调用 markConversationMessageAsRead
   ↓
3. 服务端：
   - 发送 MarkAsReadTips (2200) 给用户B自己（多端同步）
   - 调用 broadcastGroupHasReadReceipt 发送 GroupHasReadTips (2201) 给所有其他成员
   ↓
4. 其他成员的 SDK 收到 GroupHasReadTips，调用 doGroupReadDrawing()
   ↓
5. 调用 updateReadCursorAndReadState() 更新 cursor 和 state
   ↓
6. 如果 allReadSeq 变化，触发 OnConversationReadStateChanged
   ↓
7. 前端更新 UI（消息显示蓝色眼睛 👁️ 图标）
```

### allReadSeq 计算逻辑

**关键点**：计算 `allReadSeq` 时必须**排除自己**，因为：
- `allReadSeq` 表示"其他人都读到了哪里"
- 自己的已读位置对自己没有意义（自己发的消息自己当然看过）

```go
// 计算时排除当前登录用户
allSeq, err := c.db.GetAllReadSeqFromCursors(ctx, conversationID, c.loginUserID)
```

```sql
-- SQL 实现
SELECT MIN(max_read_seq) FROM local_read_cursor
WHERE conversation_id = ? AND user_id != ?  -- 排除自己
```

### updateReadCursorAndReadState 函数

```go
func (c *Conversation) updateReadCursorAndReadState(
    ctx context.Context,
    conversationID, userID string,
    maxReadSeq int64,
) (allReadSeqChanged bool, newAllReadSeq int64) {
    // 1. 检查是否需要更新
    oldCursor, err := c.db.GetReadCursor(ctx, conversationID, userID)
    if err == nil && maxReadSeq <= oldCursor.MaxReadSeq {
        return false, 0  // 无变化
    }

    // 2. 更新 cursor
    c.db.UpsertReadCursor(ctx, &LocalReadCursor{
        ConversationID: conversationID,
        UserID:         userID,
        MaxReadSeq:     maxReadSeq,
    })

    // 3. 重新计算 allReadSeq（排除自己）
    allSeq, err := c.db.GetAllReadSeqFromCursors(ctx, conversationID, c.loginUserID)

    // 4. 检查是否变化
    state, _ := c.db.GetReadState(ctx, conversationID)
    oldAllReadSeq := 0
    if state != nil {
        oldAllReadSeq = state.AllReadSeq
    }

    if allSeq != oldAllReadSeq {
        // 5. 更新 state
        c.db.UpsertReadState(ctx, &LocalReadState{
            ConversationID: conversationID,
            AllReadSeq:     allSeq,
        })
        return true, allSeq
    }
    return false, 0
}
```

---

## 前端判断逻辑

```typescript
// message-read-status.tsx
export const MessageReadStatus = ({ message, allReadSeq }: MessageReadStatusProps) => {
    const currentUserID = useCurrentUserID();
    const isSender = currentUserID === message.sendID;

    // 只显示自己发出的消息的已读状态
    const messageStatusIsSucc = message.status === MessageStatus.Succeed;
    const contentTypeIsGroupAnnouncementUpdated = message.contentType === MessageType.GroupAnnouncementUpdated;
    const contentTypeIsCustomMessage = message.contentType === MessageType.CustomMessage;
    const showMessageStatus =
        isSender && messageStatusIsSucc && !contentTypeIsGroupAnnouncementUpdated && !contentTypeIsCustomMessage;

    if (!showMessageStatus) {
        return null;
    }

    // 单聊/群聊统一使用 allReadSeq
    // allReadSeq 为 0 表示还没获取到真实值，不显示
    const isRead = allReadSeq > 0 && message.seq > 0 && message.seq <= allReadSeq;

    // 只在已读时显示蓝色眼睛图标，未读时不显示任何图标
    if (!isRead) {
        return null;
    }

    return <EyeOutlined className="text-[#1890ff]" />;
};
```

**判断条件说明**：
1. `isSender` - 只显示自己发出的消息
2. `messageStatusIsSucc` - 只显示发送成功的消息（失败由 `MessageSendStatus` 显示感叹号）
3. `allReadSeq > 0` - 确保已获取到真实的已读位置
4. `message.seq > 0` - 确保消息已被服务端分配 seq（见 OnMessageSeqUpdated 事件）
5. `message.seq <= allReadSeq` - 消息 seq 小于等于 allReadSeq 表示已被所有人已读

---

## 事件定义

### OnConversationReadStateChanged

当会话的 `allReadSeq` 发生变化时触发。

```typescript
interface ReadStateChangedEvent {
    conversationID: string;
    allReadSeq: number;    // 所有其他人都已读到的位置
}
```

**触发条件**：
- 收到其他成员的已读回执（MarkAsReadTips 或 GroupHasReadTips）
- 首次进入会话同步 cursor 完成后
- 重连后同步 cursor 完成

### OnRecvC2CReadReceipt

单聊收到已读回执时触发（保留以支持现有前端逻辑）。

```typescript
interface MessageReceipt {
    userID: string;
    msgIDList: string[];
    sessionType: number;
    readTime: number;
}
```

### OnMessageSeqUpdated

当消息被服务端分配 seq 时触发。

```typescript
interface MessageSeqUpdatedEvent {
    clientMsgID: string;
    seq: number;
    // ... 其他消息字段
}
```

**重要性**：
- 消息发送时，`seq` 初始值为 0
- 服务端处理后分配真正的 `seq`
- 前端需要监听此事件更新本地消息的 `seq`
- 只有 `seq > 0` 的消息才能正确判断已读状态

**前端处理**：
```typescript
// use-global-events.ts
const messageSeqUpdatedHandler = useCallback(({ data }: WSEvent<MessageItem>) => {
    logInfo('[MessageSeqUpdated] clientMsgID:', data.clientMsgID, 'seq:', data.seq);
    updateOneMessage(data);
}, []);

useImEventListener(CbEvents.OnMessageSeqUpdated, messageSeqUpdatedHandler);
```

**防止 seq 被覆盖**：
```typescript
// use-history-message-list.ts
// 如果新消息的 seq 是 0，保留原来的 seq（避免被旧数据覆盖）
const newMsg = { ...tmpList[idx], ...message };
if (message.seq === 0 && tmpList[idx].seq > 0) {
    newMsg.seq = tmpList[idx].seq;
}
tmpList[idx] = newMsg;
```

### 按需查询接口

如果前端需要显示"谁读了这条消息"（已读列表），可以调用：

```typescript
// 获取某个会话的所有已读 cursor
function getReadCursors(conversationID: string): Promise<ReadCursor[]>;

interface ReadCursor {
    conversationID: string;
    userID: string;
    maxReadSeq: number;
}
```

---

## ReadCursor 同步策略

详细的同步策略设计请参考：[READ_CURSOR_SYNC_ANALYSIS.md](./READ_CURSOR_SYNC_ANALYSIS.md)

### 概述

- **同步时机**：连接成功/重连时全量同步所有会话的 ReadCursor
- **实时更新**：收到 MarkAsReadTips (2200) / GroupHasReadTips (2201) 时更新本地数据
- **订阅机制**：前端通过 `SubscribeConversationReadState` / `UnsubscribeConversationReadState` 订阅会话的 ReadState 变化

---

## 服务端 API

### GetConversationReadCursors

获取指定会话的所有成员已读位置。

**请求**：
```protobuf
message GetConversationReadCursorsReq {
    repeated string conversationIDs = 1;
}
```

**响应**：
```protobuf
message GetConversationReadCursorsResp {
    repeated ConversationReadCursors conversationReadCursors = 1;
}

message ConversationReadCursors {
    string conversationID = 1;
    repeated ReadCursor cursors = 2;
}

message ReadCursor {
    string userID = 1;
    int64 maxReadSeq = 2;
}
```

**实现逻辑**（`conversation.go`）：
1. 通过 `GetConversationsByConversationID` 获取会话信息
2. 根据会话类型获取用户列表：
   - **群聊**：调用 `GetGroupMemberUserIDs` 获取群成员
   - **单聊**：从 conversationID 解析两个用户
3. 调用 `GetConversationUserReadSeqs` 批量获取用户的 ReadSeq

---

## 前端实现详解 (fuji-im)

### 核心 Hook：useReadStateSubscription

进入会话时订阅已读状态，离开时取消订阅：

```typescript
// use-read-state-subscription.ts
export const useReadStateSubscription = (conversationID: string | undefined) => {
    const [allReadSeq, setAllReadSeq] = useState(0);

    useEffect(() => {
        if (!conversationID) return;

        // 订阅并获取初始值
        IMSDK.subscribeConversationReadState(conversationID)
            .then(({ data }) => {
                logInfo(`[ReadState] Subscribed, allReadSeq: ${data}`);
                setAllReadSeq(data);
            })
            .catch((error) => {
                logError('[ReadState] Subscribe failed:', error);
            });

        // 监听事件更新
        const handler = (event: WSEvent<{ conversationID: string; allReadSeq: number }>) => {
            if (event.data.conversationID === conversationID) {
                logInfo(`[ReadState] Updated, allReadSeq: ${event.data.allReadSeq}`);
                setAllReadSeq(event.data.allReadSeq);
            }
        };
        IMSDK.on(CbEvents.OnConversationReadStateChanged, handler);

        return () => {
            logInfo(`[ReadState] Unsubscribing from: ${conversationID}`);
            IMSDK.unsubscribeConversationReadState(conversationID).catch((error) => {
                logError('[ReadState] Unsubscribe failed:', error);
            });
            IMSDK.off(CbEvents.OnConversationReadStateChanged, handler);
        };
    }, [conversationID]);

    return allReadSeq;
};
```

### 数据流

```
ConversationDetail (页面)
  │
  ├── useReadStateSubscription(conversationID)
  │     └── 返回 allReadSeq
  │
  └── ChatContent (消息列表)
        │
        └── CommonMessageItem (单条消息)
              │
              ├── allReadSeq (从 props 传入)
              │
              └── MessageReadStatus
                    │
                    └── 判断 message.seq <= allReadSeq
                          │
                          ├── 是 → 显示 👁️
                          └── 否 → 不显示
```

### 自动标记已读

```typescript
// use-auto-mark-conversation-read.ts
// 当用户进入会话且有未读消息时，自动调用：
IMSDK.markConversationMessageAsRead(conversationID);
```

---

## 文件清单

### openim-sdk-core

| 文件 | 说明 |
|------|------|
| `pkg/db/model_struct/data_model_struct.go` | 表结构定义（LocalReadCursor, LocalReadState） |
| `pkg/db/db_interface/databse.go` | 数据库接口定义（ReadCursorModel, ReadStateModel） |
| `pkg/db/read_cursor_model.go` | SQLite 数据库操作实现 |
| `wasm/indexdb/read_cursor_model.go` | WASM/IndexedDB 数据库操作实现 |
| `internal/conversation_msg/read_drawing.go` | 已读回执处理逻辑（doReadDrawing, doGroupReadDrawing） |
| `internal/conversation_msg/sync.go` | 同步逻辑（SyncReadCursors, syncRecentReadCursors） |
| `internal/conversation_msg/api.go` | 对外 API（GetReadState, GetReadCursors） |
| `open_im_sdk_callback/callback_client.go` | 回调接口定义（OnConversationReadStateChanged） |

### openim-sdk-js-wasm

| 文件 | 说明 |
|------|------|
| `src/sqls/localReadCursor.ts` | Cursor 表 SQL 操作 |
| `src/sqls/localReadState.ts` | State 表 SQL 操作 |
| `src/api/database/readCursor.ts` | 数据库操作封装 |
| `src/api/worker.ts` | Worker 方法注册 |
| `src/constant/index.ts` | CbEvents 事件定义 |

### openim-server

| 文件 | 说明 |
|------|------|
| `internal/rpc/msg/as_read.go` | 处理已读标记请求、广播群已读回执 |
| `internal/rpc/conversation/conversation.go` | GetConversationReadCursors API 实现 |

### openim-protocol

| 文件 | 说明 |
|------|------|
| `sdkws/sdkws.proto` | GroupHasReadTips 消息定义 |
| `constant/constant.go` | GroupHasReadReceipt (2201) 常量定义 |
| `conversation/conversation.proto` | GetConversationReadCursors 请求/响应定义 |

### fuji-im（前端应用）

| 文件 | 说明 |
|------|------|
| `src/pages/chat/conversation-detail/use-read-state-subscription.ts` | 已读状态订阅 Hook |
| `src/pages/chat/conversation-detail/use-auto-mark-conversation-read.ts` | 自动标记已读 Hook |
| `src/pages/chat/conversation-detail/message-item/message-read-status.tsx` | 已读图标显示组件 |
| `src/pages/chat/conversation-detail/message-item/common-message-item.tsx` | 消息项容器（传递 allReadSeq） |
| `src/pages/chat/conversation-detail/chat-content.tsx` | 消息列表（传递 allReadSeq） |
| `src/pages/chat/conversation-detail/index.tsx` | 会话详情页（调用 useReadStateSubscription） |
| `src/hooks/use-global-events.ts` | 全局事件处理（OnMessageSeqUpdated） |
| `src/pages/chat/conversation-detail/use-history-message-list.ts` | 消息列表管理（seq 保护逻辑） |

---

## 与原方案对比

| 方面 | 原方案 (attachedInfo) | 新方案 |
|------|----------------------|--------|
| 存储位置 | 每条消息的 attachedInfo | 独立的 cursor/state 表 |
| 查询复杂度 | O(N) 遍历消息 | O(1) seq 比较 |
| 存储开销 | 每条消息都存 | 每用户一条记录 |
| 实时性 | 需要更新消息 | 事件驱动更新 |
| 单聊/群聊 | 不同逻辑 | 统一逻辑 |
| 计算 allReadSeq | - | 排除自己后取最小值 |

---

## 注意事项

1. **计算 allReadSeq 时排除自己** - `GetAllReadSeqFromCursors(ctx, conversationID, excludeUserID)` 必须传入当前登录用户 ID
2. **allReadSeq 表示其他人的已读位置** - 不包括自己，因为自己发的消息自己肯定看过
3. **事件触发条件** - 只有 allReadSeq 变化时才触发 OnConversationReadStateChanged
4. **清理策略** - 退出群/删除会话时需要删除相关 cursor 和 state 记录
5. **新成员影响** - 新成员加入会使 allReadSeq 归零，需要前端正确处理这种情况
6. **单聊简化** - 单聊的 cursor 只有对方 1 条，allReadSeq 直接等于对方的 maxReadSeq
7. **群聊通知分离** - MarkAsReadTips (2200) 只发给自己同步，GroupHasReadTips (2201) 广播给其他成员
