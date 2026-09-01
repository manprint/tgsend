//go:build e2e

package e2e

import (
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
	requests := append([]telegramRequest(nil), fake.requests...)
	paths := append([]string(nil), fake.paths...)
	return requests, paths
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
