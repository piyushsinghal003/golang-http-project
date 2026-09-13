package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	DefaultAPIURL = "https://api.github.com"
	APIVersion    = "2022-11-28"
	maxPages      = 10
	perPage       = 100
)

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,37}[A-Za-z0-9])?$`)

var (
	ErrInvalidUsername = errors.New("invalid username")
	ErrUserNotFound    = errors.New("user not found")
)

type Gist struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	HTMLURL     string    `json:"html_url"`
	Public      bool      `json:"public"`
	Files       []string  `json:"files"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewClient(baseURL, token string) *Client {
	if baseURL == "" {
		baseURL = DefaultAPIURL
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) ListUserGists(ctx context.Context, username string) ([]Gist, error) {
	if !validUsername(username) {
		return nil, ErrInvalidUsername
	}

	var all []Gist
	next := fmt.Sprintf("%s/users/%s/gists?per_page=%d&page=1", c.baseURL, url.PathEscape(username), perPage)

	for page := 0; page < maxPages && next != ""; page++ {
		gists, nextURL, err := c.fetchPage(ctx, next)
		if err != nil {
			return nil, err
		}
		all = append(all, gists...)
		next = nextURL
	}

	if all == nil {
		all = []Gist{}
	}
	return all, nil
}

func (c *Client) fetchPage(ctx context.Context, pageURL string) ([]Gist, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", APIVersion)
	req.Header.Set("User-Agent", "gist-list-api")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, "", err
	}

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, "", ErrUserNotFound
	default:
		return nil, "", fmt.Errorf("github api returned %d", resp.StatusCode)
	}

	var raw []githubGist
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, "", fmt.Errorf("decode github response: %w", err)
	}

	gists := make([]Gist, 0, len(raw))
	for _, item := range raw {
		files := make([]string, 0, len(item.Files))
		for name := range item.Files {
			files = append(files, name)
		}
		sort.Strings(files)
		gists = append(gists, Gist{
			ID:          item.ID,
			Description: item.Description,
			HTMLURL:     item.HTMLURL,
			Public:      item.Public,
			Files:       files,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
		})
	}

	return gists, nextLink(resp.Header.Get("Link")), nil
}

type githubGist struct {
	ID          string              `json:"id"`
	Description string              `json:"description"`
	HTMLURL     string              `json:"html_url"`
	Public      bool                `json:"public"`
	Files       map[string]struct{} `json:"files"`
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
}

func validUsername(username string) bool {
	return usernamePattern.MatchString(username)
}

func nextLink(header string) string {
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if !strings.Contains(part, `rel="next"`) {
			continue
		}
		start := strings.Index(part, "<")
		end := strings.Index(part, ">")
		if start >= 0 && end > start {
			return part[start+1 : end]
		}
	}
	return ""
}
