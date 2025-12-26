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

//go:build js && wasm
// +build js,wasm

package indexdb

import (
	"context"

	"github.com/openimsdk/openim-sdk-core/v3/pkg/db/model_struct"
	"github.com/openimsdk/openim-sdk-core/v3/pkg/utils"
	"github.com/openimsdk/openim-sdk-core/v3/wasm/exec"
)

type LocalReadCursor struct {
}

func NewLocalReadCursor() *LocalReadCursor {
	return &LocalReadCursor{}
}

func (l *LocalReadCursor) InsertReadCursor(ctx context.Context, cursor *model_struct.LocalReadCursor) error {
	_, err := exec.Exec(utils.StructToJsonString(cursor))
	return err
}

func (l *LocalReadCursor) UpsertReadCursor(ctx context.Context, cursor *model_struct.LocalReadCursor) error {
	_, err := exec.Exec(utils.StructToJsonString(cursor))
	return err
}

func (l *LocalReadCursor) UpdateReadCursor(ctx context.Context, conversationID, userID string, maxReadSeq int64) error {
	_, err := exec.Exec(conversationID, userID, maxReadSeq)
	return err
}

func (l *LocalReadCursor) GetReadCursor(ctx context.Context, conversationID, userID string) (*model_struct.LocalReadCursor, error) {
	cursor, err := exec.Exec(conversationID, userID)
	if err != nil {
		return nil, err
	}
	if v, ok := cursor.(string); ok {
		result := model_struct.LocalReadCursor{}
		err := utils.JsonStringToStruct(v, &result)
		if err != nil {
			return nil, err
		}
		return &result, nil
	}
	return nil, exec.ErrType
}

func (l *LocalReadCursor) GetReadCursorsByConversationID(ctx context.Context, conversationID string) ([]*model_struct.LocalReadCursor, error) {
	cursorList, err := exec.Exec(conversationID)
	if err != nil {
		return nil, err
	}
	if v, ok := cursorList.(string); ok {
		var result []*model_struct.LocalReadCursor
		err := utils.JsonStringToStruct(v, &result)
		if err != nil {
			return nil, err
		}
		return result, nil
	}
	return nil, exec.ErrType
}

func (l *LocalReadCursor) GetAllReadSeqFromCursors(ctx context.Context, conversationID string, excludeUserID string) (int64, error) {
	allReadSeq, err := exec.Exec(conversationID, excludeUserID)
	if err != nil {
		return 0, err
	}
	if v, ok := allReadSeq.(float64); ok {
		return int64(v), nil
	}
	return 0, exec.ErrType
}

func (l *LocalReadCursor) DeleteReadCursor(ctx context.Context, conversationID, userID string) error {
	_, err := exec.Exec(conversationID, userID)
	return err
}

func (l *LocalReadCursor) DeleteReadCursorsByConversationID(ctx context.Context, conversationID string) error {
	_, err := exec.Exec(conversationID)
	return err
}
