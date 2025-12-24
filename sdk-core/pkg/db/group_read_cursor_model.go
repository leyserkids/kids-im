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

// InsertGroupReadCursor inserts a new group read cursor
func (d *DataBase) InsertGroupReadCursor(ctx context.Context, cursor *model_struct.LocalGroupReadCursor) error {
	d.mRWMutex.Lock()
	defer d.mRWMutex.Unlock()
	return errs.WrapMsg(d.conn.WithContext(ctx).Create(cursor).Error, "InsertGroupReadCursor failed")
}

// UpsertGroupReadCursor inserts or updates a group read cursor
func (d *DataBase) UpsertGroupReadCursor(ctx context.Context, cursor *model_struct.LocalGroupReadCursor) error {
	d.mRWMutex.Lock()
	defer d.mRWMutex.Unlock()

	result := d.conn.WithContext(ctx).Model(&model_struct.LocalGroupReadCursor{}).
		Where("conversation_id = ? AND user_id = ?", cursor.ConversationID, cursor.UserID).
		Update("max_read_seq", cursor.MaxReadSeq)

	if result.Error != nil {
		return errs.WrapMsg(result.Error, "UpsertGroupReadCursor update failed")
	}

	if result.RowsAffected == 0 {
		return errs.WrapMsg(d.conn.WithContext(ctx).Create(cursor).Error, "UpsertGroupReadCursor insert failed")
	}

	return nil
}

// UpdateGroupReadCursor updates the max read seq for a specific user in a conversation
func (d *DataBase) UpdateGroupReadCursor(ctx context.Context, conversationID, userID string, maxReadSeq int64) error {
	d.mRWMutex.Lock()
	defer d.mRWMutex.Unlock()
	return errs.WrapMsg(d.conn.WithContext(ctx).Model(&model_struct.LocalGroupReadCursor{}).
		Where("conversation_id = ? AND user_id = ?", conversationID, userID).
		Update("max_read_seq", maxReadSeq).Error, "UpdateGroupReadCursor failed")
}

// GetGroupReadCursor gets the read cursor for a specific user in a conversation
func (d *DataBase) GetGroupReadCursor(ctx context.Context, conversationID, userID string) (*model_struct.LocalGroupReadCursor, error) {
	d.mRWMutex.RLock()
	defer d.mRWMutex.RUnlock()
	var cursor model_struct.LocalGroupReadCursor
	err := d.conn.WithContext(ctx).Where("conversation_id = ? AND user_id = ?", conversationID, userID).First(&cursor).Error
	if err != nil {
		return nil, err
	}
	return &cursor, nil
}

// GetGroupReadCursorsByConversationID gets all read cursors for a conversation
func (d *DataBase) GetGroupReadCursorsByConversationID(ctx context.Context, conversationID string) ([]*model_struct.LocalGroupReadCursor, error) {
	d.mRWMutex.RLock()
	defer d.mRWMutex.RUnlock()
	var cursors []*model_struct.LocalGroupReadCursor
	return cursors, errs.WrapMsg(d.conn.WithContext(ctx).Where("conversation_id = ?", conversationID).Find(&cursors).Error, "GetGroupReadCursorsByConversationID failed")
}

// GetMinReadSeqFromCursors gets the minimum read seq from all cursors in a conversation
func (d *DataBase) GetMinReadSeqFromCursors(ctx context.Context, conversationID string) (int64, error) {
	d.mRWMutex.RLock()
	defer d.mRWMutex.RUnlock()
	var minSeq int64
	err := d.conn.WithContext(ctx).Model(&model_struct.LocalGroupReadCursor{}).
		Where("conversation_id = ?", conversationID).
		Select("COALESCE(MIN(max_read_seq), 0)").
		Scan(&minSeq).Error
	return minSeq, errs.WrapMsg(err, "GetMinReadSeqFromCursors failed")
}

// DeleteGroupReadCursor deletes the read cursor for a specific user in a conversation
func (d *DataBase) DeleteGroupReadCursor(ctx context.Context, conversationID, userID string) error {
	d.mRWMutex.Lock()
	defer d.mRWMutex.Unlock()
	return errs.WrapMsg(d.conn.WithContext(ctx).Where("conversation_id = ? AND user_id = ?", conversationID, userID).
		Delete(&model_struct.LocalGroupReadCursor{}).Error, "DeleteGroupReadCursor failed")
}

// DeleteGroupReadCursorsByConversationID deletes all read cursors for a conversation
func (d *DataBase) DeleteGroupReadCursorsByConversationID(ctx context.Context, conversationID string) error {
	d.mRWMutex.Lock()
	defer d.mRWMutex.Unlock()
	return errs.WrapMsg(d.conn.WithContext(ctx).Where("conversation_id = ?", conversationID).
		Delete(&model_struct.LocalGroupReadCursor{}).Error, "DeleteGroupReadCursorsByConversationID failed")
}

// ========== LocalGroupReadState operations ==========

// GetGroupReadState gets the group read state for a conversation
func (d *DataBase) GetGroupReadState(ctx context.Context, conversationID string) (*model_struct.LocalGroupReadState, error) {
	d.mRWMutex.RLock()
	defer d.mRWMutex.RUnlock()
	var state model_struct.LocalGroupReadState
	err := d.conn.WithContext(ctx).Where("conversation_id = ?", conversationID).First(&state).Error
	if err != nil {
		return nil, err
	}
	return &state, nil
}

// UpsertGroupReadState inserts or updates the group read state
func (d *DataBase) UpsertGroupReadState(ctx context.Context, state *model_struct.LocalGroupReadState) error {
	d.mRWMutex.Lock()
	defer d.mRWMutex.Unlock()

	result := d.conn.WithContext(ctx).Model(&model_struct.LocalGroupReadState{}).
		Where("conversation_id = ?", state.ConversationID).
		Updates(map[string]interface{}{
			"min_read_seq": state.MinReadSeq,
		})

	if result.Error != nil {
		return errs.WrapMsg(result.Error, "UpsertGroupReadState update failed")
	}

	if result.RowsAffected == 0 {
		return errs.WrapMsg(d.conn.WithContext(ctx).Create(state).Error, "UpsertGroupReadState insert failed")
	}

	return nil
}

// UpdateGroupReadStateMinSeq updates only the minReadSeq
func (d *DataBase) UpdateGroupReadStateMinSeq(ctx context.Context, conversationID string, minReadSeq int64) error {
	d.mRWMutex.Lock()
	defer d.mRWMutex.Unlock()

	return errs.WrapMsg(d.conn.WithContext(ctx).Model(&model_struct.LocalGroupReadState{}).
		Where("conversation_id = ?", conversationID).
		Updates(map[string]interface{}{
			"min_read_seq": minReadSeq,
		}).Error, "UpdateGroupReadStateMinSeq failed")
}

// DeleteGroupReadState deletes the group read state for a conversation
func (d *DataBase) DeleteGroupReadState(ctx context.Context, conversationID string) error {
	d.mRWMutex.Lock()
	defer d.mRWMutex.Unlock()
	return errs.WrapMsg(d.conn.WithContext(ctx).Where("conversation_id = ?", conversationID).
		Delete(&model_struct.LocalGroupReadState{}).Error, "DeleteGroupReadState failed")
}
