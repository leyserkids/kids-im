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
	"gorm.io/gorm/clause"
)

// GetGroupReadCursors gets all read cursors for a conversation
func (d *DataBase) GetGroupReadCursors(ctx context.Context, conversationID string) ([]*model_struct.LocalGroupReadCursor, error) {
	d.mRWMutex.RLock()
	defer d.mRWMutex.RUnlock()
	var cursors []*model_struct.LocalGroupReadCursor
	return cursors, errs.WrapMsg(d.conn.WithContext(ctx).Where("conversation_id = ?", conversationID).Find(&cursors).Error, "GetGroupReadCursors failed")
}

// GetGroupReadCursor gets the read cursor for a specific user in a conversation
func (d *DataBase) GetGroupReadCursor(ctx context.Context, conversationID, userID string) (*model_struct.LocalGroupReadCursor, error) {
	d.mRWMutex.RLock()
	defer d.mRWMutex.RUnlock()
	var cursor model_struct.LocalGroupReadCursor
	return &cursor, errs.WrapMsg(d.conn.WithContext(ctx).Where("conversation_id = ? AND user_id = ?", conversationID, userID).First(&cursor).Error, "GetGroupReadCursor failed")
}

// UpsertGroupReadCursor inserts or updates a group read cursor
func (d *DataBase) UpsertGroupReadCursor(ctx context.Context, cursor *model_struct.LocalGroupReadCursor) error {
	d.mRWMutex.Lock()
	defer d.mRWMutex.Unlock()
	return errs.WrapMsg(d.conn.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "conversation_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"max_read_seq", "cursor_version"}),
	}).Create(cursor).Error, "UpsertGroupReadCursor failed")
}

// BatchUpsertGroupReadCursors batch inserts or updates group read cursors
func (d *DataBase) BatchUpsertGroupReadCursors(ctx context.Context, cursors []*model_struct.LocalGroupReadCursor) error {
	if len(cursors) == 0 {
		return nil
	}
	d.mRWMutex.Lock()
	defer d.mRWMutex.Unlock()
	return errs.WrapMsg(d.conn.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "conversation_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"max_read_seq", "cursor_version"}),
	}).CreateInBatches(cursors, 100).Error, "BatchUpsertGroupReadCursors failed")
}

// DeleteGroupReadCursors deletes all read cursors for a conversation
func (d *DataBase) DeleteGroupReadCursors(ctx context.Context, conversationID string) error {
	d.mRWMutex.Lock()
	defer d.mRWMutex.Unlock()
	return errs.WrapMsg(d.conn.WithContext(ctx).Where("conversation_id = ?", conversationID).Delete(&model_struct.LocalGroupReadCursor{}).Error, "DeleteGroupReadCursors failed")
}

// GetGroupReadCursorsVersion gets the cursor version for a conversation
func (d *DataBase) GetGroupReadCursorsVersion(ctx context.Context, conversationID string) (int64, error) {
	d.mRWMutex.RLock()
	defer d.mRWMutex.RUnlock()
	var cursor model_struct.LocalGroupReadCursor
	err := d.conn.WithContext(ctx).Where("conversation_id = ?", conversationID).Order("cursor_version DESC").First(&cursor).Error
	if err != nil {
		return 0, nil // Return 0 if no cursors found
	}
	return cursor.CursorVersion, nil
}
