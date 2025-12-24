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
			maxReadSeq := tips.HasReadSeq
			if maxReadSeq > 0 {
				// Update cursor and calculate minReadSeq change
				minReadSeqChanged, newMinReadSeq := c.updateGroupReadCursorAndMinSeq(ctx, tips.ConversationID, tips.MarkAsReadUserID, maxReadSeq)

				// Notify frontend about group read receipt
				var groupReceiptResp = []*sdk_struct.MessageReceipt{{
					GroupID:     conversation.GroupID,
					UserID:      tips.MarkAsReadUserID,
					MsgIDList:   nil,
					HasReadSeq:  tips.HasReadSeq,
					SessionType: conversation.ConversationType,
					ReadTime:    msg.SendTime,
				}}
				c.msgListener().OnRecvGroupReadReceipt(utils.StructToJsonString(groupReceiptResp))

				// If minReadSeq changed, notify frontend for UI update
				if minReadSeqChanged {
					c.msgListener().OnGroupMinReadSeqChanged(utils.StructToJsonString(map[string]interface{}{
						"conversationID": tips.ConversationID,
						"groupID":        conversation.GroupID,
						"minReadSeq":     newMinReadSeq,
					}))
				}
			} else {
				var groupReceiptResp = []*sdk_struct.MessageReceipt{{
					GroupID:     conversation.GroupID,
					UserID:      tips.MarkAsReadUserID,
					MsgIDList:   nil,
					HasReadSeq:  tips.HasReadSeq,
					SessionType: conversation.ConversationType,
					ReadTime:    msg.SendTime,
				}}
				c.msgListener().OnRecvGroupReadReceipt(utils.StructToJsonString(groupReceiptResp))
			}
		}

	} else {
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

// updateGroupReadCursorAndMinSeq updates the group read cursor and recalculates minReadSeq if needed.
// Returns (minReadSeqChanged, newMinReadSeq).
func (c *Conversation) updateGroupReadCursorAndMinSeq(ctx context.Context, conversationID, userID string, maxReadSeq int64) (bool, int64) {
	// Get current state
	state, stateErr := c.db.GetGroupReadState(ctx, conversationID)

	// Get old cursor to check if this user was the min holder
	oldCursor, cursorErr := c.db.GetGroupReadCursor(ctx, conversationID, userID)
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
		log.ZWarn(ctx, "GetGroupReadCursor err", cursorErr, "conversationID", conversationID, "userID", userID)
		return false, 0
	}

	// Upsert the cursor
	newCursor := &model_struct.LocalGroupReadCursor{
		ConversationID: conversationID,
		UserID:         userID,
		MaxReadSeq:     maxReadSeq,
	}
	if err := c.db.UpsertGroupReadCursor(ctx, newCursor); err != nil {
		log.ZWarn(ctx, "UpsertGroupReadCursor err", err, "conversationID", conversationID, "userID", userID)
		return false, 0
	}

	// Calculate new minReadSeq
	var minReadSeqChanged bool
	var newMinReadSeq int64

	if isRecordNotFoundError(stateErr) || state == nil {
		// First time - need to calculate minReadSeq from scratch
		minSeq, err := c.db.GetMinReadSeqFromCursors(ctx, conversationID)
		if err != nil {
			log.ZWarn(ctx, "GetMinReadSeqFromCursors err", err, "conversationID", conversationID)
			return false, 0
		}
		newMinReadSeq = minSeq
		minReadSeqChanged = true

		// Create new state
		newState := &model_struct.LocalGroupReadState{
			ConversationID: conversationID,
			MinReadSeq:     newMinReadSeq,
		}
		if err := c.db.UpsertGroupReadState(ctx, newState); err != nil {
			log.ZWarn(ctx, "UpsertGroupReadState err", err, "conversationID", conversationID)
		}
	} else if stateErr == nil {
		// State exists - check if minReadSeq needs recalculation
		if isNewCursor {
			// New user added - minReadSeq may decrease
			if maxReadSeq < state.MinReadSeq || state.MinReadSeq == 0 {
				newMinReadSeq = maxReadSeq
				minReadSeqChanged = true
			}
		} else if oldReadSeq == state.MinReadSeq {
			// The updated user was the min holder - need to recalculate
			minSeq, err := c.db.GetMinReadSeqFromCursors(ctx, conversationID)
			if err != nil {
				log.ZWarn(ctx, "GetMinReadSeqFromCursors err", err, "conversationID", conversationID)
				return false, 0
			}
			if minSeq != state.MinReadSeq {
				newMinReadSeq = minSeq
				minReadSeqChanged = true
			}
		}
		// If this user wasn't the min holder and it's not a new cursor, minReadSeq stays the same

		if minReadSeqChanged {
			state.MinReadSeq = newMinReadSeq
			if err := c.db.UpsertGroupReadState(ctx, state); err != nil {
				log.ZWarn(ctx, "UpsertGroupReadState err", err, "conversationID", conversationID)
			}
		}
	}

	return minReadSeqChanged, newMinReadSeq
}

// doGroupReadDrawing handles GroupHasReadReceipt notifications (contentType 2201)
// This is triggered when another group member reads messages and the server broadcasts to all members
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

	conversation, err := c.db.GetConversation(ctx, tips.ConversationID)
	if err != nil {
		log.ZWarn(ctx, "GetConversation err", err, "conversationID", tips.ConversationID)
		return err
	}

	maxReadSeq := tips.HasReadSeq
	if maxReadSeq > 0 {
		// Update cursor and calculate minReadSeq change
		minReadSeqChanged, newMinReadSeq := c.updateGroupReadCursorAndMinSeq(ctx, tips.ConversationID, tips.UserID, maxReadSeq)

		// Notify frontend about group read receipt
		var groupReceiptResp = []*sdk_struct.MessageReceipt{{
			GroupID:     tips.GroupID,
			UserID:      tips.UserID,
			MsgIDList:   nil,
			HasReadSeq:  tips.HasReadSeq,
			SessionType: conversation.ConversationType,
			ReadTime:    msg.SendTime,
		}}
		c.msgListener().OnRecvGroupReadReceipt(utils.StructToJsonString(groupReceiptResp))

		// If minReadSeq changed, notify frontend for UI update
		if minReadSeqChanged {
			c.msgListener().OnGroupMinReadSeqChanged(utils.StructToJsonString(map[string]interface{}{
				"conversationID": tips.ConversationID,
				"groupID":        tips.GroupID,
				"minReadSeq":     newMinReadSeq,
			}))
		}
	} else {
		// Still notify about the read receipt even if hasReadSeq is 0
		var groupReceiptResp = []*sdk_struct.MessageReceipt{{
			GroupID:     tips.GroupID,
			UserID:      tips.UserID,
			MsgIDList:   nil,
			HasReadSeq:  tips.HasReadSeq,
			SessionType: conversation.ConversationType,
			ReadTime:    msg.SendTime,
		}}
		c.msgListener().OnRecvGroupReadReceipt(utils.StructToJsonString(groupReceiptResp))
	}

	return nil
}
