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

type TeamStorage interface {
	Get(ctx context.Context, id int) (*Team, error)
	GetAll(ctx context.Context) ([]*Team, error)
	Create(ctx context.Context, team *Team) error
	Update(ctx context.Context, team *Team) error
	Delete(ctx context.Context, id int) error
}

type DynamoTeamStorage struct {
	Client    *dynamodb.Client
	TableName string
}

func (s *DynamoTeamStorage) GetAll(ctx context.Context) ([]*Team, error) {
	out, err := s.Client.Scan(ctx, &dynamodb.ScanInput{
		TableName: &s.TableName,
	})
	if err != nil {
		logging.Log.Errorf("TEAM: scan failed: %v", err)
		return nil, err
	}

	var teams []*Team
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &teams); err != nil {
		logging.Log.Errorf("TEAM: failed to unmarshal team list: %v", err)
		return nil, err
	}
	return teams, nil
}

func (s *DynamoTeamStorage) Get(ctx context.Context, id int) (*Team, error) {
	key, err := attributevalue.MarshalMap(map[string]int{"PK": id})
	if err != nil {
		logging.Log.Errorf("TEAM: failed to marshal key for ID %d: %v", id, err)
		return nil, err
	}

	out, err := s.Client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &s.TableName,
		Key:       key,
	})
	if err != nil {
		logging.Log.Errorf("TEAM: GetItem for ID %d failed: %v", id, err)
		return nil, err
	}
	if out.Item == nil {
		logging.Log.Warnf("TEAM: no team found with ID %d", id)
		return nil, nil
	}

	var team Team
	if err := attributevalue.UnmarshalMap(out.Item, &team); err != nil {
		logging.Log.Errorf("TEAM: failed to unmarshal team: %v", err)
		return nil, err
	}
	return &team, nil
}

func (s *DynamoTeamStorage) Create(ctx context.Context, team *Team) error {
	item, err := attributevalue.MarshalMap(team)
	if err != nil {
		logging.Log.Errorf("TEAM: failed to marshal team: %v", err)
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
			logging.Log.Warnf("TEAM: item with ID %d already exists", team.ID)
			return ErrItemWithIDAlreadyExists
		}
		logging.Log.Errorf("TEAM: failed to create team: %v", err)
		return err
	}
	return nil
}

func (s *DynamoTeamStorage) Update(ctx context.Context, team *Team) error {
	item, err := attributevalue.MarshalMap(team)
	if err != nil {
		logging.Log.Errorf("TEAM: failed to marshal updated team: %v", err)
		return err
	}

	_, err = s.Client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: &s.TableName,
		Item:      item,
	})
	if err != nil {
		logging.Log.Errorf("TEAM: failed to update team: %v", err)
		return err
	}
	return nil
}

func (s *DynamoTeamStorage) Delete(ctx context.Context, id int) error {
	key, err := attributevalue.MarshalMap(map[string]int{"PK": id})
	if err != nil {
		logging.Log.Errorf("TEAM: failed to marshal delete key for ID %d: %v", id, err)
		return err
	}

	_, err = s.Client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: &s.TableName,
		Key:       key,
	})
	if err != nil {
		logging.Log.Errorf("TEAM: failed to delete team with ID %d: %v", id, err)
		return err
	}
	logging.Log.Infof("TEAM: deleted team with ID %d", id)
	return nil
}
