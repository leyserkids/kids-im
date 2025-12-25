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
	"time"

	"github.com/openimsdk/openim-sdk-core/v3/pkg/common"
	"github.com/openimsdk/openim-sdk-core/v3/pkg/constant"
	"github.com/openimsdk/openim-sdk-core/v3/pkg/db/model_struct"
	"github.com/openimsdk/openim-sdk-core/v3/pkg/utils"
	"github.com/openimsdk/protocol/msg"
	"github.com/openimsdk/tools/utils/datautil"

	"github.com/openimsdk/tools/log"
)

func (c *Conversation) SyncAllConversationHashReadSeqs(ctx context.Context) error {
	startTime := time.Now()
	log.ZDebug(ctx, "start SyncConversationHashReadSeqs")

	resp := msg.GetConversationsHasReadAndMaxSeqResp{}
	req := msg.GetConversationsHasReadAndMaxSeqReq{UserID: c.loginUserID}
	err := c.SendReqWaitResp(ctx, &req, constant.GetConvMaxReadSeq, &resp)
	if err != nil {
		log.ZWarn(ctx, "SendReqWaitResp err", err)
		return err
	}
	seqs := resp.Seqs
	log.ZDebug(ctx, "getServerHasReadAndMaxSeqs completed", "duration", time.Since(startTime).Seconds())

	if len(seqs) == 0 {
		return nil
	}
	var conversationChangedIDs []string
	var conversationIDsNeedSync []string

	stepStartTime := time.Now()
	conversationsOnLocal, err := c.db.GetAllConversations(ctx)
	if err != nil {
		log.ZWarn(ctx, "get all conversations err", err)
		return err
	}
	log.ZDebug(ctx, "GetAllConversations completed", "duration", time.Since(stepStartTime).Seconds())

	conversationsOnLocalMap := datautil.SliceToMap(conversationsOnLocal, func(e *model_struct.LocalConversation) string {
		return e.ConversationID
	})

	stepStartTime = time.Now()
	for conversationID, v := range seqs {
		var unreadCount int32
		c.maxSeqRecorder.Set(conversationID, v.MaxSeq)
		if v.MaxSeq-v.HasReadSeq < 0 {
			unreadCount = 0
			log.ZWarn(ctx, "unread count is less than 0", nil, "conversationID",
				conversationID, "maxSeq", v.MaxSeq, "hasReadSeq", v.HasReadSeq)
		} else {
			unreadCount = int32(v.MaxSeq - v.HasReadSeq)
		}
		if conversation, ok := conversationsOnLocalMap[conversationID]; ok {
			if conversation.UnreadCount != unreadCount {
				if err := c.db.UpdateColumnsConversation(ctx, conversationID, map[string]interface{}{"unread_count": unreadCount}); err != nil {
					log.ZWarn(ctx, "UpdateColumnsConversation err", err, "conversationID", conversationID)
					continue
				}
				conversationChangedIDs = append(conversationChangedIDs, conversationID)
			}
		} else {
			conversationIDsNeedSync = append(conversationIDsNeedSync, conversationID)
		}
	}
	log.ZDebug(ctx, "Process seqs completed", "duration", time.Since(stepStartTime).Seconds())

	if len(conversationIDsNeedSync) > 0 {
		stepStartTime = time.Now()
		r, err := c.getConversationsByIDsFromServer(ctx, conversationIDsNeedSync)
		if err != nil {
			log.ZWarn(ctx, "getServerConversationsByIDs err", err, "conversationIDs", conversationIDsNeedSync)
			return err
		}
		log.ZDebug(ctx, "getServerConversationsByIDs completed", "duration", time.Since(stepStartTime).Seconds())
		conversationsOnServer := datautil.Batch(ServerConversationToLocal, r.Conversations)
		stepStartTime = time.Now()
		if err := c.batchAddFaceURLAndName(ctx, conversationsOnServer...); err != nil {
			log.ZWarn(ctx, "batchAddFaceURLAndName err", err, "conversationsOnServer", conversationsOnServer)
			return err
		}
		log.ZDebug(ctx, "batchAddFaceURLAndName completed", "duration", time.Since(stepStartTime).Seconds())

		for _, conversation := range conversationsOnServer {
			var unreadCount int32
			v, ok := seqs[conversation.ConversationID]
			if !ok {
				continue
			}
			if v.MaxSeq-v.HasReadSeq < 0 {
				unreadCount = 0
				log.ZWarn(ctx, "unread count is less than 0", nil, "server seq", v, "conversation", conversation)
			} else {
				unreadCount = int32(v.MaxSeq - v.HasReadSeq)
			}
			conversation.UnreadCount = unreadCount
		}

		stepStartTime = time.Now()
		err = c.db.BatchInsertConversationList(ctx, conversationsOnServer)
		if err != nil {
			log.ZWarn(ctx, "BatchInsertConversationList err", err, "conversationsOnServer", conversationsOnServer)
		}
		log.ZDebug(ctx, "BatchInsertConversationList completed", "duration", time.Since(stepStartTime).Seconds())
	}

	log.ZDebug(ctx, "update conversations", "conversations", conversationChangedIDs)
	if len(conversationChangedIDs) > 0 {
		stepStartTime = time.Now()
		common.TriggerCmdUpdateConversation(ctx, common.UpdateConNode{Action: constant.ConChange, Args: conversationChangedIDs}, c.GetCh())
		common.TriggerCmdUpdateConversation(ctx, common.UpdateConNode{Action: constant.TotalUnreadMessageChanged}, c.GetCh())
		log.ZDebug(ctx, "TriggerCmdUpdateConversation completed", "duration", time.Since(stepStartTime).Seconds())
	}

	log.ZDebug(ctx, "SyncAllConversationHashReadSeqs completed", "totalDuration", time.Since(startTime).Seconds())
	return nil
}

// SyncReadCursors syncs read cursors for the specified conversations
func (c *Conversation) SyncReadCursors(ctx context.Context, conversationIDs []string) error {
	if len(conversationIDs) == 0 {
		return nil
	}
	startTime := time.Now()
	log.ZDebug(ctx, "start SyncReadCursors", "conversationIDs", conversationIDs)

	resp, err := c.getConversationReadCursorsFromServer(ctx, conversationIDs)
	if err != nil {
		log.ZWarn(ctx, "getConversationReadCursorsFromServer err", err, "conversationIDs", conversationIDs)
		return err
	}

	for _, convCursors := range resp.ConversationReadCursors {
		conversationID := convCursors.ConversationID
		var hasChanges bool

		for _, cursor := range convCursors.Cursors {
			localCursor := &model_struct.LocalReadCursor{
				ConversationID: conversationID,
				UserID:         cursor.UserID,
				MaxReadSeq:     cursor.MaxReadSeq,
			}
			// Try to get existing cursor first
			existingCursor, err := c.db.GetReadCursor(ctx, conversationID, cursor.UserID)
			if err != nil {
				// If not found, insert new cursor
				if err := c.db.InsertReadCursor(ctx, localCursor); err != nil {
					log.ZWarn(ctx, "InsertReadCursor err", err, "cursor", localCursor)
				} else {
					hasChanges = true
				}
			} else {
				// If found and new seq is greater, update it
				if cursor.MaxReadSeq > existingCursor.MaxReadSeq {
					if err := c.db.UpdateReadCursor(ctx, conversationID, cursor.UserID, cursor.MaxReadSeq); err != nil {
						log.ZWarn(ctx, "UpdateReadCursor err", err, "cursor", localCursor)
					} else {
						hasChanges = true
					}
				}
			}
		}

		// If any cursor changed, recalculate and update AllReadSeq
		if hasChanges {
			c.updateAllReadSeqAfterSync(ctx, conversationID)
		}
	}

	log.ZDebug(ctx, "SyncReadCursors completed", "totalDuration", time.Since(startTime).Seconds())
	return nil
}

// updateAllReadSeqAfterSync recalculates AllReadSeq after syncing cursors and triggers callback if changed
func (c *Conversation) updateAllReadSeqAfterSync(ctx context.Context, conversationID string) {
	// Calculate new AllReadSeq from all cursors
	newAllReadSeq, err := c.db.GetAllReadSeqFromCursors(ctx, conversationID)
	if err != nil {
		log.ZWarn(ctx, "GetAllReadSeqFromCursors err", err, "conversationID", conversationID)
		return
	}

	// Get current state
	state, stateErr := c.db.GetReadState(ctx, conversationID)

	var oldAllReadSeq int64
	if stateErr == nil && state != nil {
		oldAllReadSeq = state.AllReadSeq
	}

	// Update state if AllReadSeq changed
	if newAllReadSeq != oldAllReadSeq {
		newState := &model_struct.LocalReadState{
			ConversationID: conversationID,
			AllReadSeq:     newAllReadSeq,
		}
		if err := c.db.UpsertReadState(ctx, newState); err != nil {
			log.ZWarn(ctx, "UpsertReadState err", err, "conversationID", conversationID)
			return
		}

		// Trigger callback to notify frontend
		c.msgListener().OnAllReadSeqChanged(utils.StructToJsonString(map[string]interface{}{
			"conversationID": conversationID,
			"allReadSeq":     newAllReadSeq,
		}))
		log.ZDebug(ctx, "AllReadSeq changed after sync", "conversationID", conversationID,
			"oldAllReadSeq", oldAllReadSeq, "newAllReadSeq", newAllReadSeq)
	}
}

// getConversationIDsForReadCursorSync returns conversation IDs for ReadCursor sync:
// - All single chat conversations
// - Top N most recent (by latest_msg_send_time) group chat conversations
func (c *Conversation) getConversationIDsForReadCursorSync(ctx context.Context, groupLimit int) ([]string, error) {
	// Get all single chat conversations
	allConversations, err := c.db.GetAllConversations(ctx)
	if err != nil {
		return nil, err
	}

	var conversationIDs []string
	for _, conv := range allConversations {
		if conv.ConversationType == constant.SingleChatType {
			conversationIDs = append(conversationIDs, conv.ConversationID)
		}
	}

	// Get recent group conversations (ordered by latest_msg_send_time desc)
	recentConversations, err := c.db.GetConversationListSplitDB(ctx, 0, groupLimit*3)
	if err != nil {
		return nil, err
	}

	var groupCount int
	for _, conv := range recentConversations {
		if conv.ConversationType == constant.ReadGroupChatType {
			conversationIDs = append(conversationIDs, conv.ConversationID)
			groupCount++
			if groupCount >= groupLimit {
				break
			}
		}
	}

	return conversationIDs, nil
}

// syncRecentReadCursors syncs read cursors for all single chats and recent group chats
func (c *Conversation) syncRecentReadCursors(ctx context.Context) {
	conversationIDs, err := c.getConversationIDsForReadCursorSync(ctx, 10)
	if err != nil {
		log.ZWarn(ctx, "getConversationIDsForReadCursorSync err", err)
		return
	}
	if len(conversationIDs) == 0 {
		log.ZDebug(ctx, "No conversations to sync ReadCursors")
		return
	}
	log.ZDebug(ctx, "syncRecentReadCursors", "count", len(conversationIDs))
	if err := c.SyncReadCursors(ctx, conversationIDs); err != nil {
		log.ZWarn(ctx, "SyncReadCursors err", err, "conversationIDs", conversationIDs)
	}
}
