package service

import (
	"context"
	"fmt"
	"time"
)

type Service struct {
}

type Entity struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (service *Service) FindEntity(
	ctx context.Context,
	id string,
	baseLatency time.Duration,
) (*Entity, error) {

	// Simulating data fetching here
	if id == "" {
		return nil, fmt.Errorf("Invalid id")
	}

	select {
	case <-time.After(baseLatency):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	entity := &Entity{
		ID:   id,
		Name: "Test Entity",
	}

	return entity, nil
}
