package images

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/xingkaixin/pic-create/internal/oauth"
)

type Client struct {
	oauth *oauth.Client
}

type Options struct {
	Model         string
	Prompt        string
	Size          string
	Quality       string
	Format        string
	InputFidelity string
}

type response struct {
	Data []struct {
		Base64 string `json:"b64_json"`
	} `json:"data"`
}

func NewClient(oauthClient *oauth.Client) *Client {
	return &Client{oauth: oauthClient}
}

func (c *Client) Generate(ctx context.Context, options Options) ([]byte, error) {
	if err := validateFormat(options); err != nil {
		return nil, err
	}
	payload := imagePayload(options)
	var result response
	if err := c.oauth.Post(ctx, "/images/generations", payload, &result); err != nil {
		return nil, err
	}
	return decodeFirstImage(result)
}

func (c *Client) Edit(ctx context.Context, imagePath string, options Options) ([]byte, error) {
	if err := validateFormat(options); err != nil {
		return nil, err
	}
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return nil, fmt.Errorf("read input image: %w", err)
	}
	payload := imagePayload(options)
	payload["images"] = []map[string]string{{"image_url": dataURL(imageData)}}
	if options.InputFidelity != "" {
		payload["input_fidelity"] = options.InputFidelity
	}
	var result response
	if err := c.oauth.Post(ctx, "/images/edits", payload, &result); err != nil {
		return nil, err
	}
	return decodeFirstImage(result)
}

func validateFormat(options Options) error {
	if options.Format != "png" {
		return errors.New("ChatGPT OAuth only supports PNG output; use --format png")
	}
	return nil
}

func imagePayload(options Options) map[string]any {
	payload := map[string]any{
		"model":  options.Model,
		"prompt": options.Prompt,
		"n":      1,
	}
	if options.Size != "auto" {
		payload["size"] = options.Size
	}
	if options.Quality != "auto" {
		payload["quality"] = options.Quality
	}
	return payload
}

func dataURL(data []byte) string {
	contentType := http.DetectContentType(data)
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data)
}

func decodeFirstImage(result response) ([]byte, error) {
	if len(result.Data) == 0 {
		return nil, errors.New("image response contains no image data")
	}
	if result.Data[0].Base64 == "" {
		return nil, errors.New("image response contains no b64_json")
	}
	data, err := base64.StdEncoding.DecodeString(result.Data[0].Base64)
	if err != nil {
		return nil, fmt.Errorf("decode image data: %w", err)
	}
	return data, nil
}

func WriteFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write output image: %w", err)
	}
	return nil
}
