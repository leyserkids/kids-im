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

type LocalGroupReadState struct {
}

func NewLocalGroupReadState() *LocalGroupReadState {
	return &LocalGroupReadState{}
}

// GetGroupReadStateDB uses DB suffix to avoid name conflict with WASM API function getGroupReadState
func (l *LocalGroupReadState) GetGroupReadStateDB(ctx context.Context, conversationID string) (*model_struct.LocalGroupReadState, error) {
	state, err := exec.Exec(conversationID)
	if err != nil {
		return nil, err
	}
	if v, ok := state.(string); ok {
		result := model_struct.LocalGroupReadState{}
		err := utils.JsonStringToStruct(v, &result)
		if err != nil {
			return nil, err
		}
		return &result, nil
	}
	return nil, exec.ErrType
}

// GetGroupReadState wraps GetGroupReadStateDB for interface compatibility
func (l *LocalGroupReadState) GetGroupReadState(ctx context.Context, conversationID string) (*model_struct.LocalGroupReadState, error) {
	return l.GetGroupReadStateDB(ctx, conversationID)
}

// UpsertGroupReadStateDB uses DB suffix to avoid name conflict with WASM API function
func (l *LocalGroupReadState) UpsertGroupReadStateDB(ctx context.Context, state *model_struct.LocalGroupReadState) error {
	_, err := exec.Exec(utils.StructToJsonString(state))
	return err
}

// UpsertGroupReadState wraps UpsertGroupReadStateDB for interface compatibility
func (l *LocalGroupReadState) UpsertGroupReadState(ctx context.Context, state *model_struct.LocalGroupReadState) error {
	return l.UpsertGroupReadStateDB(ctx, state)
}

// UpdateGroupReadStateMinSeqDB uses DB suffix to avoid name conflict with WASM API function
func (l *LocalGroupReadState) UpdateGroupReadStateMinSeqDB(ctx context.Context, conversationID string, minReadSeq int64) error {
	_, err := exec.Exec(conversationID, minReadSeq)
	return err
}

// UpdateGroupReadStateMinSeq wraps UpdateGroupReadStateMinSeqDB for interface compatibility
func (l *LocalGroupReadState) UpdateGroupReadStateMinSeq(ctx context.Context, conversationID string, minReadSeq int64) error {
	return l.UpdateGroupReadStateMinSeqDB(ctx, conversationID, minReadSeq)
}

// DeleteGroupReadStateDB uses DB suffix to avoid name conflict with WASM API function
func (l *LocalGroupReadState) DeleteGroupReadStateDB(ctx context.Context, conversationID string) error {
	_, err := exec.Exec(conversationID)
	return err
}

// DeleteGroupReadState wraps DeleteGroupReadStateDB for interface compatibility
func (l *LocalGroupReadState) DeleteGroupReadState(ctx context.Context, conversationID string) error {
	return l.DeleteGroupReadStateDB(ctx, conversationID)
}
