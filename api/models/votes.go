package models

// VoteEntry represents a single vote cast for a team in a category.
type VoteEntry struct {
	CategoryID int `json:"categoryId" binding:"required"`
	TeamID     int `json:"teamId" binding:"required"`
	Rating     int `json:"rating" binding:"required,min=1,max=5"`
}

// RegisterVoteRequest is the payload for submitting a full vote set by a user.
type RegisterVoteRequest struct {
	Code  string      `json:"code" binding:"required"`
	Votes []VoteEntry `json:"votes" binding:"required,dive"`
}

type RegisterVoteResponse struct {
	Message string `json:"message"`
}

type VoteResponse struct {
	Code  string      `json:"code"`
	Votes []VoteEntry `json:"votes"`
}

type GetVoteResponse struct {
	Code  string         `json:"code"`
	Votes []GetVoteEntry `json:"votes"`
}

type GetVoteEntry struct {
	VoteEntry
	Team     string `json:"team" binding:"required"`
	Category string `json:"category"`
}

type CategoryScore struct {
	CategoryID   int     `json:"categoryId"`
	CategoryName string  `json:"category"`
	Score        float64 `json:"score"`
}

type VoteResult struct {
	TeamID      int             `json:"teamId"`
	TeamName    string          `json:"teamName"`
	TotalScore  float64         `json:"totalScore"`
	Categories  []CategoryScore `json:"categories"`
	TeamMembers []string        `json:"teamMembers"`
}

type VoteResultsResponse struct {
	TotalVotes int          `json:"totalVotes"`
	Results    []VoteResult `json:"results"`
	UsedCodes  int          `json:"usedCodes"`
}
