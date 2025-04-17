package storage

import (
	"context"
)

type Storage interface {
	Get(ctx context.Context, code string) (*VotingCode, error)
	GetAll(ctx context.Context) ([]*VotingCode, error)
	Put(ctx context.Context, votingCode *VotingCode) error
	Delete(ctx context.Context, code string) error
}
