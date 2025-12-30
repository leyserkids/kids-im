package mgo

import (
	"context"
	"errors"
	"github.com/openimsdk/open-im-server/v3/pkg/common/storage/database"
	"github.com/openimsdk/open-im-server/v3/pkg/common/storage/model"
	"github.com/openimsdk/tools/db/mongoutil"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func NewSeqUserMongo(db *mongo.Database) (database.SeqUser, error) {
	coll := db.Collection(database.SeqUserName)
	_, err := coll.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{
			{Key: "user_id", Value: 1},
			{Key: "conversation_id", Value: 1},
		},
	})
	if err != nil {
		return nil, err
	}
	return &seqUserMongo{coll: coll}, nil
}

type seqUserMongo struct {
	coll *mongo.Collection
}

func (s *seqUserMongo) setSeq(ctx context.Context, conversationID string, userID string, seq int64, field string) error {
	filter := map[string]any{
		"user_id":         userID,
		"conversation_id": conversationID,
	}
	insert := bson.M{
		"user_id":         userID,
		"conversation_id": conversationID,
		"min_seq":         0,
		"max_seq":         0,
		"read_seq":        0,
	}
	delete(insert, field)
	update := map[string]any{
		"$set": bson.M{
			field: seq,
		},
		"$setOnInsert": insert,
	}
	opt := options.Update().SetUpsert(true)
	return mongoutil.UpdateOne(ctx, s.coll, filter, update, false, opt)
}

func (s *seqUserMongo) getSeq(ctx context.Context, conversationID string, userID string, failed string) (int64, error) {
	filter := map[string]any{
		"user_id":         userID,
		"conversation_id": conversationID,
	}
	opt := options.FindOne().SetProjection(bson.M{"_id": 0, failed: 1})
	seq, err := mongoutil.FindOne[int64](ctx, s.coll, filter, opt)
	if err == nil {
		return seq, nil
	} else if errors.Is(err, mongo.ErrNoDocuments) {
		return 0, nil
	} else {
		return 0, err
	}
}

func (s *seqUserMongo) GetUserMaxSeq(ctx context.Context, conversationID string, userID string) (int64, error) {
	return s.getSeq(ctx, conversationID, userID, "max_seq")
}

func (s *seqUserMongo) SetUserMaxSeq(ctx context.Context, conversationID string, userID string, seq int64) error {
	return s.setSeq(ctx, conversationID, userID, seq, "max_seq")
}

func (s *seqUserMongo) GetUserMinSeq(ctx context.Context, conversationID string, userID string) (int64, error) {
	return s.getSeq(ctx, conversationID, userID, "min_seq")
}

func (s *seqUserMongo) SetUserMinSeq(ctx context.Context, conversationID string, userID string, seq int64) error {
	return s.setSeq(ctx, conversationID, userID, seq, "min_seq")
}

func (s *seqUserMongo) GetUserReadSeq(ctx context.Context, conversationID string, userID string) (int64, error) {
	return s.getSeq(ctx, conversationID, userID, "read_seq")
}

func (s *seqUserMongo) notFoundSet0(seq map[string]int64, conversationIDs []string) {
	for _, conversationID := range conversationIDs {
		if _, ok := seq[conversationID]; !ok {
			seq[conversationID] = 0
		}
	}
}

func (s *seqUserMongo) GetUserReadSeqs(ctx context.Context, userID string, conversationID []string) (map[string]int64, error) {
	if len(conversationID) == 0 {
		return map[string]int64{}, nil
	}
	filter := bson.M{"user_id": userID, "conversation_id": bson.M{"$in": conversationID}}
	opt := options.Find().SetProjection(bson.M{"_id": 0, "conversation_id": 1, "read_seq": 1})
	seqs, err := mongoutil.Find[*model.SeqUser](ctx, s.coll, filter, opt)
	if err != nil {
		return nil, err
	}
	res := make(map[string]int64)
	for _, seq := range seqs {
		res[seq.ConversationID] = seq.ReadSeq
	}
	s.notFoundSet0(res, conversationID)
	return res, nil
}

func (s *seqUserMongo) SetUserReadSeq(ctx context.Context, conversationID string, userID string, seq int64) error {
	dbSeq, err := s.GetUserReadSeq(ctx, conversationID, userID)
	if err != nil {
		return err
	}
	if dbSeq > seq {
		return nil
	}
	return s.setSeq(ctx, conversationID, userID, seq, "read_seq")
}

// GetConversationsUserReadSeqs gets read seqs for specified users in multiple conversations
func (s *seqUserMongo) GetConversationsUserReadSeqs(ctx context.Context, conversationUserIDs map[string][]string) (map[string]map[string]int64, error) {
	if len(conversationUserIDs) == 0 {
		return map[string]map[string]int64{}, nil
	}
	// 收集所有 conversationIDs，并构建用户集合用于快速查找
	conversationIDs := make([]string, 0, len(conversationUserIDs))
	userIDSets := make(map[string]map[string]struct{})
	for conversationID, userIDs := range conversationUserIDs {
		conversationIDs = append(conversationIDs, conversationID)
		set := make(map[string]struct{})
		for _, uid := range userIDs {
			set[uid] = struct{}{}
		}
		userIDSets[conversationID] = set
	}
	// 简单查询：获取这些会话的所有用户数据
	filter := bson.M{"conversation_id": bson.M{"$in": conversationIDs}}
	opt := options.Find().SetProjection(bson.M{"_id": 0, "conversation_id": 1, "user_id": 1, "read_seq": 1})
	seqs, err := mongoutil.Find[*model.SeqUser](ctx, s.coll, filter, opt)
	if err != nil {
		return nil, err
	}
	// 在应用层过滤：只保留指定用户的数据
	res := make(map[string]map[string]int64)
	for _, seq := range seqs {
		if userSet, ok := userIDSets[seq.ConversationID]; ok {
			if _, exists := userSet[seq.UserID]; exists {
				if res[seq.ConversationID] == nil {
					res[seq.ConversationID] = make(map[string]int64)
				}
				res[seq.ConversationID][seq.UserID] = seq.ReadSeq
			}
		}
	}
	return res, nil
}

