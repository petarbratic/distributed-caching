package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type SemaphoreConfig struct {
	SemaphoreSize int `json:"semaphoreSize"`
}

func (h *Handler) updateBackendSemaphore(
	ctx context.Context,
	size int,
) error {
	body, err := json.Marshal(SemaphoreConfig{
		SemaphoreSize: size,
	})
	if err != nil {
		return err
	}

	url := h.backendURL + "/config/semaphore"

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPut,
		url,
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf(
			"backend returned status %d",
			resp.StatusCode,
		)
	}

	return nil
}
