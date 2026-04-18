package data

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	http       *http.Client
	sessionKey string
	baseURL    string
}

func NewClient(sessionKey string) *Client {
	return &Client{
		http:       &http.Client{Timeout: 2 * time.Minute},
		sessionKey: sessionKey,
		baseURL:    "https://claude.ai",
	}
}

type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("claude.ai API error %d: %s", e.StatusCode, e.Message)
}

func (e *APIError) IsAuth() bool {
	return e.StatusCode == 401 || e.StatusCode == 403
}

func (e *APIError) IsRateLimit() bool {
	return e.StatusCode == 429
}

type apiOrganization struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

type APIConversationListItem struct {
	UUID      string    `json:"uuid"`
	Name      string    `json:"name"`
	Summary   string    `json:"summary"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (c *Client) doRequest(ctx context.Context, path string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Cookie", "sessionKey="+c.sessionKey)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return &APIError{StatusCode: resp.StatusCode, Message: string(body)}
	}

	return json.NewDecoder(resp.Body).Decode(target)
}

func (c *Client) GetOrganizationID(ctx context.Context) (string, error) {
	var orgs []apiOrganization
	if err := c.doRequest(ctx, "/api/organizations", &orgs); err != nil {
		return "", fmt.Errorf("fetching organizations: %w", err)
	}
	if len(orgs) == 0 {
		return "", fmt.Errorf("no organizations found")
	}
	return orgs[0].UUID, nil
}

func (c *Client) ListConversations(ctx context.Context, orgID string) ([]APIConversationListItem, error) {
	path := fmt.Sprintf("/api/organizations/%s/chat_conversations", orgID)
	var convs []APIConversationListItem
	if err := c.doRequest(ctx, path, &convs); err != nil {
		return nil, fmt.Errorf("listing conversations: %w", err)
	}
	return convs, nil
}

func (c *Client) GetConversation(ctx context.Context, orgID, convID string) (rawConversation, error) {
	path := fmt.Sprintf("/api/organizations/%s/chat_conversations/%s", orgID, convID)
	var conv rawConversation
	if err := c.doRequest(ctx, path, &conv); err != nil {
		return conv, fmt.Errorf("fetching conversation %s: %w", convID, err)
	}
	return conv, nil
}
