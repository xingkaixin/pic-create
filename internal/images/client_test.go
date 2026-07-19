package images

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/xingkaixin/pic-create/internal/oauth"
)

func TestGenerateUsesOAuthImageRoute(t *testing.T) {
	var payload map[string]any
	server := imageServer(t, "/images/generations", &payload)
	defer server.Close()
	client := testClient(t, server.URL)

	result, err := client.Generate(context.Background(), Options{
		Model:   "gpt-image-2",
		Prompt:  "a skyline",
		Size:    "1536x864",
		Quality: "high",
		Format:  "png",
	})
	if err != nil {
		t.Fatalf("generate image: %v", err)
	}
	if string(result) != "image" {
		t.Fatalf("unexpected image data: %q", result)
	}
	if payload["size"] != "1536x864" || payload["quality"] != "high" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestEditSendsInputAsDataURL(t *testing.T) {
	var payload map[string]any
	server := imageServer(t, "/images/edits", &payload)
	defer server.Close()
	client := testClient(t, server.URL)
	imagePath := filepath.Join(t.TempDir(), "input.png")
	pngHeader := []byte("\x89PNG\r\n\x1a\nimage")
	if err := os.WriteFile(imagePath, pngHeader, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := client.Edit(context.Background(), imagePath, Options{
		Model:         "gpt-image-2",
		Prompt:        "replace background",
		Size:          "auto",
		Quality:       "auto",
		Format:        "png",
		InputFidelity: "high",
	})
	if err != nil {
		t.Fatalf("edit image: %v", err)
	}
	images, ok := payload["images"].([]any)
	if !ok || len(images) != 1 {
		t.Fatalf("unexpected images payload: %#v", payload["images"])
	}
	image := images[0].(map[string]any)
	if image["image_url"] != "data:image/png;base64,iVBORw0KGgppbWFnZQ==" {
		t.Fatalf("unexpected data URL: %q", image["image_url"])
	}
	if payload["input_fidelity"] != "high" {
		t.Fatalf("unexpected input fidelity: %#v", payload)
	}
}

func TestWebPIsRejectedBeforeRequest(t *testing.T) {
	client := &Client{}
	_, err := client.Generate(context.Background(), Options{Format: "webp"})
	if err == nil {
		t.Fatal("expected WebP validation error")
	}
}

func imageServer(t *testing.T, expectedPath string, payload *map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != expectedPath {
			t.Errorf("got path %q, want %q", request.URL.Path, expectedPath)
		}
		if err := json.NewDecoder(request.Body).Decode(payload); err != nil {
			t.Errorf("decode request: %v", err)
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.Write([]byte(`{"data":[{"b64_json":"aW1hZ2U="}]}`))
	}))
}

func testClient(t *testing.T, endpoint string) *Client {
	t.Helper()
	authPath := filepath.Join(t.TempDir(), "auth.json")
	authData := []byte(`{"tokens":{"access_token":"access","account_id":"account"}}`)
	if err := os.WriteFile(authPath, authData, 0o600); err != nil {
		t.Fatal(err)
	}
	return NewClient(oauth.NewClientWithEndpoints(oauth.NewStore(authPath), endpoint, endpoint))
}
