package service

import (
	//"encoding/json"
	"fmt"
	//"log"
	//"net/http"
	"time"
)

type Service struct {
}

type Entity struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (service *Service) FindEntity(id string) (*Entity, error) {

	// Simulating data fetching here
	if id == "" {
		return nil, fmt.Errorf("Invalid id")
	}

	// 2 seconds (for now)
	time.Sleep(2 * time.Second)

	entity := &Entity{
		ID:   id,
		Name: "Test Entity",
	}

	return entity, nil
}
