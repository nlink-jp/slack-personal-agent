// Package slack provides a Slack Web API client for User Token operations.
package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const baseURL = "https://slack.com/api"

// Client is a Slack Web API client using a User Token.
type Client struct {
	token      string
	httpClient *http.Client
}

// NewClient creates a new Slack API client with the given user token.
func NewClient(token string) *Client {
	return &Client{
		token: token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Channel represents a Slack channel.
type Channel struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	IsPrivate  bool   `json:"is_private"`
	IsMember   bool   `json:"is_member"`
	NumMembers int    `json:"num_members"`
	Topic      struct {
		Value string `json:"value"`
	} `json:"topic"`
	Purpose struct {
		Value string `json:"value"`
	} `json:"purpose"`
}

// Message represents a Slack message.
type Message struct {
	Type      string `json:"type"`
	SubType   string `json:"subtype,omitempty"`
	User      string `json:"user"`
	Text      string `json:"text"`
	Ts        string `json:"ts"`
	ThreadTs  string `json:"thread_ts,omitempty"`
	ReplyCount int   `json:"reply_count,omitempty"`
	Files     []File `json:"files,omitempty"`
}

// File represents a Slack file attachment.
type File struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	MimeType string `json:"mimetype"`
	Size     int    `json:"size"`
	URLPrivate string `json:"url_private"`
}

// User represents a Slack user.
type User struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	RealName string `json:"real_name"`
}

// slackResponse is the common envelope for Slack API responses.
type slackResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// channelsResponse is the response from conversations.list.
type channelsResponse struct {
	slackResponse
	Channels []Channel `json:"channels"`
	Meta     struct {
		NextCursor string `json:"next_cursor"`
	} `json:"response_metadata"`
}

// historyResponse is the response from conversations.history.
type historyResponse struct {
	slackResponse
	Messages []Message `json:"messages"`
	HasMore  bool      `json:"has_more"`
	Meta     struct {
		NextCursor string `json:"next_cursor"`
	} `json:"response_metadata"`
}

// userResponse is the response from users.info.
type userResponse struct {
	slackResponse
	User User `json:"user"`
}

// postResponse is the response from chat.postMessage.
type postResponse struct {
	slackResponse
	Ts      string `json:"ts"`
	Channel string `json:"channel"`
}

// ListChannels returns channels the user has joined or can read.
// Excludes DMs and archived channels.
func (c *Client) ListChannels(ctx context.Context) ([]Channel, error) {
	var all []Channel
	cursor := ""

	for {
		params := url.Values{
			"types":            {"public_channel,private_channel"},
			"exclude_archived": {"true"},
			"limit":            {"200"},
		}
		if cursor != "" {
			params.Set("cursor", cursor)
		}

		var resp channelsResponse
		if err := c.get(ctx, "conversations.list", params, &resp); err != nil {
			return nil, err
		}

		all = append(all, resp.Channels...)

		cursor = resp.Meta.NextCursor
		if cursor == "" {
			break
		}
	}
	return all, nil
}

// FetchHistory retrieves messages from a channel.
// If oldest is non-empty, only messages after that timestamp are returned.
// Returns at most limit messages (max 1000).
func (c *Client) FetchHistory(ctx context.Context, channelID, oldest string, limit int) ([]Message, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}

	params := url.Values{
		"channel": {channelID},
		"limit":   {fmt.Sprintf("%d", limit)},
	}
	if oldest != "" {
		params.Set("oldest", oldest)
	}

	var resp historyResponse
	if err := c.get(ctx, "conversations.history", params, &resp); err != nil {
		return nil, err
	}

	return resp.Messages, nil
}

// GetUser retrieves user information by ID.
func (c *Client) GetUser(ctx context.Context, userID string) (*User, error) {
	params := url.Values{
		"user": {userID},
	}

	var resp userResponse
	if err := c.get(ctx, "users.info", params, &resp); err != nil {
		return nil, err
	}

	return &resp.User, nil
}

// PostMessage posts a message to a channel.
func (c *Client) PostMessage(ctx context.Context, channelID, text, threadTs string) (string, error) {
	params := url.Values{
		"channel": {channelID},
		"text":    {text},
	}
	if threadTs != "" {
		params.Set("thread_ts", threadTs)
	}

	var resp postResponse
	if err := c.post(ctx, "chat.postMessage", params, &resp); err != nil {
		return "", err
	}

	return resp.Ts, nil
}

// PostProxyMessage posts a MITL proxy response with a system signature appended.
// The signature identifies the message as system-generated, not user-authored.
func (c *Client) PostProxyMessage(ctx context.Context, channelID, text, threadTs, signature string) (string, error) {
	signed := text + "\n\n" + signature
	return c.PostMessage(ctx, channelID, signed, threadTs)
}

func (c *Client) get(ctx context.Context, method string, params url.Values, result interface{}) error {
	reqURL := fmt.Sprintf("%s/%s?%s", baseURL, method, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	return c.do(req, result)
}

func (c *Client) post(ctx context.Context, method string, params url.Values, result interface{}) error {
	reqURL := fmt.Sprintf("%s/%s", baseURL, method)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, strings.NewReader(params.Encode()))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return c.do(req, result)
}

func (c *Client) do(req *http.Request, result interface{}) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return &RateLimitError{RetryAfter: resp.Header.Get("Retry-After")}
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack API %s: HTTP %d", req.URL.Path, resp.StatusCode)
	}

	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	// Check Slack's ok field
	var base slackResponse
	if err := json.Unmarshal(body, &base); err == nil && !base.OK {
		return fmt.Errorf("slack API %s: %s", req.URL.Path, base.Error)
	}

	return nil
}

// RateLimitError is returned when Slack responds with 429.
type RateLimitError struct {
	RetryAfter string
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limited (retry after %s)", e.RetryAfter)
}
