package models

import (
	"github.com/alex-pricope/simple-voting-system/storage"
)

type TeamCreateRequest struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Members     []string `json:"members"`
}

type TeamResponse struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	Members     []string `json:"members"`
	Description string   `json:"description"`
}

type TeamUpdateRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Members     []string `json:"members"`
}

func TransformTeamFromStorage(vc *storage.Team) TeamResponse {
	return TeamResponse{
		ID:          vc.ID,
		Name:        vc.Name,
		Description: vc.Description,
		Members:     vc.Members,
	}
}
