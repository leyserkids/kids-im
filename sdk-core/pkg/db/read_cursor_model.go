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

//go:build !js
// +build !js

package db

import (
	"context"

	"github.com/openimsdk/openim-sdk-core/v3/pkg/db/model_struct"
	"github.com/openimsdk/tools/errs"
)

// ========== LocalReadCursor operations ==========

// InsertReadCursor inserts a new read cursor
func (d *DataBase) InsertReadCursor(ctx context.Context, cursor *model_struct.LocalReadCursor) error {
	d.mRWMutex.Lock()
	defer d.mRWMutex.Unlock()
	return errs.WrapMsg(d.conn.WithContext(ctx).Create(cursor).Error, "InsertReadCursor failed")
}

// UpsertReadCursor inserts or updates a read cursor
func (d *DataBase) UpsertReadCursor(ctx context.Context, cursor *model_struct.LocalReadCursor) error {
	d.mRWMutex.Lock()
	defer d.mRWMutex.Unlock()

	result := d.conn.WithContext(ctx).Model(&model_struct.LocalReadCursor{}).
		Where("conversation_id = ? AND user_id = ?", cursor.ConversationID, cursor.UserID).
		Update("max_read_seq", cursor.MaxReadSeq)

	if result.Error != nil {
		return errs.WrapMsg(result.Error, "UpsertReadCursor update failed")
	}

	if result.RowsAffected == 0 {
		return errs.WrapMsg(d.conn.WithContext(ctx).Create(cursor).Error, "UpsertReadCursor insert failed")
	}

	return nil
}

// UpdateReadCursor updates the max read seq for a specific user in a conversation
func (d *DataBase) UpdateReadCursor(ctx context.Context, conversationID, userID string, maxReadSeq int64) error {
	d.mRWMutex.Lock()
	defer d.mRWMutex.Unlock()
	return errs.WrapMsg(d.conn.WithContext(ctx).Model(&model_struct.LocalReadCursor{}).
		Where("conversation_id = ? AND user_id = ?", conversationID, userID).
		Update("max_read_seq", maxReadSeq).Error, "UpdateReadCursor failed")
}

// GetReadCursor gets the read cursor for a specific user in a conversation
func (d *DataBase) GetReadCursor(ctx context.Context, conversationID, userID string) (*model_struct.LocalReadCursor, error) {
	d.mRWMutex.RLock()
	defer d.mRWMutex.RUnlock()
	var cursor model_struct.LocalReadCursor
	err := d.conn.WithContext(ctx).Where("conversation_id = ? AND user_id = ?", conversationID, userID).First(&cursor).Error
	if err != nil {
		return nil, err
	}
	return &cursor, nil
}

// GetReadCursorsByConversationID gets all read cursors for a conversation
func (d *DataBase) GetReadCursorsByConversationID(ctx context.Context, conversationID string) ([]*model_struct.LocalReadCursor, error) {
	d.mRWMutex.RLock()
	defer d.mRWMutex.RUnlock()
	var cursors []*model_struct.LocalReadCursor
	return cursors, errs.WrapMsg(d.conn.WithContext(ctx).Where("conversation_id = ?", conversationID).Find(&cursors).Error, "GetReadCursorsByConversationID failed")
}

// GetAllReadSeqFromCursors gets the minimum read seq from all cursors in a conversation
// This represents the position that ALL members have read up to
// excludeUserID is the current logged-in user, whose cursor should be excluded from the calculation
func (d *DataBase) GetAllReadSeqFromCursors(ctx context.Context, conversationID string, excludeUserID string) (int64, error) {
	d.mRWMutex.RLock()
	defer d.mRWMutex.RUnlock()
	var allReadSeq int64
	err := d.conn.WithContext(ctx).Model(&model_struct.LocalReadCursor{}).
		Where("conversation_id = ? AND user_id != ?", conversationID, excludeUserID).
		Select("COALESCE(MIN(max_read_seq), 0)").
		Scan(&allReadSeq).Error
	return allReadSeq, errs.WrapMsg(err, "GetAllReadSeqFromCursors failed")
}

// DeleteReadCursor deletes the read cursor for a specific user in a conversation
func (d *DataBase) DeleteReadCursor(ctx context.Context, conversationID, userID string) error {
	d.mRWMutex.Lock()
	defer d.mRWMutex.Unlock()
	return errs.WrapMsg(d.conn.WithContext(ctx).Where("conversation_id = ? AND user_id = ?", conversationID, userID).
		Delete(&model_struct.LocalReadCursor{}).Error, "DeleteReadCursor failed")
}

// DeleteReadCursorsByConversationID deletes all read cursors for a conversation
func (d *DataBase) DeleteReadCursorsByConversationID(ctx context.Context, conversationID string) error {
	d.mRWMutex.Lock()
	defer d.mRWMutex.Unlock()
	return errs.WrapMsg(d.conn.WithContext(ctx).Where("conversation_id = ?", conversationID).
		Delete(&model_struct.LocalReadCursor{}).Error, "DeleteReadCursorsByConversationID failed")
}

// ========== LocalReadState operations ==========

// GetReadState gets the read state for a conversation
func (d *DataBase) GetReadState(ctx context.Context, conversationID string) (*model_struct.LocalReadState, error) {
	d.mRWMutex.RLock()
	defer d.mRWMutex.RUnlock()
	var state model_struct.LocalReadState
	err := d.conn.WithContext(ctx).Where("conversation_id = ?", conversationID).First(&state).Error
	if err != nil {
		return nil, err
	}
	return &state, nil
}

// UpsertReadState inserts or updates the read state
func (d *DataBase) UpsertReadState(ctx context.Context, state *model_struct.LocalReadState) error {
	d.mRWMutex.Lock()
	defer d.mRWMutex.Unlock()

	result := d.conn.WithContext(ctx).Model(&model_struct.LocalReadState{}).
		Where("conversation_id = ?", state.ConversationID).
		Updates(map[string]interface{}{
			"all_read_seq": state.AllReadSeq,
		})

	if result.Error != nil {
		return errs.WrapMsg(result.Error, "UpsertReadState update failed")
	}

	if result.RowsAffected == 0 {
		return errs.WrapMsg(d.conn.WithContext(ctx).Create(state).Error, "UpsertReadState insert failed")
	}

	return nil
}

// UpdateReadStateAllReadSeq updates only the allReadSeq
func (d *DataBase) UpdateReadStateAllReadSeq(ctx context.Context, conversationID string, allReadSeq int64) error {
	d.mRWMutex.Lock()
	defer d.mRWMutex.Unlock()

	return errs.WrapMsg(d.conn.WithContext(ctx).Model(&model_struct.LocalReadState{}).
		Where("conversation_id = ?", conversationID).
		Updates(map[string]interface{}{
			"all_read_seq": allReadSeq,
		}).Error, "UpdateReadStateAllReadSeq failed")
}

// DeleteReadState deletes the read state for a conversation
func (d *DataBase) DeleteReadState(ctx context.Context, conversationID string) error {
	d.mRWMutex.Lock()
	defer d.mRWMutex.Unlock()
	return errs.WrapMsg(d.conn.WithContext(ctx).Where("conversation_id = ?", conversationID).
		Delete(&model_struct.LocalReadState{}).Error, "DeleteReadState failed")
}
