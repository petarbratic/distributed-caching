package main

import (
	"errors"
	"net/http"
	"sync"
	"testing"
)

func newSingleFlightTestHandler() *Handler {
	return &Handler{
		inFlight: make(map[string]*inFlightCall),
	}
}

func TestGetOrCreateInFlight_FirstRequestIsLeader(t *testing.T) {
	handler := newSingleFlightTestHandler()
	key := "/api/backend/1"

	call, isLeader := handler.getOrCreateInFlight(key)

	if !isLeader {
		t.Fatal("expected the first request to be the leader")
	}

	if call == nil {
		t.Fatal("expected an in-flight call to be created")
	}

	if call.done == nil {
		t.Fatal("expected the done channel to be initialized")
	}

	if handler.inFlight[key] != call {
		t.Fatal("expected the call to be stored in the inFlight map")
	}
}

func TestGetOrCreateInFlight_SecondRequestIsFollower(t *testing.T) {
	handler := newSingleFlightTestHandler()
	key := "/api/backend/1"

	leaderCall, isLeader := handler.getOrCreateInFlight(key)
	if !isLeader {
		t.Fatal("expected the first request to be the leader")
	}

	followerCall, isLeader := handler.getOrCreateInFlight(key)
	if isLeader {
		t.Fatal("expected the second request to be a follower")
	}

	if followerCall != leaderCall {
		t.Fatal("expected leader and follower to share the same in-flight call")
	}
}

func TestGetOrCreateInFlight_DifferentKeysHaveDifferentLeaders(t *testing.T) {
	handler := newSingleFlightTestHandler()

	firstCall, firstIsLeader := handler.getOrCreateInFlight("/api/backend/1")

	secondCall, secondIsLeader := handler.getOrCreateInFlight("/api/backend/2")

	if !firstIsLeader || !secondIsLeader {
		t.Fatal("expected the first request for each key to be a leader")
	}

	if firstCall == secondCall {
		t.Fatal("expected different keys to have different in-flight calls")
	}

	if len(handler.inFlight) != 2 {
		t.Fatalf("expected 2 active calls, got %d", len(handler.inFlight))
	}
}

func TestFinishInFlight_PublishesResultAndRemovesCall(t *testing.T) {
	handler := newSingleFlightTestHandler()
	key := "/api/backend/1"

	call, _ := handler.getOrCreateInFlight(key)

	expectedData := []byte(`{"id":"1"}`)
	expectedStatus := http.StatusOK
	expectedError := errors.New("test error")

	handler.finishInFlight(key, call, expectedData, expectedStatus, expectedError)

	select {
	case <-call.done:
	default:
		t.Fatal("expected the done channel to be closed")
	}

	if string(call.data) != string(expectedData) {
		t.Fatalf("expected data %q, got %q", expectedData, call.data)
	}

	if call.statusCode != expectedStatus {
		t.Fatalf("expected status code %d, got %d", expectedStatus, call.statusCode)
	}

	if !errors.Is(call.err, expectedError) {
		t.Fatalf("expected error %v, got %v", expectedError, call.err)
	}

	if _, exists := handler.inFlight[key]; exists {
		t.Fatal("expected the call to be removed from the inFlight map")
	}
}

func TestFinishInFlight_CopiesResponseData(t *testing.T) {
	handler := newSingleFlightTestHandler()
	key := "/api/backend/1"

	call, _ := handler.getOrCreateInFlight(key)

	data := []byte("original")

	handler.finishInFlight(key, call, data, http.StatusOK, nil)

	data[0] = 'X'

	if string(call.data) != "original" {
		t.Fatalf("expected stored data to be an independent copy, got %q", call.data)
	}
}

func TestGetOrCreateInFlight_ConcurrentRequestsHaveOneLeader(t *testing.T) {
	handler := newSingleFlightTestHandler()
	key := "/api/backend/1"

	const requestCount = 100

	var waitGroup sync.WaitGroup
	var resultMu sync.Mutex

	leaderCount := 0
	calls := make([]*inFlightCall, 0, requestCount)

	waitGroup.Add(requestCount)

	for i := 0; i < requestCount; i++ {
		go func() {
			defer waitGroup.Done()

			call, isLeader := handler.getOrCreateInFlight(key)

			resultMu.Lock()
			defer resultMu.Unlock()

			calls = append(calls, call)

			if isLeader {
				leaderCount++
			}
		}()
	}

	waitGroup.Wait()

	if leaderCount != 1 {
		t.Fatalf("expected exactly one leader, got %d", leaderCount)
	}

	firstCall := calls[0]

	for _, call := range calls {
		if call != firstCall {
			t.Fatal("expected all requests to share the same in-flight call")
		}
	}

	if len(handler.inFlight) != 1 {
		t.Fatalf("expected one active call, got %d", len(handler.inFlight))
	}
}
