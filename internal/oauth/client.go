package oauth

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultAuthIssuer = "https://auth.openai.com"
	defaultCodexBase  = "https://chatgpt.com/backend-api/codex"
	oauthClientID     = "app_EMoamEEZ73f0CkXaXp7hrann"
	requestTimeout    = 10 * time.Minute
)

var ErrNoSession = errors.New("no Codex OAuth session found; run `codex login` first")

type Tokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	AccountID    string `json:"account_id,omitempty"`
}

type Store struct {
	path string
}

func CodexStore() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("find home directory: %w", err)
	}
	return &Store{path: filepath.Join(home, ".codex", "auth.json")}, nil
}

func NewStore(path string) *Store {
	return &Store{path: path}
}

func (s *Store) Load() (Tokens, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return Tokens{}, ErrNoSession
	}
	if err != nil {
		return Tokens{}, fmt.Errorf("read OAuth session: %w", err)
	}

	var authFile struct {
		Tokens Tokens `json:"tokens"`
	}
	if err := json.Unmarshal(data, &authFile); err != nil {
		return Tokens{}, fmt.Errorf("parse OAuth session: %w", err)
	}
	if authFile.Tokens.AccessToken == "" {
		return Tokens{}, ErrNoSession
	}
	return authFile.Tokens, nil
}

func (s *Store) Save(tokens Tokens) error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return fmt.Errorf("read OAuth session before update: %w", err)
	}

	var authFile map[string]any
	if err := json.Unmarshal(data, &authFile); err != nil {
		return fmt.Errorf("parse OAuth session before update: %w", err)
	}
	currentTokens, _ := authFile["tokens"].(map[string]any)
	if currentTokens == nil {
		currentTokens = make(map[string]any)
	}
	currentTokens["access_token"] = tokens.AccessToken
	setIfPresent(currentTokens, "refresh_token", tokens.RefreshToken)
	setIfPresent(currentTokens, "id_token", tokens.IDToken)
	setIfPresent(currentTokens, "account_id", tokens.AccountID)
	authFile["tokens"] = currentTokens
	authFile["last_refresh"] = time.Now().UTC().Format(time.RFC3339)

	updated, err := json.MarshalIndent(authFile, "", "  ")
	if err != nil {
		return fmt.Errorf("encode OAuth session: %w", err)
	}
	updated = append(updated, '\n')
	return writePrivateFile(s.path, updated)
}

func setIfPresent(values map[string]any, key, value string) {
	if value != "" {
		values[key] = value
	}
}

func writePrivateFile(path string, data []byte) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".auth-*.json")
	if err != nil {
		return fmt.Errorf("create OAuth session update: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("secure OAuth session update: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return fmt.Errorf("write OAuth session update: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close OAuth session update: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace OAuth session: %w", err)
	}
	return nil
}

type Client struct {
	httpClient *http.Client
	store      *Store
	authIssuer string
	codexBase  string
}

func NewClient(store *Store) *Client {
	return NewClientWithEndpoints(store, defaultAuthIssuer, defaultCodexBase)
}

func NewClientWithEndpoints(store *Store, authIssuer, codexBase string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: requestTimeout},
		store:      store,
		authIssuer: strings.TrimRight(authIssuer, "/"),
		codexBase:  strings.TrimRight(codexBase, "/"),
	}
}

func (c *Client) Post(ctx context.Context, path string, payload any, result any) error {
	tokens, err := c.store.Load()
	if err != nil {
		return err
	}
	if tokenNearExpiry(tokens.AccessToken) {
		tokens, err = c.refresh(ctx, tokens)
		if err != nil {
			return err
		}
	}

	status, err := c.post(ctx, path, payload, tokens, result)
	if status != http.StatusUnauthorized {
		return err
	}
	tokens, refreshErr := c.refresh(ctx, tokens)
	if refreshErr != nil {
		return refreshErr
	}
	_, err = c.post(ctx, path, payload, tokens, result)
	return err
}

func (c *Client) post(ctx context.Context, path string, payload any, tokens Tokens, result any) (int, error) {
	accountID := tokens.AccountID
	if accountID == "" {
		accountID = deriveAccountID(tokens.IDToken, tokens.AccessToken)
	}
	if accountID == "" {
		return 0, errors.New("OAuth session has no ChatGPT account ID; run `codex login` again")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("encode image request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.codexBase+path, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("create image request: %w", err)
	}
	setCodexHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	req.Header.Set("chatgpt-account-id", accountID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("send image request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, responseError(resp)
	}
	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return resp.StatusCode, fmt.Errorf("decode image response: %w", err)
	}
	return resp.StatusCode, nil
}

func (c *Client) refresh(ctx context.Context, tokens Tokens) (Tokens, error) {
	if tokens.RefreshToken == "" {
		return Tokens{}, errors.New("OAuth session expired without a refresh token; run `codex login` again")
	}
	payload := map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": tokens.RefreshToken,
		"client_id":     oauthClientID,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Tokens{}, fmt.Errorf("encode token refresh: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.authIssuer+"/oauth/token", bytes.NewReader(body))
	if err != nil {
		return Tokens{}, fmt.Errorf("create token refresh: %w", err)
	}
	setCodexHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Tokens{}, fmt.Errorf("refresh OAuth session: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Tokens{}, fmt.Errorf("refresh OAuth session: %w", responseError(resp))
	}

	var refreshed Tokens
	if err := json.NewDecoder(resp.Body).Decode(&refreshed); err != nil {
		return Tokens{}, fmt.Errorf("decode token refresh: %w", err)
	}
	if refreshed.AccessToken == "" {
		return Tokens{}, errors.New("refresh OAuth session: response has no access token")
	}
	if refreshed.RefreshToken == "" {
		refreshed.RefreshToken = tokens.RefreshToken
	}
	if refreshed.IDToken == "" {
		refreshed.IDToken = tokens.IDToken
	}
	if refreshed.AccountID == "" {
		refreshed.AccountID = tokens.AccountID
	}
	if refreshed.AccountID == "" {
		refreshed.AccountID = deriveAccountID(refreshed.IDToken, refreshed.AccessToken)
	}
	if err := c.store.Save(refreshed); err != nil {
		return Tokens{}, err
	}
	return refreshed, nil
}

func setCodexHeaders(req *http.Request) {
	req.Header.Set("originator", "codex_cli_rs")
	req.Header.Set("User-Agent", "codex_cli_rs/0.48.0 (Mac OS; arm64) Terminal")
}

func responseError(resp *http.Response) error {
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	detail := strings.TrimSpace(string(body))
	var payload struct {
		Error any `json:"error"`
	}
	if json.Unmarshal(body, &payload) == nil && payload.Error != nil {
		switch value := payload.Error.(type) {
		case string:
			detail = value
		case map[string]any:
			if message, ok := value["message"].(string); ok {
				detail = message
			}
		}
	}
	if detail == "" {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return fmt.Errorf("HTTP %d: %s", resp.StatusCode, detail)
}

func tokenNearExpiry(accessToken string) bool {
	claims := jwtClaims(accessToken)
	expiresAt, ok := claims["exp"].(float64)
	if !ok {
		return false
	}
	return time.Now().Unix() > int64(expiresAt)-60
}

func deriveAccountID(tokens ...string) string {
	for _, token := range tokens {
		claims := jwtClaims(token)
		if auth, ok := claims["https://api.openai.com/auth"].(map[string]any); ok {
			if accountID, ok := auth["chatgpt_account_id"].(string); ok {
				return accountID
			}
		}
		if accountID, ok := claims["chatgpt_account_id"].(string); ok {
			return accountID
		}
	}
	return ""
}

func jwtClaims(token string) map[string]any {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil
	}
	var claims map[string]any
	if json.Unmarshal(payload, &claims) != nil {
		return nil
	}
	return claims
}
