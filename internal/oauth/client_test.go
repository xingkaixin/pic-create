package oauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestPostRefreshesUnauthorizedSession(t *testing.T) {
	directory := t.TempDir()
	authPath := filepath.Join(directory, "auth.json")
	writeAuthFile(t, authPath, map[string]any{
		"auth_mode": "chatgpt",
		"preserved": true,
		"tokens": map[string]any{
			"access_token":  "old-access",
			"refresh_token": "refresh",
			"account_id":    "account-1",
		},
	})

	imageRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/images/generations":
			imageRequests++
			if request.Header.Get("Authorization") == "Bearer old-access" {
				http.Error(writer, `{"error":{"message":"expired"}}`, http.StatusUnauthorized)
				return
			}
			if request.Header.Get("chatgpt-account-id") != "account-1" {
				t.Errorf("unexpected account header: %q", request.Header.Get("chatgpt-account-id"))
			}
			writer.Header().Set("Content-Type", "application/json")
			writer.Write([]byte(`{"data":[{"b64_json":"aW1hZ2U="}]}`))
		case "/oauth/token":
			writer.Header().Set("Content-Type", "application/json")
			writer.Write([]byte(`{"access_token":"new-access"}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	client := NewClientWithEndpoints(NewStore(authPath), server.URL, server.URL)
	var result map[string]any
	if err := client.Post(context.Background(), "/images/generations", map[string]string{"prompt": "test"}, &result); err != nil {
		t.Fatalf("post image request: %v", err)
	}
	if imageRequests != 2 {
		t.Fatalf("got %d image requests, want 2", imageRequests)
	}

	data, err := os.ReadFile(authPath)
	if err != nil {
		t.Fatal(err)
	}
	var saved map[string]any
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if saved["preserved"] != true {
		t.Fatal("session update removed an unrelated field")
	}
	tokens := saved["tokens"].(map[string]any)
	if tokens["access_token"] != "new-access" || tokens["refresh_token"] != "refresh" {
		t.Fatalf("unexpected saved tokens: %#v", tokens)
	}
	info, err := os.Stat(authPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("OAuth file mode is %o, want 600", info.Mode().Perm())
	}
}

func TestDeriveAccountID(t *testing.T) {
	token := "header.eyJodHRwczovL2FwaS5vcGVuYWkuY29tL2F1dGgiOnsiY2hhdGdwdF9hY2NvdW50X2lkIjoiYWNjb3VudC0yIn19.signature"
	if got := deriveAccountID(token); got != "account-2" {
		t.Fatalf("got %q, want account-2", got)
	}
}

func writeAuthFile(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
