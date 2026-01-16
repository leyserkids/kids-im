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
	"github.com/openimsdk/protocol/sdkws"
	"github.com/openimsdk/tools/errs"
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
			// Skip current login user's cursor - we don't need to track our own read position
			if cursor.UserID == c.loginUserID {
				continue
			}

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

		// If any cursor changed, recalculate and update ReadState
		if hasChanges {
			c.updateReadStateAfterSync(ctx, conversationID)
		}
	}

	log.ZDebug(ctx, "SyncReadCursors completed", "totalDuration", time.Since(startTime).Seconds())
	return nil
}

// updateReadStateAfterSync recalculates ReadState after syncing cursors and triggers callback if changed
func (c *Conversation) updateReadStateAfterSync(ctx context.Context, conversationID string) {
	// Calculate new AllReadSeq from all cursors, excluding self
	// The allReadSeq represents the minimum read position of OTHER members
	newAllReadSeq, err := c.db.GetAllReadSeqFromCursors(ctx, conversationID, c.loginUserID)
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

		// Trigger callback for subscribed conversation
		c.checkAndNotifyReadStateChanged(ctx, conversationID)
		log.ZDebug(ctx, "ReadState changed after sync", "conversationID", conversationID,
			"oldAllReadSeq", oldAllReadSeq, "newAllReadSeq", newAllReadSeq)
	}
}

// syncAllReadCursors syncs read cursors for all conversations (single chat + group chat)
// Called on connection/reconnection to ensure read state is up to date
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

	// Notify all subscribed conversations about their ReadState
	c.notifySubscribedConversationsReadStateChanged(ctx)
}

// notifyConversationReadStateChanged notifies a single conversation's ReadState change
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

// notifySubscribedConversationsReadStateChanged notifies all subscribed conversations about their ReadState
// Called after reconnection sync
func (c *Conversation) notifySubscribedConversationsReadStateChanged(ctx context.Context) {
	for _, convID := range c.getSubscribedConversations() {
		c.notifyConversationReadStateChanged(ctx, convID)
	}
}

// checkAndNotifyReadStateChanged checks if the conversation is subscribed and triggers callback if so
func (c *Conversation) checkAndNotifyReadStateChanged(ctx context.Context, conversationID string) {
	if !c.isConversationSubscribed(conversationID) {
		return // Not subscribed, no callback
	}
	c.notifyConversationReadStateChanged(ctx, conversationID)
}

// handleGroupMemberChangeForReadCursor handles ReadCursor updates when group members change
func (c *Conversation) handleGroupMemberChangeForReadCursor(ctx context.Context, msg *sdkws.MsgData) {
	go func() {
		switch msg.ContentType {
		// Note: GroupCreatedNotification (1501) is handled by the unified new conversation sync
		// in syncer.WithNotice and doUpdateConversation, no need to handle here
		case constant.MemberQuitNotification: // 1504
			c.handleMemberQuitForReadCursor(ctx, msg)
		case constant.MemberKickedNotification: // 1508
			c.handleMemberKickedForReadCursor(ctx, msg)
		case constant.MemberInvitedNotification: // 1509
			c.handleMemberInvitedForReadCursor(ctx, msg)
		case constant.MemberEnterNotification: // 1510
			c.handleMemberEnterForReadCursor(ctx, msg)
			// Note: GroupDismissedNotification (1511) cleanup is handled when user deletes the conversation
		}
	}()
}

// handleMemberQuitForReadCursor - member quit: delete cursor, recalculate allReadSeq
func (c *Conversation) handleMemberQuitForReadCursor(ctx context.Context, msg *sdkws.MsgData) {
	var detail sdkws.MemberQuitTips
	if err := utils.UnmarshalNotificationElem(msg.Content, &detail); err != nil {
		log.ZWarn(ctx, "UnmarshalNotificationElem err", err)
		return
	}

	// Skip if it's the current user quitting
	if detail.QuitUser.UserID == c.loginUserID {
		return
	}

	conversationID := c.getConversationIDBySessionType(detail.Group.GroupID, constant.ReadGroupChatType)
	c.handleMemberLeftForReadCursor(ctx, conversationID, []string{detail.QuitUser.UserID})
}

// handleMemberKickedForReadCursor - members kicked: delete cursors, recalculate allReadSeq
func (c *Conversation) handleMemberKickedForReadCursor(ctx context.Context, msg *sdkws.MsgData) {
	var detail sdkws.MemberKickedTips
	if err := utils.UnmarshalNotificationElem(msg.Content, &detail); err != nil {
		log.ZWarn(ctx, "UnmarshalNotificationElem err", err)
		return
	}

	var userIDs []string
	for _, member := range detail.KickedUserList {
		// Skip current user
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

// handleMemberLeftForReadCursor - member left: delete cursor, recalculate allReadSeq (may increase)
func (c *Conversation) handleMemberLeftForReadCursor(ctx context.Context, conversationID string, userIDs []string) {
	for _, userID := range userIDs {
		if err := c.db.DeleteReadCursor(ctx, conversationID, userID); err != nil {
			log.ZWarn(ctx, "DeleteReadCursor failed", err, "conversationID", conversationID, "userID", userID)
		}
	}

	// Recalculate allReadSeq and notify
	c.updateReadStateAfterSync(ctx, conversationID)
}

// handleMemberInvitedForReadCursor - members invited: create cursors (maxReadSeq=0)
func (c *Conversation) handleMemberInvitedForReadCursor(ctx context.Context, msg *sdkws.MsgData) {
	var detail sdkws.MemberInvitedTips
	if err := utils.UnmarshalNotificationElem(msg.Content, &detail); err != nil {
		log.ZWarn(ctx, "UnmarshalNotificationElem err", err)
		return
	}

	var userIDs []string
	for _, member := range detail.InvitedUserList {
		// Skip current user
		if member.UserID != c.loginUserID {
			userIDs = append(userIDs, member.UserID)
		}
	}

	if len(userIDs) == 0 {
		return
	}

	conversationID := c.getConversationIDBySessionType(detail.Group.GroupID, constant.ReadGroupChatType)
	c.handleMemberEnterForReadCursorInternal(ctx, conversationID, userIDs)
}

// handleMemberEnterForReadCursor - member entered: create cursor (maxReadSeq=0)
func (c *Conversation) handleMemberEnterForReadCursor(ctx context.Context, msg *sdkws.MsgData) {
	var detail sdkws.MemberEnterTips
	if err := utils.UnmarshalNotificationElem(msg.Content, &detail); err != nil {
		log.ZWarn(ctx, "UnmarshalNotificationElem err", err)
		return
	}

	// Skip current user
	if detail.EntrantUser.UserID == c.loginUserID {
		return
	}

	conversationID := c.getConversationIDBySessionType(detail.Group.GroupID, constant.ReadGroupChatType)
	c.handleMemberEnterForReadCursorInternal(ctx, conversationID, []string{detail.EntrantUser.UserID})
}

// handleMemberEnterForReadCursorInternal - new members joined: sync cursors from server
// Don't create local cursors with maxReadSeq=0, instead sync from server to get actual read positions
func (c *Conversation) handleMemberEnterForReadCursorInternal(ctx context.Context, conversationID string, userIDs []string) {
	// Sync from server to get actual cursor values
	// This handles both new members (server returns their actual position) and
	// rejoining members (server may have their previous read position)
	if err := c.SyncReadCursors(ctx, []string{conversationID}); err != nil {
		log.ZWarn(ctx, "SyncReadCursors failed after member enter", err, "conversationID", conversationID, "userIDs", userIDs)
	}
}

// ensureReadCursorsForConversation ensures the conversation has complete ReadCursor data.
// Checks whether all other members (excluding self) have cursor data.
// If any member is missing, syncs from server.
func (c *Conversation) ensureReadCursorsForConversation(ctx context.Context, conversationID string) error {
	log.ZDebug(ctx, "ensureReadCursorsForConversation", "conversationID", conversationID)

	conv, err := c.db.GetConversation(ctx, conversationID)
	if err != nil {
		return c.handleConversationNotFound(ctx, conversationID, err)
	}

	expectedMembers, err := c.getExpectedMembersForReadCursor(ctx, conv)
	if err != nil {
		return err
	}
	if len(expectedMembers) == 0 {
		return nil
	}

	missingMembers, err := c.findMissingCursorMembers(ctx, conversationID, expectedMembers)
	if err != nil {
		return err
	}
	if len(missingMembers) == 0 {
		log.ZDebug(ctx, "ensureReadCursorsForConversation: all members have cursor",
			"conversationID", conversationID, "memberCount", len(expectedMembers))
		return nil
	}

	log.ZInfo(ctx, "ensureReadCursorsForConversation: missing cursors, syncing",
		"conversationID", conversationID, "missingMembers", missingMembers)
	return c.SyncReadCursors(ctx, []string{conversationID})
}

// handleConversationNotFound handles the case when conversation is not found locally.
// If RecordNotFound, schedules a delayed sync; otherwise returns the error.
func (c *Conversation) handleConversationNotFound(ctx context.Context, conversationID string, err error) error {
	if errs.Unwrap(err) == errs.ErrRecordNotFound {
		log.ZDebug(ctx, "ensureReadCursorsForConversation: conversation not found, scheduling delayed sync",
			"conversationID", conversationID)
		go func() {
			time.Sleep(time.Second)
			if err := c.SyncReadCursors(ctx, []string{conversationID}); err != nil {
				log.ZWarn(ctx, "ensureReadCursorsForConversation: delayed sync failed", err,
					"conversationID", conversationID)
			}
		}()
		return nil
	}
	log.ZError(ctx, "ensureReadCursorsForConversation: GetConversation failed", err,
		"conversationID", conversationID)
	return err
}

// getExpectedMembersForReadCursor returns the list of members who should have ReadCursor data.
// For single chat: the other party. For group chat: all members except self.
func (c *Conversation) getExpectedMembersForReadCursor(ctx context.Context, conv *model_struct.LocalConversation) ([]string, error) {
	switch conv.ConversationType {
	case constant.SingleChatType:
		return []string{conv.UserID}, nil

	case constant.ReadGroupChatType:
		members, err := c.db.GetGroupMemberListByGroupID(ctx, conv.GroupID)
		if err != nil {
			log.ZError(ctx, "getExpectedMembersForReadCursor: GetGroupMemberListByGroupID failed", err,
				"groupID", conv.GroupID)
			return nil, err
		}
		result := make([]string, 0, len(members))
		for _, m := range members {
			if m.UserID != c.loginUserID {
				result = append(result, m.UserID)
			}
		}
		return result, nil

	default:
		return nil, nil
	}
}

// findMissingCursorMembers returns members who don't have cursor data locally.
func (c *Conversation) findMissingCursorMembers(ctx context.Context, conversationID string, expectedMembers []string) ([]string, error) {
	cursors, err := c.db.GetReadCursorsByConversationID(ctx, conversationID)
	if err != nil {
		log.ZError(ctx, "findMissingCursorMembers: GetReadCursorsByConversationID failed", err,
			"conversationID", conversationID)
		return nil, err
	}

	existingSet := make(map[string]struct{}, len(cursors))
	for _, cursor := range cursors {
		existingSet[cursor.UserID] = struct{}{}
	}

	var missing []string
	for _, memberID := range expectedMembers {
		if _, exists := existingSet[memberID]; !exists {
			missing = append(missing, memberID)
		}
	}
	return missing, nil
}

// cleanupReadCursorsForDeletedConversation cleans up ReadCursor and ReadState when a conversation is deleted
func (c *Conversation) cleanupReadCursorsForDeletedConversation(ctx context.Context, conversationID string) {
	log.ZDebug(ctx, "cleanupReadCursorsForDeletedConversation", "conversationID", conversationID)

	if err := c.db.DeleteReadCursorsByConversationID(ctx, conversationID); err != nil {
		log.ZWarn(ctx, "DeleteReadCursorsByConversationID failed", err, "conversationID", conversationID)
	}

	if err := c.db.DeleteReadState(ctx, conversationID); err != nil {
		log.ZWarn(ctx, "DeleteReadState failed", err, "conversationID", conversationID)
	}

	// Clean up subscription state for this conversation.
	// This is not strictly necessary for functionality (the callback won't fire anyway
	// since ReadState data is deleted), but keeps the in-memory subscription map clean.
	c.subscribedConversationsMu.Lock()
	delete(c.subscribedConversations, conversationID)
	c.subscribedConversationsMu.Unlock()
}
