package main

// import (
// 	"bytes"
// 	"context"
// 	"encoding/json"
// 	"fmt"
// 	"net/http"
// )

// type BackendConfig struct {
// 	SemaphoreSize     int `json:"semaphoreSize"`
// 	ConcurrentDelayMs int `json:"concurrentDelayMs"`
// 	BaseLatencyMs     int `json:"baseLatencyMs"`
// }

// func (h *Handler) updateBackendConfig(
// 	ctx context.Context,
// 	size int,
// 	concurrent int,
// 	base int,
// ) error {
// 	body, err := json.Marshal(BackendConfig{
// 		SemaphoreSize:     size,
// 		ConcurrentDelayMs: concurrent,
// 		BaseLatencyMs:     base,
// 	})
// 	if err != nil {
// 		return err
// 	}

// 	url := h.backendURL + "/config"

// 	req, err := http.NewRequestWithContext(
// 		ctx,
// 		http.MethodPut,
// 		url,
// 		bytes.NewReader(body),
// 	)
// 	if err != nil {
// 		return err
// 	}

// 	req.Header.Set("Content-Type", "application/json")

// 	resp, err := h.httpClient.Do(req)
// 	if err != nil {
// 		return err
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
// 		return fmt.Errorf(
// 			"backend returned status %d",
// 			resp.StatusCode,
// 		)
// 	}

// 	return nil
// }
