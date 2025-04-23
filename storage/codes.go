package storage

import (
	"context"
	"github.com/alex-pricope/simple-voting-system/logging"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"time"
)

type VotingCodeStorage interface {
	Get(ctx context.Context, code string) (*VotingCode, error)
	GetAll(ctx context.Context) ([]*VotingCode, error)
	Put(ctx context.Context, votingCode *VotingCode) error
	MarkUnused(ctx context.Context, code string) error
	MarkUsed(ctx context.Context, code string) error
	Delete(ctx context.Context, code string) error
}

type DynamoVotingCodesStorage struct {
	Client    *dynamodb.Client
	TableName string
}

func (s *DynamoVotingCodesStorage) Get(ctx context.Context, code string) (*VotingCode, error) {
	key, err := attributevalue.MarshalMap(map[string]string{"PK": code})
	if err != nil {
		logging.Log.Errorf("failed to marshal key: %v", err)
		return &VotingCode{}, err
	}

	out, err := s.Client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &s.TableName,
		Key:       key,
	})
	if err != nil {
		logging.Log.Errorf("GET storage failed: %v", err)
		return &VotingCode{}, err
	}
	if out.Item == nil {
		return nil, ErrCodeNotFound
	}

	var vc *VotingCode
	if err := attributevalue.UnmarshalMap(out.Item, &vc); err != nil {
		logging.Log.Errorf("failed to unmarshal result: %v", err)
		return &VotingCode{}, err
	}
	return vc, nil
}

func (s *DynamoVotingCodesStorage) GetAll(ctx context.Context) ([]*VotingCode, error) {
	out, err := s.Client.Scan(ctx, &dynamodb.ScanInput{
		TableName: &s.TableName,
	})
	if err != nil {
		logging.Log.Errorf("SCAN storage failed: %v", err)
		return nil, err
	}

	var codes []*VotingCode
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &codes); err != nil {
		logging.Log.Errorf("failed to unmarshal list: %v", err)
		return nil, err
	}
	return codes, nil
}

func (s *DynamoVotingCodesStorage) Put(ctx context.Context, code *VotingCode) error {
	if code.CreatedAt.IsZero() {
		code.CreatedAt = time.Now().UTC()
	}
	code.Used = false
	item, err := attributevalue.MarshalMap(code)
	if err != nil {
		logging.Log.Errorf("failed to marshal code: %v", err)
		return err
	}

	_, err = s.Client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           &s.TableName,
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(PK)"),
	})
	if err != nil {
		logging.Log.Errorf("PUT storage failed: %v", err)
		return err
	}
	return nil
}

func (s *DynamoVotingCodesStorage) MarkUnused(ctx context.Context, code string) error {
	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(s.TableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: code},
		},
		UpdateExpression:          aws.String("SET Used = :val"),
		ExpressionAttributeValues: map[string]types.AttributeValue{":val": &types.AttributeValueMemberBOOL{Value: false}},
	}
	_, err := s.Client.UpdateItem(ctx, input)
	return err
}

func (s *DynamoVotingCodesStorage) MarkUsed(ctx context.Context, code string) error {
	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(s.TableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: code},
		},
		UpdateExpression:          aws.String("SET Used = :val"),
		ExpressionAttributeValues: map[string]types.AttributeValue{":val": &types.AttributeValueMemberBOOL{Value: true}},
	}
	_, err := s.Client.UpdateItem(ctx, input)
	return err
}

func (s *DynamoVotingCodesStorage) Delete(ctx context.Context, code string) error {
	key, err := attributevalue.MarshalMap(map[string]string{"PK": code})
	if err != nil {
		logging.Log.Errorf("failed to marshal key: %v", err)
		return err
	}

	_, err = s.Client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: &s.TableName,
		Key:       key,
	})
	if err != nil {
		logging.Log.Errorf("DEL storage item failed: %v", err)
		return err
	}
	return nil
}
