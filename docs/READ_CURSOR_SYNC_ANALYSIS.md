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

## 八、同步场景实现

### 8.1 同步时机总览

| 时机 | 触发点 | 同步范围 | 优先级 | 状态 |
|------|--------|----------|--------|------|
| 连接成功/重连 | `MsgSyncEnd` | 所有会话 | P0 | ✅ 已实现 |
| 应用唤醒/数据同步 | `syncData` (CmdSyncData) | 所有会话 | P1 | ✅ 已实现 |
| 成员退出群聊 | `MemberQuitNotification` (1504) | 当前群 | P1 | ✅ 已实现 |
| 成员被踢出群聊 | `MemberKickedNotification` (1508) | 当前群 | P1 | ✅ 已实现 |
| 新成员加入群聊 | `MemberEnterNotification` (1510) | 当前群 | P1 | ✅ 已实现 |
| 新成员被邀请 | `MemberInvitedNotification` (1509) | 当前群 | P1 | ✅ 已实现 |
| 群解散 | `GroupDismissedNotification` (1511) | 当前群 | P2 | ✅ 已实现 |

---

### 8.2 应用唤醒 ✅

**场景**：
- 移动端应用从后台切换到前台
- 桌面端从息屏恢复
- 页面刷新重新加载

**触发点**：`notification.go` → `syncData` 函数末尾

**实现代码**：
```go
func (c *Conversation) syncData(c2v common.Cmd2Value) {
    // ... 现有代码 ...

    runSyncFunctions(ctx, asyncFuncs, asyncNoWait)

    // 异步同步所有 ReadCursor
    go c.syncAllReadCursors(ctx)
}
```

**同步范围**：所有会话（单聊 + 群聊）

**原因**：后台期间可能错过了推送通知

---

### 8.3 成员退出群聊 ✅

**场景**：群成员主动退出群聊

**触发点**：`notification.go` → `doNotificationManager` → `handleGroupMemberChangeForReadCursor`

**实现代码**：
```go
// handleMemberQuitForReadCursor 成员退出 - 删除 cursor，重算 allReadSeq
func (c *Conversation) handleMemberQuitForReadCursor(ctx context.Context, msg *sdkws.MsgData) {
    var detail sdkws.MemberQuitTips
    if err := utils.UnmarshalNotificationElem(msg.Content, &detail); err != nil {
        log.ZWarn(ctx, "UnmarshalNotificationElem err", err)
        return
    }

    // 跳过自己退出的情况
    if detail.QuitUser.UserID == c.loginUserID {
        return
    }

    conversationID := c.getConversationIDBySessionType(detail.Group.GroupID, constant.ReadGroupChatType)
    c.handleMemberLeftForReadCursor(ctx, conversationID, []string{detail.QuitUser.UserID})
}

// handleMemberLeftForReadCursor 成员离开 - 删除 cursor，重算 allReadSeq
func (c *Conversation) handleMemberLeftForReadCursor(ctx context.Context, conversationID string, userIDs []string) {
    for _, userID := range userIDs {
        c.db.DeleteReadCursor(ctx, conversationID, userID)
    }
    // 重新计算 allReadSeq 并通知
    c.updateReadStateAfterSync(ctx, conversationID)
}
```

**影响**：`allReadSeq` 可能会**增加**（因为读得最少的人走了）

---

### 8.4 成员被踢出群聊 ✅

**场景**：群成员被管理员踢出

**触发点**：`notification.go` → `doNotificationManager` → `handleGroupMemberChangeForReadCursor`

**实现代码**：
```go
// handleMemberKickedForReadCursor 成员被踢 - 删除 cursors，重算 allReadSeq
func (c *Conversation) handleMemberKickedForReadCursor(ctx context.Context, msg *sdkws.MsgData) {
    var detail sdkws.MemberKickedTips
    if err := utils.UnmarshalNotificationElem(msg.Content, &detail); err != nil {
        log.ZWarn(ctx, "UnmarshalNotificationElem err", err)
        return
    }

    var userIDs []string
    for _, member := range detail.KickedUserList {
        // 跳过自己
        if member.UserID != c.loginUserID {
            userIDs = append(userIDs, member.UserID)
        }
    }

    if len(userIDs) == 0 {
        return
    }

    conversationID := c.getConversationIDBySessionType(detail.Group.GroupID, constant.ReadGroupChatType)
    c.handleMemberLeftForReadCursor(ctx, conversationID, userIDs)
}
```

**影响**：与"成员退出"相同，`allReadSeq` 可能会增加

---

### 8.5 新成员加入群聊 ✅

**场景**：
- 新成员通过邀请加入（MemberInvitedNotification）
- 新成员申请加入被批准（MemberEnterNotification）

**触发点**：`notification.go` → `doNotificationManager` → `handleGroupMemberChangeForReadCursor`

**实现代码**：
```go
// handleMemberInvitedForReadCursor 成员被邀请 - 从服务器同步 cursor
func (c *Conversation) handleMemberInvitedForReadCursor(ctx context.Context, msg *sdkws.MsgData) {
    var detail sdkws.MemberInvitedTips
    if err := utils.UnmarshalNotificationElem(msg.Content, &detail); err != nil {
        log.ZWarn(ctx, "UnmarshalNotificationElem err", err)
        return
    }

    conversationID := c.getConversationIDBySessionType(detail.Group.GroupID, constant.ReadGroupChatType)
    c.handleMemberEnterForReadCursorInternal(ctx, conversationID)
}

// handleMemberEnterForReadCursor 成员进入 - 从服务器同步 cursor
func (c *Conversation) handleMemberEnterForReadCursor(ctx context.Context, msg *sdkws.MsgData) {
    var detail sdkws.MemberEnterTips
    if err := utils.UnmarshalNotificationElem(msg.Content, &detail); err != nil {
        log.ZWarn(ctx, "UnmarshalNotificationElem err", err)
        return
    }

    // 跳过自己
    if detail.EntrantUser.UserID == c.loginUserID {
        return
    }

    conversationID := c.getConversationIDBySessionType(detail.Group.GroupID, constant.ReadGroupChatType)
    c.handleMemberEnterForReadCursorInternal(ctx, conversationID)
}

// handleMemberEnterForReadCursorInternal - 从服务器同步 cursor（获取真实阅读位置）
func (c *Conversation) handleMemberEnterForReadCursorInternal(ctx context.Context, conversationID string) {
    // 不要本地创建 maxReadSeq=0 的 cursor
    // 而是从服务器同步获取真实的阅读位置
    if err := c.SyncReadCursors(ctx, []string{conversationID}); err != nil {
        log.ZWarn(ctx, "SyncReadCursors failed after member enter", err, "conversationID", conversationID)
    }
}
```

**设计说明**：
- **不直接创建 maxReadSeq=0 的 cursor**：避免 allReadSeq 错误变为 0
- **从服务器同步**：获取新成员的真实阅读位置
- 服务器可能保留重新加入成员之前的阅读记录
- 对于全新成员，服务器返回的数据也是准确的

---

### 8.6 群解散 ✅

**场景**：群被群主解散

**触发点**：`notification.go` → `doNotificationManager` → `handleGroupMemberChangeForReadCursor`

**实现代码**：
```go
// handleGroupDismissedForReadCursor 群解散 - 清理所有 cursor 和 state
func (c *Conversation) handleGroupDismissedForReadCursor(ctx context.Context, msg *sdkws.MsgData) {
    var detail sdkws.GroupDismissedTips
    if err := utils.UnmarshalNotificationElem(msg.Content, &detail); err != nil {
        log.ZWarn(ctx, "UnmarshalNotificationElem err", err)
        return
    }

    conversationID := c.getConversationIDBySessionType(detail.Group.GroupID, constant.ReadGroupChatType)

    if err := c.db.DeleteReadCursorsByConversationID(ctx, conversationID); err != nil {
        log.ZWarn(ctx, "DeleteReadCursorsByConversationID failed", err, "conversationID", conversationID)
    }

    if err := c.db.DeleteReadState(ctx, conversationID); err != nil {
        log.ZWarn(ctx, "DeleteReadState failed", err, "conversationID", conversationID)
    }
}
```

**影响**：清理本地数据，释放存储空间

---

### 8.7 实现架构

**统一入口**：所有成员变动的 ReadCursor 处理在 `doNotificationManager` 中统一处理：

```go
func (c *Conversation) doNotificationManager(c2v common.Cmd2Value) {
    ctx := c2v.Ctx
    allMsg := c2v.Value.(sdk_struct.CmdNewMsgComeToConversation).Msgs

    for conversationID, msgs := range allMsg {
        for _, msg := range msgs.Msgs {
            if msg.ContentType > constant.FriendNotificationBegin && msg.ContentType < constant.FriendNotificationEnd {
                c.relation.DoNotification(ctx, msg)
            } else if msg.ContentType > constant.UserNotificationBegin && msg.ContentType < constant.UserNotificationEnd {
                c.user.DoNotification(ctx, msg)
            } else if msg.ContentType > constant.GroupNotificationBegin && msg.ContentType < constant.GroupNotificationEnd {
                c.group.DoNotification(ctx, msg)
                // 处理成员变动的 ReadCursor 更新
                c.handleGroupMemberChangeForReadCursor(ctx, msg)
            } else {
                c.DoNotification(ctx, msg)
            }
        }
        // ...
    }
}

// handleGroupMemberChangeForReadCursor 处理群成员变动对 ReadCursor 的影响
func (c *Conversation) handleGroupMemberChangeForReadCursor(ctx context.Context, msg *sdkws.MsgData) {
    go func() {
        switch msg.ContentType {
        case constant.MemberQuitNotification:      // 1504
            c.handleMemberQuitForReadCursor(ctx, msg)
        case constant.MemberKickedNotification:    // 1508
            c.handleMemberKickedForReadCursor(ctx, msg)
        case constant.MemberInvitedNotification:   // 1509
            c.handleMemberInvitedForReadCursor(ctx, msg)
        case constant.MemberEnterNotification:     // 1510
            c.handleMemberEnterForReadCursor(ctx, msg)
        case constant.GroupDismissedNotification:  // 1511
            c.handleGroupDismissedForReadCursor(ctx, msg)
        }
    }()
}
```

**设计原则**：

1. **异步执行**：所有 ReadCursor 处理在 goroutine 中执行，不阻塞通知处理
2. **跳过自己**：成员变动时跳过当前登录用户
3. **只通知已订阅会话**：使用 `updateReadStateAfterSync` 内部的 `checkAndNotifyReadStateChanged`
4. **不修改 Group 模块**：在 Conversation 模块中独立解析通知并处理 ReadCursor
