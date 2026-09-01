//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type telegramEntity struct {
	Type   string `json:"type"`
	Offset int    `json:"offset"`
	Length int    `json:"length"`
}

type telegramRequest struct {
	ChatID              string           `json:"chat_id"`
	Text                string           `json:"text"`
	Entities            []telegramEntity `json:"entities"`
	DisableNotification bool             `json:"disable_notification"`
}

type scriptedTelegramResponse struct {
	Status int
	Body   string
}

type fakeTelegramServer struct {
	t         *testing.T
	server    *httptest.Server
	mu        sync.Mutex
	responses []scriptedTelegramResponse
	requests  []telegramRequest
	paths     []string
}

func newFakeTelegramServer(t *testing.T, responses ...scriptedTelegramResponse) *fakeTelegramServer {
	t.Helper()
	fake := &fakeTelegramServer{t: t, responses: responses}
	fake.server = httptest.NewServer(http.HandlerFunc(fake.handle))
	t.Cleanup(fake.server.Close)
	return fake
}

func (fake *fakeTelegramServer) handle(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost || !strings.HasSuffix(request.URL.Path, "/sendMessage") {
		http.Error(writer, "unexpected endpoint", http.StatusNotFound)
		return
	}
	var decoded telegramRequest
	if err := json.NewDecoder(request.Body).Decode(&decoded); err != nil {
		http.Error(writer, "invalid request", http.StatusBadRequest)
		return
	}
	fake.mu.Lock()
	fake.requests = append(fake.requests, decoded)
	fake.paths = append(fake.paths, request.URL.Path)
	var response scriptedTelegramResponse
	if len(fake.responses) > 0 {
		response = fake.responses[0]
		fake.responses = fake.responses[1:]
	}
	fake.mu.Unlock()
	if response.Status == 0 {
		response = scriptedTelegramResponse{Status: http.StatusInternalServerError, Body: "{\"ok\":false,\"error_code\":500}"}
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(response.Status)
	_, _ = writer.Write([]byte(response.Body))
}

func (fake *fakeTelegramServer) URL() string {
	return fake.server.URL
}

func (fake *fakeTelegramServer) Requests() ([]telegramRequest, []string) {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	requests := make([]telegramRequest, len(fake.requests))
	for index, request := range fake.requests {
		requests[index] = request
		requests[index].Entities = append([]telegramEntity(nil), request.Entities...)
	}
	paths := append([]string(nil), fake.paths...)
	return requests, paths
}

func TestScriptedTelegramServerExhaustionAndDeepCopy(t *testing.T) {
	fake := newFakeTelegramServer(t, telegramSuccess(1))
	body := []byte(`{"chat_id":"-100","text":"one","entities":[{"type":"pre","offset":0,"length":3}]}`)
	response, err := http.Post(fake.URL()+"/bot/test/sendMessage", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("first request: %v", err)
	}
	if response.StatusCode != http.StatusOK {
		response.Body.Close()
		t.Fatalf("first status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	response.Body.Close()
	response, err = http.Post(fake.URL()+"/bot/test/sendMessage", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("exhausted request: %v", err)
	}
	if response.StatusCode != http.StatusInternalServerError {
		response.Body.Close()
		t.Fatalf("exhausted status = %d, want %d", response.StatusCode, http.StatusInternalServerError)
	}
	response.Body.Close()

	requests, _ := fake.Requests()
	if len(requests) != 2 || len(requests[0].Entities) != 1 {
		t.Fatalf("recorded requests = %#v", requests)
	}
	requests[0].Text = "mutated"
	requests[0].Entities[0].Type = "mutated"
	again, _ := fake.Requests()
	if again[0].Text != "one" || again[0].Entities[0].Type != "pre" {
		t.Fatalf("request snapshot was not deep copied: %#v", again[0])
	}
}

func telegramSuccess(messageID int64) scriptedTelegramResponse {
	return scriptedTelegramResponse{
		Status: http.StatusOK,
		Body:   fmt.Sprintf("{\"ok\":true,\"result\":{\"message_id\":%d}}", messageID),
	}
}

func telegramRejection(code int) scriptedTelegramResponse {
	return scriptedTelegramResponse{
		Status: http.StatusOK,
		Body:   fmt.Sprintf("{\"ok\":false,\"error_code\":%d,\"description\":\"test rejection\"}", code),
	}
}
