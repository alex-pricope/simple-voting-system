package storage

import "time"

type VotingCode struct {
	Code      string    `dynamodbav:"PK"`
	Category  string    `dynamodbav:"Category"`
	TeamID    *int      `dynamodbav:"TeamID"`
	CreatedAt time.Time `dynamodbav:"CreatedAt"`
	Used      bool      `dynamodbav:"Used"`
}

type VotingCategory struct {
	ID          int     `dynamodbav:"PK"`
	Name        string  `dynamodbav:"Name"`
	Description string  `dynamodbav:"Description"`
	Weight      float64 `dynamodbav:"Weight"`
}

type Team struct {
	ID          int      `dynamodbav:"PK"`
	Name        string   `dynamodbav:"Name"`
	Members     []string `dynamodbav:"Members"`
	Description string   `dynamodbav:"Description"`
}

type Vote struct {
	Code       string    `dynamodbav:"PK" json:"code"` // Voting code
	SortKey    string    `dynamodbav:"SK" json:"-"`    // Unique composite of category/team
	CategoryID int       `dynamodbav:"CategoryID" json:"categoryId"`
	TeamID     int       `dynamodbav:"TeamID" json:"teamId"`
	Rating     int       `dynamodbav:"Rating" json:"rating"`
	Timestamp  time.Time `dynamodbav:"Timestamp" json:"timestamp"`
}
