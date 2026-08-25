package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/archive"
	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/persistence"
)

func TestStrictJSONAndNormalWorkflow(t *testing.T) {
	repository, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	server := httptest.NewServer(New(archive.NewService(repository, time.Now, &archive.RandomIDGenerator{})).Handler())
	defer server.Close()
	invalid := []byte(`{"idempotencyKey":"bad","unknown":true}`)
	response, err := http.Post(server.URL+"/api/v1/archive-tasks", "application/json", bytes.NewReader(invalid))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("未知字段应返回 400，实际为 %d", response.StatusCode)
	}
	ctx := context.Background()
	if err := RunSelfCheck(ctx, server.URL); err != nil {
		t.Fatal(err)
	}
	listResponse, err := http.Get(server.URL + "/api/v1/archive-tasks")
	if err != nil {
		t.Fatal(err)
	}
	defer listResponse.Body.Close()
	var listed struct {
		Count int `json:"count"`
	}
	if err := json.NewDecoder(listResponse.Body).Decode(&listed); err != nil || listed.Count != 1 {
		t.Fatalf("任务列表异常：%v %#v", err, listed)
	}
}
