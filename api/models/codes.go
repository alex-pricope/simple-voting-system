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

var CodeCategoryWeights = map[VotingCategory]float64{
	CategoryGrandJury:     0.5,
	CategoryOtherTeam:     0.3,
	CategoryGeneralPublic: 0.2,
}

type CodeValidationResponse struct {
	Valid     bool      `json:"valid"`
	Category  string    `json:"category"`
	Used      bool      `json:"used,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	Code      string    `json:"code,omitempty"`
}

type CreateCodeRequest struct {
	Category string `json:"category"`
	Count    int    `json:"count"`
}

type CodeResponse struct {
	Category  string    `json:"category"`
	Code      string    `json:"code"`
	CreatedAt time.Time `json:"created_at"`
	Used      bool      `json:"used"`
}

func TransformVotingCodeToValidationResponse(vc *storage.VotingCode) *CodeValidationResponse {
	return &CodeValidationResponse{
		Valid:     true,
		Category:  vc.Category,
		Used:      vc.Used,
		CreatedAt: vc.CreatedAt,
		Code:      vc.Code,
	}
}

func TransformVotingCodeToCodeResponse(vc *storage.VotingCode) *CodeResponse {
	return &CodeResponse{
		Category:  vc.Category,
		Used:      vc.Used,
		CreatedAt: vc.CreatedAt,
		Code:      vc.Code,
	}
}
