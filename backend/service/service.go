package service

import (
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
	id string,
	baseLatency time.Duration,
) (*Entity, error) {

	// Simulating data fetching here
	if id == "" {
		return nil, fmt.Errorf("Invalid id")
	}

	time.Sleep(baseLatency)

	entity := &Entity{
		ID:   id,
		Name: "Test Entity",
	}

	return entity, nil
}
