package models

import (
	"github.com/alex-pricope/simple-voting-system/storage"
	"time"
)

type VotingCategory string

const (
	CategoryGrandJury     VotingCategory = "grand_jury"
	CategoryOtherTeam     VotingCategory = "other_team"
	CategoryGeneralPublic VotingCategory = "general_public"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

type CodeValidationResponse struct {
	Valid     bool      `json:"valid"`
	Category  string    `json:"category"`
	Used      bool      `json:"used,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	Code      string    `json:"code,omitempty"`
}

func TransformVotingCodeToValidationResponse(vc *storage.VotingCode) CodeValidationResponse {
	return CodeValidationResponse{
		Valid:     true,
		Category:  vc.Category,
		Used:      vc.Used,
		CreatedAt: vc.CreatedAt,
		Code:      vc.Code,
	}
}
