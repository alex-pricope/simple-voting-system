package storage

import (
	"context"
	"github.com/alex-pricope/simple-voting-system/logging"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

type VoteStorage interface {
	GetAll(ctx context.Context) ([]*Vote, error)
	Create(ctx context.Context, vote *Vote) error
}

type DynamoVoteStorage struct {
	Client    *dynamodb.Client
	TableName string
}

func (s *DynamoVoteStorage) GetAll(ctx context.Context) ([]*Vote, error) {
	out, err := s.Client.Scan(ctx, &dynamodb.ScanInput{
		TableName: &s.TableName,
	})
	if err != nil {
		logging.Log.Errorf("VOTE: scan failed: %v", err)
		return nil, err
	}

	var votes []*Vote
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &votes); err != nil {
		logging.Log.Errorf("VOTE: failed to unmarshal vote list: %v", err)
		return nil, err
	}
	return votes, nil
}

func (s *DynamoVoteStorage) Create(ctx context.Context, vote *Vote) error {
	item, err := attributevalue.MarshalMap(vote)
	if err != nil {
		logging.Log.Errorf("VOTE: failed to marshal vote: %v", err)
		return err
	}
	_, err = s.Client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           &s.TableName,
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(PK) AND attribute_not_exists(SK)"),
	})
	if err != nil {
		logging.Log.Errorf("VOTE: failed to create vote: %v", err)
		return err
	}
	return nil
}
