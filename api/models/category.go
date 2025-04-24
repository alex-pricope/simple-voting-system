package models

import "github.com/alex-pricope/simple-voting-system/storage"

type VotingCategoryCreateRequest struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Weight      float64 `json:"weight"`
}

type VotingCategoryUpdateRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Weight      float64 `json:"weight"`
}

type VotingCategoryResponse struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Weight      float64 `json:"weight"`
}

func TransformVotingCategoryFromStorage(vc *storage.VotingCategory) VotingCategoryResponse {
	return VotingCategoryResponse{
		ID:          vc.ID,
		Name:        vc.Name,
		Description: vc.Description,
		Weight:      vc.Weight,
	}
}
