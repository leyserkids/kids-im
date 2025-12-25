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

type LocalReadState struct {
}

func NewLocalReadState() *LocalReadState {
	return &LocalReadState{}
}

// GetReadStateDB uses DB suffix to avoid name conflict with WASM API function getReadState
func (l *LocalReadState) GetReadStateDB(ctx context.Context, conversationID string) (*model_struct.LocalReadState, error) {
	state, err := exec.Exec(conversationID)
	if err != nil {
		return nil, err
	}
	if v, ok := state.(string); ok {
		result := model_struct.LocalReadState{}
		err := utils.JsonStringToStruct(v, &result)
		if err != nil {
			return nil, err
		}
		return &result, nil
	}
	return nil, exec.ErrType
}

// GetReadState wraps GetReadStateDB for interface compatibility
func (l *LocalReadState) GetReadState(ctx context.Context, conversationID string) (*model_struct.LocalReadState, error) {
	return l.GetReadStateDB(ctx, conversationID)
}

// UpsertReadStateDB uses DB suffix to avoid name conflict with WASM API function
func (l *LocalReadState) UpsertReadStateDB(ctx context.Context, state *model_struct.LocalReadState) error {
	_, err := exec.Exec(utils.StructToJsonString(state))
	return err
}

// UpsertReadState wraps UpsertReadStateDB for interface compatibility
func (l *LocalReadState) UpsertReadState(ctx context.Context, state *model_struct.LocalReadState) error {
	return l.UpsertReadStateDB(ctx, state)
}

// UpdateReadStateAllReadSeqDB uses DB suffix to avoid name conflict with WASM API function
func (l *LocalReadState) UpdateReadStateAllReadSeqDB(ctx context.Context, conversationID string, allReadSeq int64) error {
	_, err := exec.Exec(conversationID, allReadSeq)
	return err
}

// UpdateReadStateAllReadSeq wraps UpdateReadStateAllReadSeqDB for interface compatibility
func (l *LocalReadState) UpdateReadStateAllReadSeq(ctx context.Context, conversationID string, allReadSeq int64) error {
	return l.UpdateReadStateAllReadSeqDB(ctx, conversationID, allReadSeq)
}

// DeleteReadStateDB uses DB suffix to avoid name conflict with WASM API function
func (l *LocalReadState) DeleteReadStateDB(ctx context.Context, conversationID string) error {
	_, err := exec.Exec(conversationID)
	return err
}

// DeleteReadState wraps DeleteReadStateDB for interface compatibility
func (l *LocalReadState) DeleteReadState(ctx context.Context, conversationID string) error {
	return l.DeleteReadStateDB(ctx, conversationID)
}
