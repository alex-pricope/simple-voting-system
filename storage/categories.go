package storage

import (
	"context"
	"errors"
	"github.com/alex-pricope/simple-voting-system/logging"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type VotingCategoryStorage interface {
	Get(ctx context.Context, id int) (*VotingCategory, error)
	GetAll(ctx context.Context) ([]*VotingCategory, error)
	Create(ctx context.Context, category *VotingCategory) error
	Update(ctx context.Context, category *VotingCategory) error
	Delete(ctx context.Context, id int) error
}

type DynamoVotingCategoryStorage struct {
	Client    *dynamodb.Client
	TableName string
}

func (s *DynamoVotingCategoryStorage) Get(ctx context.Context, id int) (*VotingCategory, error) {
	key, err := attributevalue.MarshalMap(map[string]int{"PK": id})
	if err != nil {
		logging.Log.Errorf("CATEGORY: failed to marshal key for ID %d: %v", id, err)
		return nil, err
	}

	out, err := s.Client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &s.TableName,
		Key:       key,
	})
	if err != nil {
		logging.Log.Errorf("CATEGORY: GetItem for ID %d failed: %v", id, err)
		return nil, err
	}
	if out.Item == nil {
		logging.Log.Warnf("CATEGORY: no category found with ID %d", id)
		return nil, nil
	}

	var category VotingCategory
	if err := attributevalue.UnmarshalMap(out.Item, &category); err != nil {
		logging.Log.Errorf("CATEGORY: failed to unmarshal category: %v", err)
		return nil, err
	}
	return &category, nil
}

func (s *DynamoVotingCategoryStorage) GetAll(ctx context.Context) ([]*VotingCategory, error) {
	out, err := s.Client.Scan(ctx, &dynamodb.ScanInput{
		TableName: &s.TableName,
	})
	if err != nil {
		logging.Log.Errorf("CATEGORY: scan failed: %v", err)
		return nil, err
	}

	var categories []*VotingCategory
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &categories); err != nil {
		logging.Log.Errorf("CATEGORY: failed to unmarshal list: %v", err)
		return nil, err
	}
	return categories, nil
}

func (s *DynamoVotingCategoryStorage) Create(ctx context.Context, category *VotingCategory) error {
	item, err := attributevalue.MarshalMap(category)
	if err != nil {
		logging.Log.Errorf("CATEGORY: failed to marshal category: %v", err)
		return err
	}

	_, err = s.Client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           &s.TableName,
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(PK)"),
	})
	if err != nil {
		var cce *types.ConditionalCheckFailedException
		if errors.As(err, &cce) {
			logging.Log.Warnf("CATEGORY: item with ID %d already exists", category.ID)
			return ErrItemWithIDAlreadyExists
		}
		logging.Log.Errorf("CATEGORY: failed to create category: %v", err)
		return err
	}
	return nil
}

func (s *DynamoVotingCategoryStorage) Update(ctx context.Context, category *VotingCategory) error {
	item, err := attributevalue.MarshalMap(category)
	if err != nil {
		logging.Log.Errorf("CATEGORY: failed to marshal updated category: %v", err)
		return err
	}

	_, err = s.Client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: &s.TableName,
		Item:      item,
	})
	if err != nil {
		logging.Log.Errorf("CATEGORY: failed to update category: %v", err)
		return err
	}
	return nil
}

func (s *DynamoVotingCategoryStorage) Delete(ctx context.Context, id int) error {
	key, err := attributevalue.MarshalMap(map[string]int{"PK": id})
	if err != nil {
		logging.Log.Errorf("CATEGORY: failed to marshal delete key for ID %d: %v", id, err)
		return err
	}

	_, err = s.Client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: &s.TableName,
		Key:       key,
	})
	if err != nil {
		logging.Log.Errorf("CATEGORY: failed to delete category with ID %d: %v", id, err)
		return err
	}
	logging.Log.Infof("CATEGORY: deleted category with ID %d", id)
	return nil
}
