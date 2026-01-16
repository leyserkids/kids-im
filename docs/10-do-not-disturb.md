# 免打扰设计

## 概述

OpenIM 实现会话级免打扰（Do Not Disturb, DND）功能：
- **正常用户**：通过 WebSocket + FCM 双通道接收消息
- **免打扰用户**：仅通过 WebSocket 接收消息，不触发 FCM 推送
- **被 @ 用户**：即使开启免打扰，仍会收到 FCM 推送

核心设计原则：
1. **Webhook 层过滤**：在调用 FCM 推送 webhook 前过滤免打扰用户
2. **WebSocket 不受影响**：所有用户（包括 DND）仍通过 WebSocket 正常接收消息
3. **@mention 优先**：被 @ 的用户绕过 DND 限制

---

## 一、数据模型

### 1.1 RecvMsgOpt 字段

**存储位置**：`Conversation` 表的 `RecvMsgOpt` 字段

| 值 | 常量 | 含义 |
|----|------|------|
| 0 | ReceiveMessage | 正常接收并推送 |
| 1 | NotReceiveMessage | 不接收消息（已禁用） |
| 2 | ReceiveNotNotifyMessage | 免打扰：接收但不推送 |

### 1.2 @全员 标识

| 位置 | 常量名 | 值 |
|------|--------|-----|
| Server | `constant.AtAllString` | `"AtAllTag"` |
| SDK-Core | `constant.AtAllString` | `"AtAllTag"` |

**存储位置**：消息的 `AtUserIDList` 字段

---

## 二、推送流程

### 2.1 整体流程

```
消息发送
    ↓
Push Service 接收
    ↓
┌─────────────────────────────────────────────────────────┐
│  filterBeforeOnlinePushWebhookUserIDs()                 │
│                                                         │
│  1. GetConversationOfflinePushUserIDs() → 过滤 DND 用户 │
│  2. 检测 @全员 → 恢复所有用户                           │
│  3. 检测 @specific → 恢复被 @ 的用户                    │
│  4. 排除发送者                                          │
└─────────────────────────────────────────────────────────┘
    ↓
判断是否调用 Webhook
    ├─ len(webhookUserIDs) > 0 && ContentType < 1000 → 调用 Webhook → FCM 推送
    └─ 否则 → 跳过 Webhook
    ↓
WebSocket 推送（所有用户，包括 DND）
```

### 2.2 ContentType 分类

**需要推送（触发 Webhook）**：

| 范围 | 类型 | 说明 |
|------|------|------|
| 100-123 | 聊天消息 | Text=101, Picture=102, Voice=103... |
| 200-203 | 扩展消息 | Common=200, GroupMsg=201, SignalMsg=202, CustomNotification=203 |

**不需要推送（跳过 Webhook）**：

| 范围 | 类型 | 说明 |
|------|------|------|
| 1000-5000 | 系统通知 | NotificationBegin=1000, NotificationEnd=5000 |

---

## 三、核心实现

### 3.1 过滤函数

**文件**：`openim-server/internal/push/push_handler.go`

```go
// filterBeforeOnlinePushWebhookUserIDs 过滤 webhook 调用的用户列表：
// 1. 过滤 DND 用户
// 2. 恢复被 @ 的用户（即使 DND 也推送）
// 3. 排除发送者
func (c *ConsumerHandler) filterBeforeOnlinePushWebhookUserIDs(
    ctx context.Context,
    conversationID string,
    allUserIDs []string,
    msg *sdkws.MsgData,
) []string {
    // 过滤 DND 用户
    webhookUserIDs, err := c.conversationClient.GetConversationOfflinePushUserIDs(
        ctx, conversationID, allUserIDs)
    if err != nil {
        log.ZWarn(ctx, "GetConversationOfflinePushUserIDs failed", err)
        webhookUserIDs = allUserIDs // 降级：不过滤
    }

    // 恢复被 @ 的用户
    if datautil.Contain(constant.AtAllString, msg.AtUserIDList...) {
        // @全员：所有用户都推送
        webhookUserIDs = allUserIDs
    } else {
        // @特定用户：仅恢复被 @ 的用户
        for _, atUserID := range msg.AtUserIDList {
            if !datautil.Contain(atUserID, webhookUserIDs...) &&
               datautil.Contain(atUserID, allUserIDs...) {
                webhookUserIDs = append(webhookUserIDs, atUserID)
            }
        }
    }

    // 排除发送者
    return datautil.DeleteElems(webhookUserIDs, msg.SendID)
}
```

### 3.2 调用时机

**单聊（Push2User）**：

```go
conversationID := conversationutil.GenConversationIDForSingle(msg.SendID, msg.RecvID)
webhookUserIDs := c.filterBeforeOnlinePushWebhookUserIDs(ctx, conversationID, userIDs, msg)

if len(webhookUserIDs) > 0 && msg.ContentType < constant.NotificationBegin {
    c.webhookBeforeOnlinePush(ctx, ..., webhookUserIDs, msg)
}
```

**群聊（Push2Group）**：

```go
conversationID := conversationutil.GenGroupConversationID(groupID)
webhookUserIDs := c.filterBeforeOnlinePushWebhookUserIDs(ctx, conversationID, pushToUserIDs, msg)

if len(webhookUserIDs) > 0 && msg.ContentType < constant.NotificationBegin {
    c.webhookBeforeGroupOnlinePush(ctx, ..., groupID, msg, &webhookUserIDs)
}
```

---

## 四、缓存机制

### 4.1 缓存策略

`GetConversationOfflinePushUserIDs` 使用 Redis 缓存：

- **缓存 key**：`ConversationNotReceiveMessageUserIDs:{conversationID}`
- **缓存内容**：该会话中设置了 DND 的用户 ID 列表
- **自动失效**：用户修改 DND 设置时清理缓存

### 4.2 缓存清理

**文件**：`openim-server/pkg/common/storage/controller/conversation.go`

```go
func (c *conversationDatabase) SetUserConversations(...) error {
    // ... 更新会话设置 ...

    // 清理 DND 缓存
    cache = cache.DelConversationNotReceiveMessageUserIDs(conversationIDs...)

    return cache.ChainExecDel(ctx)
}
```

### 4.3 降级策略

```go
webhookUserIDs, err := c.conversationClient.GetConversationOfflinePushUserIDs(...)
if err != nil {
    // RPC 失败时降级为不过滤，确保消息不丢失
    log.ZWarn(ctx, "GetConversationOfflinePushUserIDs failed", err)
    webhookUserIDs = allUserIDs
}
```

---

## 五、测试场景

### 5.1 单聊测试

| 场景 | 预期结果 |
|------|----------|
| 接收者开启 DND | 不触发 webhook，WebSocket 正常接收 |
| 接收者开启 DND + 被 @ | 触发 webhook |
| 发送系统通知 | 不触发 webhook |

### 5.2 群聊测试

| 场景 | 预期结果 |
|------|----------|
| 部分成员开启 DND | 仅非 DND 成员触发 webhook |
| @全员 | 所有成员触发 webhook（包括 DND 用户） |
| @特定 DND 用户 | 被 @ 的 DND 用户也触发 webhook |
| 发送者发消息 | 发送者自己不触发 webhook |

---

## 六、修改文件清单

| 文件 | 修改内容 |
|------|----------|
| `internal/push/push_handler.go` | 添加 `filterBeforeOnlinePushWebhookUserIDs`，修改 `Push2User` 和 `Push2Group` |
| `internal/push/callback.go` | Webhook 请求添加 `UserIDs` 字段 |
| `pkg/callbackstruct/push.go` | `CallbackBeforeSuperGroupOnlinePushReq` 添加 `UserIDs` |
| `pkg/common/storage/controller/conversation.go` | `SetUserConversations` 添加缓存清理 |
