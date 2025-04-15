package models

type VoteEntry struct {
	CategoryID int `json:"categoryId"`
	TeamID     int `json:"teamId"`
	Rating     int `json:"rating"`
}

type RegisterVoteRequest struct {
	Code  string      `json:"code"`
	Votes []VoteEntry `json:"votes"`
}

type RegisterVoteResponse struct {
	Message string `json:"message"`
}
