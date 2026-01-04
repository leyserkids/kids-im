# ReadCursor 同步策略设计文档

## 一、业务背景

### 1.1 用户规模
- **500 家学校**
- **每家学校 50 个老师**
- 总用户量：约 25,000 用户

### 1.2 聊天特点
- 每个学校内部聊天，不跨校
- 群聊成员都是学校内部老师
- 每个老师最多 49 个单聊对象
- 群聊规模：最多 50 人/群

### 1.3 数据量估算
- 假设用户有 50 个单聊 + 10 个群聊
- **注意**：同步时排除当前用户自己的 cursor，每个会话少一行
- 单聊：50 × 1 = 50 个 cursor（只同步对方）
- 群聊：10 × 49 = 490 个 cursor（排除自己）
- 总共：约 540 个 cursor，数据量不大

---

## 二、前端接口设计

### 2.1 多窗口场景说明

**场景**：一个用户可能在多个浏览器窗口/标签页同时打开不同的聊天会话，每个窗口需要独立监听自己会话的 `allReadSeq`。

**设计**：使用订阅模式，支持同时订阅多个会话。

### 2.2 订阅会话已读状态 - `SubscribeConversationReadState`

**功能**：
- 订阅指定会话的 `allReadSeq` 变化
- 立即返回本地数据库中的当前值（无数据时返回 0）
- 后续变化通过回调通知
- 支持同时订阅多个会话（多窗口场景）

**设计参考**：与现有 `subscribeUsersStatus` API 风格一致

**接口**：
```typescript
// Go SDK
func (c *Conversation) SubscribeConversationReadState(ctx context.Context, conversationID string) (allReadSeq int64, err error)

// JS SDK
subscribeConversationReadState(conversationID: string): Promise<number>  // 返回 allReadSeq
```

**行为**：
1. 将 `conversationID` 加入订阅集合
2. 查询本地数据库，返回当前 `allReadSeq`（无数据时返回 0）
3. 后续该会话的已读状态变化通过 `OnConversationReadStateChanged` 回调通知

### 2.3 取消订阅 - `UnsubscribeConversationReadState`

**功能**：取消订阅指定会话的 `allReadSeq` 变化

**接口**：
```typescript
// Go SDK
func (c *Conversation) UnsubscribeConversationReadState(ctx context.Context, conversationID string) error

// JS SDK
unsubscribeConversationReadState(conversationID: string): Promise<void>
```

**行为**：
1. 将 `conversationID` 从订阅集合移除
2. 不再触发该会话的回调

### 2.4 已读状态变化回调 - `OnConversationReadStateChanged`

**触发条件**：
- 已订阅会话的 `allReadSeq` 发生变化时触发
- 注意：初始值通过 `SubscribeConversationReadState` 返回值获取，不通过回调

**回调数据**：
```json
{
  "conversationID": "sg_xxx",
  "allReadSeq": 123
}
```

### 2.5 获取消息已读成员列表 - `GetGroupMessageReadMemberList`

**功能**：用户右键消息时获取已读成员列表（用于显示"谁读了这条消息"）

**接口**：
```typescript
// Go SDK
func (c *Conversation) GetGroupMessageReadMemberList(ctx context.Context, conversationID string, seq int64) ([]*GroupMessageReceipt, error)

// JS SDK
getGroupMessageReadMemberList(conversationID: string, seq: number): Promise<GroupMessageReceipt[]>
```

**现有接口**：已实现，无需修改

---

## 三、同步策略（最终方案）

### 3.1 采用方案：连接成功后全量同步

```
连接成功/重连 (MsgSyncEnd)
  └── syncAllReadCursors()
        └── 获取所有会话（单聊 + 群聊）
        └── SyncReadCursors(ctx, allConversationIDs)
        └── 对所有已订阅的会话，触发 OnConversationReadStateChanged

实时更新
  └── 收到 MarkAsReadTips (2200) / GroupHasReadTips (2201)
        └── 更新本地 ReadCursor
        └── 如果该会话已订阅，触发 OnConversationReadStateChanged
```

### 3.2 选择理由

1. **简单可靠**：一个地方同步，逻辑清晰
2. **数据量可控**：业务场景决定了数据量不大（~600 cursor）
3. **用户体验好**：进入任何会话都立即可用
4. **无漏洞**：不存在状态不一致问题

### 3.3 实现代码

```go
// sync.go - 简化后的同步函数
// 函数名通用：syncAllReadCursors，不加 OnConnect 后缀，因为这个函数本身就是同步所有的
func (c *Conversation) syncAllReadCursors(ctx context.Context) {
    allConversations, err := c.db.GetAllConversations(ctx)
    if err != nil {
        log.ZWarn(ctx, "GetAllConversations err", err)
        return
    }

    var conversationIDs []string
    for _, conv := range allConversations {
        if conv.ConversationType == constant.SingleChatType ||
           conv.ConversationType == constant.ReadGroupChatType {
            conversationIDs = append(conversationIDs, conv.ConversationID)
        }
    }

    if len(conversationIDs) == 0 {
        log.ZDebug(ctx, "No conversations to sync ReadCursors")
        return
    }

    log.ZDebug(ctx, "syncAllReadCursors", "count", len(conversationIDs))
    if err := c.SyncReadCursors(ctx, conversationIDs); err != nil {
        log.ZWarn(ctx, "SyncReadCursors err", err, "conversationIDs", conversationIDs)
    }

    // 对所有已订阅的会话，触发回调
    c.notifySubscribedConversationsReadStateChanged(ctx)
}
```

---

## 四、激活会话管理

### 4.1 订阅集合（内存存储，不持久化）

```go
// conversation.go
type Conversation struct {
    // ... 现有字段

    // 已订阅的会话集合（内存中，不持久化）
    // 使用 map[string]struct{} 作为 set，节省内存
    subscribedConversations   map[string]struct{}
    subscribedConversationsMu sync.RWMutex
}

// SubscribeConversationReadState 订阅会话的 allReadSeq 变化
// 返回本地数据库中的当前值，无数据时返回 0
func (c *Conversation) SubscribeConversationReadState(ctx context.Context, conversationID string) (int64, error) {
    c.subscribedConversationsMu.Lock()
    if c.subscribedConversations == nil {
        c.subscribedConversations = make(map[string]struct{})
    }
    c.subscribedConversations[conversationID] = struct{}{}
    c.subscribedConversationsMu.Unlock()

    // 查询本地数据库，无数据时返回 0
    state, err := c.db.GetReadState(ctx, conversationID)
    if err != nil || state == nil {
        return 0, nil
    }
    return state.AllReadSeq, nil
}

// UnsubscribeConversationReadState 取消订阅
func (c *Conversation) UnsubscribeConversationReadState(ctx context.Context, conversationID string) error {
    c.subscribedConversationsMu.Lock()
    delete(c.subscribedConversations, conversationID)
    c.subscribedConversationsMu.Unlock()
    return nil
}

// isConversationSubscribed 检查会话是否已订阅
func (c *Conversation) isConversationSubscribed(conversationID string) bool {
    c.subscribedConversationsMu.RLock()
    defer c.subscribedConversationsMu.RUnlock()
    _, ok := c.subscribedConversations[conversationID]
    return ok
}

// getSubscribedConversations 获取所有已订阅的会话
func (c *Conversation) getSubscribedConversations() []string {
    c.subscribedConversationsMu.RLock()
    defer c.subscribedConversationsMu.RUnlock()
    result := make([]string, 0, len(c.subscribedConversations))
    for convID := range c.subscribedConversations {
        result = append(result, convID)
    }
    return result
}
```

### 4.2 回调触发逻辑

```go
// 通知单个会话的 allReadSeq 变化
func (c *Conversation) notifyConversationReadStateChanged(ctx context.Context, conversationID string) {
    state, err := c.db.GetReadState(ctx, conversationID)
    var allReadSeq int64
    if err == nil && state != nil {
        allReadSeq = state.AllReadSeq
    }

    c.msgListener().OnConversationReadStateChanged(utils.StructToJsonString(map[string]interface{}{
        "conversationID": conversationID,
        "allReadSeq":     allReadSeq,
    }))
}

// 通知所有已订阅会话的 allReadSeq 变化（重连后调用）
func (c *Conversation) notifySubscribedConversationsReadStateChanged(ctx context.Context) {
    for _, convID := range c.getSubscribedConversations() {
        c.notifyConversationReadStateChanged(ctx, convID)
    }
}

// 在 ReadCursor 更新后检查是否需要触发回调
func (c *Conversation) checkAndNotifyReadStateChanged(ctx context.Context, conversationID string) {
    if !c.isConversationSubscribed(conversationID) {
        return  // 未订阅，不触发
    }
    c.notifyConversationReadStateChanged(ctx, conversationID)
}
```

---

## 五、需要修改的文件

### 5.1 Go SDK

| 文件 | 修改内容 |
|------|---------|
| `internal/conversation_msg/conversation.go` | 新增 `subscribedConversations` 字段和相关方法 |
| `internal/conversation_msg/conversation.go` | 新增 `SubscribeConversationReadState`、`UnsubscribeConversationReadState` |
| `internal/conversation_msg/sync.go` | 重写为 `syncAllReadCursors` |
| `internal/conversation_msg/sync.go` | 删除 `syncReadCursorsOnEnter`、`getConversationIDsForReadCursorSync`、`syncRecentReadCursors` |
| `internal/conversation_msg/notification.go` | 更新调用处 |
| `internal/conversation_msg/read_drawing.go` | 更新回调触发逻辑，使用 `checkAndNotifyReadStateChanged` |
| `open_im_sdk/conversation_msg.go` | 暴露新接口 |
| `sdk_callback/callback.go` | 新增 `OnConversationReadStateChanged` 回调 |

### 5.2 JS SDK

| 文件 | 修改内容 |
|------|---------|
| `src/api/index.ts` | 新增 `subscribeConversationReadState`、`unsubscribeConversationReadState` |

---

## 六、旧实现分析（供参考）

### 6.1 旧同步策略的问题

| 问题 | 说明 |
|------|------|
| 命名不清晰 | `syncReadCursorsOnEnter` 只同步群聊，但名字没体现 |
| 逻辑复杂 | 两个地方同步，容易混淆 |
| 可能重复同步 | 群聊在重连时和进入时可能被同步两次 |

### 6.2 旧实现代码路径

```
旧实现：
├── 连接成功/重连 → syncRecentReadCursors → 所有单聊 + 10个群聊
└── 进入群聊 → syncReadCursorsOnEnter → 当前群聊

新实现：
└── 连接成功/重连 → syncAllReadCursors → 所有会话（单聊 + 群聊）
```

---

## 七、命名规范总结

| 函数 | 命名 | 说明 |
|------|------|------|
| 同步所有 ReadCursor | `syncAllReadCursors` | 通用名，不加场景后缀 |
| 同步指定会话 | `SyncReadCursors` | 已有，保持不变 |
| 订阅会话（公开） | `SubscribeConversationReadState` | 大写开头，公开接口 |
| 取消订阅（公开） | `UnsubscribeConversationReadState` | 大写开头，公开接口 |
| 检查是否订阅（内部） | `isConversationSubscribed` | 内部方法，小写开头 |
| 获取订阅列表（内部） | `getSubscribedConversations` | 内部方法，小写开头 |
| 通知单个会话变化 | `notifyConversationReadStateChanged` | 通用名 |
| 通知所有订阅会话 | `notifySubscribedConversationsReadStateChanged` | 描述实际行为 |
| 检查并通知 | `checkAndNotifyReadStateChanged` | 通用名，描述实际行为 |
| 回调 | `OnConversationReadStateChanged` | 通用名，会话级别的回调 |

---

## 八、待实现的同步场景

以下场景需要在未来实现，以确保 ReadCursor 数据的准确性。

### 8.1 同步时机总览

| 时机 | 触发点 | 同步范围 | 优先级 | 状态 |
|------|--------|----------|--------|------|
| 连接成功/重连 | `MsgSyncEnd` | 所有会话 | P0 | ✅ 已实现 |
| 应用唤醒 | `CmdWakeUpDataSync` | 所有会话? | P1 | ❌ 待实现 |
| 成员退出群聊 | `MemberQuitNotification` | 当前群 | P1 | ❌ 待实现 |
| 成员被踢出群聊 | `MemberKickedNotification` | 当前群 | P1 | ❌ 待实现 |
| 新成员加入群聊 | `MemberEnterNotification` | 当前群 | P1 | ❌ 待实现 |
| 群解散 | `GroupDismissedNotification` | 当前群 | P2 | ❌ 待实现 |

---

### 8.2 应用唤醒 ❌

**场景**：
- 移动端应用从后台切换到前台
- 桌面端从息屏恢复

**触发点**：`MsgSyncer.handlePushMsgAndEvent` → `case CmdWakeUpDataSync`

**处理逻辑**：
```go
func (m *MsgSyncer) doWakeupDataSync(ctx context.Context) {
    // 现有逻辑...

    // 新增：同步最近活跃会话的 cursor
    m.syncRecentReadCursors(ctx)
}
```

**同步范围**：最近活跃的 N 个会话

**原因**：后台期间可能错过了推送通知

---

### 8.3 成员退出群聊 ❌

**场景**：群成员主动退出群聊

**触发点**：`group/notification.go` → `case MemberQuitNotification`

**处理逻辑**：
```go
func (c *Conversation) handleMemberQuit(ctx context.Context, conversationID, userID string) {
    // 1. 删除该成员的 cursor
    c.db.DeleteReadCursor(ctx, conversationID, userID)

    // 2. 重新计算 allReadSeq（可能会增加）
    newAllReadSeq := c.db.GetAllReadSeqFromCursors(ctx, conversationID, c.loginUserID)

    // 3. 更新 state
    c.db.UpsertReadState(ctx, &LocalReadState{
        ConversationID: conversationID,
        AllReadSeq:     newAllReadSeq,
    })

    // 4. 通知前端（如果已订阅）
    c.checkAndNotifyReadStateChanged(ctx, conversationID)
}
```

**影响**：`allReadSeq` 可能会**增加**（因为读得最少的人走了）

---

### 8.4 成员被踢出群聊 ❌

**场景**：群成员被管理员踢出

**触发点**：`group/notification.go` → `case MemberKickedNotification`

**处理逻辑**：与"成员退出"相同，需要对每个被踢成员执行 `handleMemberQuit`

---

### 8.5 新成员加入群聊 ❌

**场景**：
- 新成员通过邀请加入
- 新成员申请加入被批准

**触发点**：
- `group/notification.go` → `case MemberEnterNotification`
- `group/notification.go` → `case MemberInvitedNotification`

**处理逻辑**：
```go
func (c *Conversation) handleMemberEnter(ctx context.Context, conversationID, userID string) {
    // 1. 为新成员创建 cursor（初始 maxReadSeq = 0）
    cursor := &model_struct.LocalReadCursor{
        ConversationID: conversationID,
        UserID:         userID,
        MaxReadSeq:     0,
    }
    c.db.UpsertReadCursor(ctx, cursor)

    // 2. 重新计算 allReadSeq（变为 0）
    c.db.UpsertReadState(ctx, &LocalReadState{
        ConversationID: conversationID,
        AllReadSeq:     0,
    })

    // 3. 通知前端（如果已订阅）
    c.checkAndNotifyReadStateChanged(ctx, conversationID)
}
```

**影响**：`allReadSeq` 会变为 **0**（因为新成员还没读任何消息）

**注意**：这会导致所有消息暂时显示为"未全部已读"，直到新成员开始阅读

---

### 8.6 群解散 ❌

**场景**：群被群主解散

**触发点**：`group/notification.go` → `case GroupDismissedNotification`

**处理逻辑**：
```go
func (c *Conversation) handleGroupDismissed(ctx context.Context, conversationID string) {
    // 1. 删除该群的所有 cursor
    c.db.DeleteReadCursorsByConversationID(ctx, conversationID)

    // 2. 删除该群的 state
    c.db.DeleteReadState(ctx, conversationID)
}
```

**影响**：清理本地数据，释放存储空间

---

### 8.7 实现建议

1. **异步执行**：所有同步操作应异步执行，不阻塞 UI 渲染

2. **成员变化及时处理**：成员加入/退出时立即更新本地数据，无需从服务端同步

3. **只通知已订阅会话**：使用 `checkAndNotifyReadStateChanged` 确保只有前端关心的会话才触发回调
