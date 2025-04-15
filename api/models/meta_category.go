package models

import "github.com/alex-pricope/simple-voting-system/storage"

type VotingCategoryCreateRequest struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type VotingCategoryUpdateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type VotingCategoryResponse struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func TransformVotingCategoryFromStorage(vc *storage.VotingCategory) VotingCategoryResponse {
	return VotingCategoryResponse{
		ID:          vc.ID,
		Name:        vc.Name,
		Description: vc.Description,
	}
}
