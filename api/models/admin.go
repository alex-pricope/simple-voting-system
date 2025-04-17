package models

type CreateCodeRequest struct {
	Category string `json:"category"`
	Count    int    `json:"count"`
}
