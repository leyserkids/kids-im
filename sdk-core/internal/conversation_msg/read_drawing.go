// Copyright © 2023 OpenIM SDK. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package conversation_msg

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/openimsdk/openim-sdk-core/v3/pkg/common"
	"github.com/openimsdk/openim-sdk-core/v3/pkg/constant"
	"github.com/openimsdk/openim-sdk-core/v3/pkg/db/model_struct"
	"github.com/openimsdk/openim-sdk-core/v3/pkg/utils"
	"github.com/openimsdk/openim-sdk-core/v3/sdk_struct"
	"github.com/openimsdk/tools/errs"
	"github.com/openimsdk/tools/utils/datautil"

	"github.com/openimsdk/protocol/sdkws"
	"github.com/openimsdk/tools/log"
)

func (c *Conversation) getConversationMaxSeqAndSetHasRead(ctx context.Context, conversationID string) error {
	maxSeq, err := c.db.GetConversationNormalMsgSeq(ctx, conversationID)
	if err != nil {
		return err
	}
	if maxSeq == 0 {
		return nil
	}
	return c.setConversationHasReadSeq(ctx, conversationID, maxSeq)
}

// mark a conversation's all message as read
func (c *Conversation) markConversationMessageAsRead(ctx context.Context, conversationID string) error {
	c.conversationSyncMutex.Lock()
	defer c.conversationSyncMutex.Unlock()
	conversation, err := c.db.GetConversation(ctx, conversationID)
	if err != nil {
		return err
	}
	if conversation.UnreadCount == 0 {
		log.ZWarn(ctx, "unread count is 0", nil, "conversationID", conversationID)
		return nil
	}
	// get the maximum sequence number of messages in the table that are not sent by oneself
	peerUserMaxSeq, err := c.db.GetConversationPeerNormalMsgSeq(ctx, conversationID)
	if err != nil {
		return err
	}
	// get the maximum sequence number of messages in the table
	maxSeq, err := c.db.GetConversationNormalMsgSeq(ctx, conversationID)
	if err != nil {
		return err
	}
	switch conversation.ConversationType {
	case constant.SingleChatType:
		msgs, err := c.db.GetUnreadMessage(ctx, conversationID)
		if err != nil {
			return err
		}
		log.ZDebug(ctx, "get unread message", "msgs", len(msgs))
		msgIDs, seqs := c.getAsReadMsgMapAndList(ctx, msgs)
		if len(seqs) == 0 {
			log.ZWarn(ctx, "seqs is empty", nil, "conversationID", conversationID)
			if err := c.markConversationAsReadServer(ctx, conversationID, maxSeq, seqs); err != nil {
				return err
			}
		} else {
			log.ZDebug(ctx, "markConversationMessageAsRead", "conversationID", conversationID, "seqs",
				seqs, "peerUserMaxSeq", peerUserMaxSeq, "maxSeq", maxSeq)
			if err := c.markConversationAsReadServer(ctx, conversationID, maxSeq, seqs); err != nil {
				return err
			}
			_, err = c.db.MarkConversationMessageAsReadDB(ctx, conversationID, msgIDs)
			if err != nil {
				log.ZWarn(ctx, "MarkConversationMessageAsRead err", err, "conversationID", conversationID, "msgIDs", msgIDs)
			}
		}
	case constant.ReadGroupChatType, constant.NotificationChatType:
		log.ZDebug(ctx, "markConversationMessageAsRead", "conversationID", conversationID, "peerUserMaxSeq", peerUserMaxSeq, "maxSeq", maxSeq)
		if err := c.markConversationAsReadServer(ctx, conversationID, maxSeq, nil); err != nil {
			return err
		}
	}

	if err := c.db.UpdateColumnsConversation(ctx, conversationID, map[string]interface{}{"unread_count": 0}); err != nil {
		log.ZError(ctx, "UpdateColumnsConversation err", err, "conversationID", conversationID)
	}
	log.ZDebug(ctx, "update columns sucess")
	c.unreadChangeTrigger(ctx, conversationID, peerUserMaxSeq == maxSeq)
	return nil
}

// mark a conversation's message as read by seqs
func (c *Conversation) markMessagesAsReadByMsgID(ctx context.Context, conversationID string, msgIDs []string) error {
	_, err := c.db.GetConversation(ctx, conversationID)
	if err != nil {
		return err
	}
	msgs, err := c.db.GetMessagesByClientMsgIDs(ctx, conversationID, msgIDs)
	if err != nil {
		return err
	}
	if len(msgs) == 0 {
		return nil
	}
	var hasReadSeq = msgs[0].Seq
	maxSeq, err := c.db.GetConversationNormalMsgSeq(ctx, conversationID)
	if err != nil {
		return err
	}
	markAsReadMsgIDs, seqs := c.getAsReadMsgMapAndList(ctx, msgs)
	log.ZDebug(ctx, "msgs len", "markAsReadMsgIDs", len(markAsReadMsgIDs), "seqs", seqs)
	if len(seqs) == 0 {
		log.ZWarn(ctx, "seqs is empty", nil, "conversationID", conversationID)
		return nil
	}
	if err := c.markMsgAsRead2Server(ctx, conversationID, seqs); err != nil {
		return err
	}
	decrCount, err := c.db.MarkConversationMessageAsReadDB(ctx, conversationID, markAsReadMsgIDs)
	if err != nil {
		return err
	}
	if err := c.db.DecrConversationUnreadCount(ctx, conversationID, decrCount); err != nil {
		log.ZError(ctx, "decrConversationUnreadCount err", err, "conversationID", conversationID,
			"decrCount", decrCount)
	}
	c.unreadChangeTrigger(ctx, conversationID, hasReadSeq == maxSeq && msgs[0].SendID != c.loginUserID)
	return nil
}

func (c *Conversation) getAsReadMsgMapAndList(ctx context.Context,
	msgs []*model_struct.LocalChatLog) (asReadMsgIDs []string, seqs []int64) {
	for _, msg := range msgs {
		if !msg.IsRead && msg.SendID != c.loginUserID {
			if msg.Seq == 0 {
				log.ZWarn(ctx, "exception seq", errors.New("exception message "), "msg", msg)
			} else {
				asReadMsgIDs = append(asReadMsgIDs, msg.ClientMsgID)
				seqs = append(seqs, msg.Seq)
			}
		} else {
			log.ZWarn(ctx, "msg can't marked as read", nil, "msg", msg)
		}
	}
	return
}

func (c *Conversation) unreadChangeTrigger(ctx context.Context, conversationID string, latestMsgIsRead bool) {
	if latestMsgIsRead {
		c.doUpdateConversation(common.Cmd2Value{Value: common.UpdateConNode{ConID: conversationID,
			Action: constant.UpdateLatestMessageReadState, Args: []string{conversationID}}, Ctx: ctx})
	}
	c.doUpdateConversation(common.Cmd2Value{Value: common.UpdateConNode{ConID: conversationID,
		Action: constant.ConChange, Args: []string{conversationID}}, Ctx: ctx})
	c.doUpdateConversation(common.Cmd2Value{Value: common.UpdateConNode{Action: constant.TotalUnreadMessageChanged},
		Ctx: ctx})
}

func (c *Conversation) doUnreadCount(ctx context.Context, conversation *model_struct.LocalConversation, hasReadSeq int64, seqs []int64) error {
	if conversation.ConversationType == constant.SingleChatType {
		if len(seqs) != 0 {
			hasReadMessage, err := c.db.GetMessageBySeq(ctx, conversation.ConversationID, hasReadSeq)
			if err != nil {
				return err
			}
			if hasReadMessage.IsRead {
				return errs.New("read info from self can be ignored").Wrap()

			} else {
				_, err := c.db.MarkConversationMessageAsReadBySeqs(ctx, conversation.ConversationID, seqs)
				if err != nil {
					return err
				}
			}

		} else {
			return errs.New("seqList is empty", "conversationID", conversation.ConversationID, "hasReadSeq", hasReadSeq).Wrap()
		}
		currentMaxSeq := c.maxSeqRecorder.Get(conversation.ConversationID)
		if currentMaxSeq == 0 {
			return errs.New("currentMaxSeq is 0", "conversationID", conversation.ConversationID).Wrap()
		} else {
			unreadCount := currentMaxSeq - hasReadSeq
			if unreadCount < 0 {
				log.ZWarn(ctx, "unread count is less than 0", nil, "conversationID", conversation.ConversationID, "currentMaxSeq", currentMaxSeq, "hasReadSeq", hasReadSeq)
				unreadCount = 0
			}
			if err := c.db.UpdateColumnsConversation(ctx, conversation.ConversationID, map[string]interface{}{"unread_count": unreadCount}); err != nil {
				return err
			}
		}
		latestMsg := &sdk_struct.MsgStruct{}
		if err := json.Unmarshal([]byte(conversation.LatestMsg), latestMsg); err != nil {
			log.ZError(ctx, "Unmarshal err", err, "conversationID", conversation.ConversationID, "latestMsg", conversation.LatestMsg)
			return err
		}
		if (!latestMsg.IsRead) && datautil.Contain(latestMsg.Seq, seqs...) {
			c.doUpdateConversation(common.Cmd2Value{Value: common.UpdateConNode{ConID: conversation.ConversationID,
				Action: constant.UpdateLatestMessageReadState, Args: []string{conversation.ConversationID}}, Ctx: ctx})
		}
	} else {
		if err := c.db.UpdateColumnsConversation(ctx, conversation.ConversationID, map[string]interface{}{"unread_count": 0}); err != nil {
			log.ZError(ctx, "UpdateColumnsConversation err", err, "conversationID", conversation.ConversationID)
			return err
		}
	}
	c.doUpdateConversation(common.Cmd2Value{Value: common.UpdateConNode{ConID: conversation.ConversationID, Action: constant.ConChange, Args: []string{conversation.ConversationID}}})
	c.doUpdateConversation(common.Cmd2Value{Value: common.UpdateConNode{Action: constant.TotalUnreadMessageChanged}})

	return nil
}

// doReadDrawing handles HasReadReceipt notifications (contentType 2200)
//
// This notification is triggered by the server in the following scenarios:
//   - SetConversationHasReadSeq: User sets read position
//   - MarkMsgsAsRead: User marks specific messages as read
//   - MarkConversationAsRead: User marks entire conversation as read
//
// The notification target varies by conversation type:
//   - SingleChat: Sent to the other party (告诉对方"我读了你的消息")
//   - GroupChat: Sent to self only (同步自己在其他设备的已读状态)
//
// IMPORTANT: For group chats, MarkAsReadTips (2200) is only sent to SELF to sync read status.
// The broadcast to OTHER group members uses GroupHasReadTips (2201), handled by doGroupReadDrawing.
// Therefore, in group chat scenario, we should NOT update read cursors here - that's doGroupReadDrawing's job.
func (c *Conversation) doReadDrawing(ctx context.Context, msg *sdkws.MsgData) error {
	tips := &sdkws.MarkAsReadTips{}
	err := utils.UnmarshalNotificationElem(msg.Content, tips)
	if err != nil {
		log.ZWarn(ctx, "UnmarshalNotificationElem err", err, "msg", msg)
		return err
	}
	log.ZDebug(ctx, "do readDrawing", "tips", tips)
	conversation, err := c.db.GetConversation(ctx, tips.ConversationID)
	if err != nil {
		log.ZWarn(ctx, "GetConversation err", err, "conversationID", tips.ConversationID)
		return err

	}
	if tips.MarkAsReadUserID != c.loginUserID {
		// Notification from another user - only applies to single chat
		// For group chat, other users' read receipts come via GroupHasReadTips (2201)
		switch conversation.ConversationType {
		case constant.SingleChatType:
			if len(tips.Seqs) == 0 {
				return errs.New("tips Seqs is empty").Wrap()
			}
			messages, err := c.db.GetMessagesBySeqs(ctx, tips.ConversationID, tips.Seqs)
			if err != nil {
				log.ZWarn(ctx, "GetMessagesBySeqs err", err, "conversationID", tips.ConversationID, "seqs", tips.Seqs)
				return err

			}
			latestMsg := &sdk_struct.MsgStruct{}
			if err := json.Unmarshal([]byte(conversation.LatestMsg), latestMsg); err != nil {
				log.ZWarn(ctx, "Unmarshal err", err, "conversationID", tips.ConversationID, "latestMsg", conversation.LatestMsg)
				return err
			}
			var successMsgIDs []string
			for _, message := range messages {
				attachInfo := sdk_struct.AttachedInfoElem{}
				_ = utils.JsonStringToStruct(message.AttachedInfo, &attachInfo)
				attachInfo.HasReadTime = msg.SendTime
				message.AttachedInfo = utils.StructToJsonString(attachInfo)
				message.IsRead = true
				if err = c.db.UpdateMessage(ctx, tips.ConversationID, message); err != nil {
					log.ZWarn(ctx, "UpdateMessage err", err, "conversationID", tips.ConversationID, "message", message)
					return err
				} else {
					if latestMsg.ClientMsgID == message.ClientMsgID {
						latestMsg.IsRead = message.IsRead
						conversation.LatestMsg = utils.StructToJsonString(latestMsg)
						c.doUpdateConversation(common.Cmd2Value{Value: common.UpdateConNode{ConID: conversation.ConversationID, Action: constant.AddConOrUpLatMsg, Args: *conversation}, Ctx: ctx})

					}
					successMsgIDs = append(successMsgIDs, message.ClientMsgID)
				}
			}
			var messageReceiptResp = []*sdk_struct.MessageReceipt{{UserID: tips.MarkAsReadUserID, MsgIDList: successMsgIDs,
				SessionType: conversation.ConversationType, ReadTime: msg.SendTime}}
			c.msgListener().OnRecvC2CReadReceipt(utils.StructToJsonString(messageReceiptResp))

		case constant.ReadGroupChatType:
			// For group chat, MarkAsReadTips from other users should NOT happen here.
			// The server sends GroupHasReadTips (2201) to broadcast other members' read status.
			// If we somehow receive this, just log and ignore - don't update cursors.
			log.ZWarn(ctx, "Unexpected MarkAsReadTips from other user in group chat, ignoring",
				nil, "conversationID", tips.ConversationID, "markAsReadUserID", tips.MarkAsReadUserID)
		}

	} else {
		// Notification about self's read action - sync from other devices
		return c.doUnreadCount(ctx, conversation, tips.HasReadSeq, tips.Seqs)
	}
	return nil
}

// isRecordNotFoundError checks if the error is a record not found error
// Works for both GORM (SQLite) and WASM (IndexedDB) environments
func isRecordNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	// Check for errs.ErrRecordNotFound (used in WASM/IndexedDB environment)
	if errs.ErrRecordNotFound.Is(err) {
		return true
	}
	// Check error message as fallback
	errMsg := err.Error()
	return errMsg == "record not found" || errMsg == "ErrRecordNotFound"
}

// updateReadCursorAndAllReadSeq updates the read cursor and recalculates allReadSeq if needed.
// Returns (allReadSeqChanged, newAllReadSeq).
func (c *Conversation) updateReadCursorAndAllReadSeq(ctx context.Context, conversationID, userID string, maxReadSeq int64) (bool, int64) {
	// Get current state
	state, stateErr := c.db.GetReadState(ctx, conversationID)

	// Get old cursor to check if this user was the min holder
	oldCursor, cursorErr := c.db.GetReadCursor(ctx, conversationID, userID)
	var oldReadSeq int64
	var isNewCursor bool
	if isRecordNotFoundError(cursorErr) {
		isNewCursor = true
	} else if cursorErr == nil {
		oldReadSeq = oldCursor.MaxReadSeq
		// Skip if no change
		if maxReadSeq <= oldReadSeq {
			return false, 0
		}
	} else {
		log.ZWarn(ctx, "GetReadCursor err", cursorErr, "conversationID", conversationID, "userID", userID)
		return false, 0
	}

	// Upsert the cursor
	newCursor := &model_struct.LocalReadCursor{
		ConversationID: conversationID,
		UserID:         userID,
		MaxReadSeq:     maxReadSeq,
	}
	if err := c.db.UpsertReadCursor(ctx, newCursor); err != nil {
		log.ZWarn(ctx, "UpsertReadCursor err", err, "conversationID", conversationID, "userID", userID)
		return false, 0
	}

	// Calculate new allReadSeq
	var allReadSeqChanged bool
	var newAllReadSeq int64

	if isRecordNotFoundError(stateErr) || state == nil {
		// First time - need to calculate allReadSeq from scratch
		allSeq, err := c.db.GetAllReadSeqFromCursors(ctx, conversationID)
		if err != nil {
			log.ZWarn(ctx, "GetAllReadSeqFromCursors err", err, "conversationID", conversationID)
			return false, 0
		}
		newAllReadSeq = allSeq
		allReadSeqChanged = true

		// Create new state
		newState := &model_struct.LocalReadState{
			ConversationID: conversationID,
			AllReadSeq:     newAllReadSeq,
		}
		if err := c.db.UpsertReadState(ctx, newState); err != nil {
			log.ZWarn(ctx, "UpsertReadState err", err, "conversationID", conversationID)
		}
	} else if stateErr == nil {
		// State exists - check if allReadSeq needs recalculation
		if isNewCursor {
			// New user added - allReadSeq may decrease
			if maxReadSeq < state.AllReadSeq || state.AllReadSeq == 0 {
				newAllReadSeq = maxReadSeq
				allReadSeqChanged = true
			}
		} else if oldReadSeq == state.AllReadSeq {
			// The updated user was the min holder - need to recalculate
			allSeq, err := c.db.GetAllReadSeqFromCursors(ctx, conversationID)
			if err != nil {
				log.ZWarn(ctx, "GetAllReadSeqFromCursors err", err, "conversationID", conversationID)
				return false, 0
			}
			if allSeq != state.AllReadSeq {
				newAllReadSeq = allSeq
				allReadSeqChanged = true
			}
		}
		// If this user wasn't the min holder and it's not a new cursor, allReadSeq stays the same

		if allReadSeqChanged {
			state.AllReadSeq = newAllReadSeq
			if err := c.db.UpsertReadState(ctx, state); err != nil {
				log.ZWarn(ctx, "UpsertReadState err", err, "conversationID", conversationID)
			}
		}
	}

	return allReadSeqChanged, newAllReadSeq
}

// doGroupReadDrawing handles GroupHasReadReceipt notifications (contentType 2201)
//
// This notification is triggered by the server's broadcastGroupHasReadReceipt function,
// which is called when a group member marks the conversation as read (MarkConversationAsRead).
//
// Key differences from doReadDrawing (2200):
//   - MarkAsReadTips (2200) for group chat is sent ONLY to SELF (for multi-device sync)
//   - GroupHasReadTips (2201) is broadcast to ALL OTHER group members
//
// This function updates the read cursor for the user who read the messages,
// and recalculates allReadSeq (the minimum read position across all members).
// The allReadSeq is used by frontend to determine "all members have read up to this point".
func (c *Conversation) doGroupReadDrawing(ctx context.Context, msg *sdkws.MsgData) error {
	tips := &sdkws.GroupHasReadTips{}
	err := utils.UnmarshalNotificationElem(msg.Content, tips)
	if err != nil {
		log.ZWarn(ctx, "UnmarshalNotificationElem err", err, "msg", msg)
		return err
	}
	log.ZDebug(ctx, "doGroupReadDrawing", "tips", tips)

	// Skip if this is our own read receipt
	if tips.UserID == c.loginUserID {
		return nil
	}

	_, err = c.db.GetConversation(ctx, tips.ConversationID)
	if err != nil {
		log.ZWarn(ctx, "GetConversation err", err, "conversationID", tips.ConversationID)
		return err
	}

	maxReadSeq := tips.HasReadSeq
	if maxReadSeq > 0 {
		// Update cursor and calculate allReadSeq change
		allReadSeqChanged, newAllReadSeq := c.updateReadCursorAndAllReadSeq(ctx, tips.ConversationID, tips.UserID, maxReadSeq)

		// If allReadSeq changed, notify frontend for UI update
		if allReadSeqChanged {
			c.msgListener().OnAllReadSeqChanged(utils.StructToJsonString(map[string]interface{}{
				"conversationID": tips.ConversationID,
				"allReadSeq":     newAllReadSeq,
			}))
		}
	}

	return nil
}
