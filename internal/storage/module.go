package storage

import (
	"context"
	"fmt"
)

type Module struct {
	service *Service
}

func Setup(ctx context.Context, cfg Config) (*Module, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	client, err := NewR2Client(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("initialize r2 client: %w", err)
	}

	service, err := NewService(cfg.Bucket, client, NewR2PresignClient(client), PresignConfig{
		PutExpiry: cfg.PresignPutExpiry,
		GetExpiry: cfg.PresignGetExpiry,
	})
	if err != nil {
		return nil, err
	}

	return &Module{
		service: service,
	}, nil
}

func (m *Module) Service() *Service {
	if m == nil {
		return nil
	}

	return m.service
}
