package storage

import (
	"context"
	"github.com/alex-pricope/simple-voting-system/logging"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type VoteStorage interface {
	GetAll(ctx context.Context) ([]*Vote, error)
	Create(ctx context.Context, vote *Vote) error
	GetByCode(ctx context.Context, code string) ([]*Vote, error)
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

func (s *DynamoVoteStorage) GetByCode(ctx context.Context, code string) ([]*Vote, error) {
	input := &dynamodb.QueryInput{
		TableName:              &s.TableName,
		KeyConditionExpression: aws.String("PK = :code"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":code": &types.AttributeValueMemberS{Value: code},
		},
	}

	output, err := s.Client.Query(ctx, input)
	if err != nil {
		logging.Log.Errorf("VOTE: failed to query votes by code: %v", err)
		return nil, err
	}

	var votes []*Vote
	if err := attributevalue.UnmarshalListOfMaps(output.Items, &votes); err != nil {
		logging.Log.Errorf("VOTE: failed to unmarshal votes for code %s: %v", code, err)
		return nil, err
	}
	return votes, nil
}
