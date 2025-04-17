package storage

import "time"

type VotingCode struct {
	Code      string    `dynamodbav:"PK"`
	Category  string    `dynamodbav:"Category"`
	CreatedAt time.Time `dynamodbav:"CreatedAt"`
	Used      bool      `dynamodbav:"Used"`
}
