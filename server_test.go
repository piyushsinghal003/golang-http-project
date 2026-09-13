package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gist-api/internal/github"
	"gist-api/internal/server"
)

func TestListGistsOctocat(t *testing.T) {
	created := time.Date(2010, 4, 14, 2, 15, 15, 0, time.UTC)
	updated := time.Date(2011, 6, 20, 11, 34, 15, 0, time.UTC)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/octocat/gists" {
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if got := r.Header.Get("Accept"); got != "application/vnd.github+json" {
			t.Errorf("Accept header = %q", got)
		}
		if got := r.Header.Get("X-GitHub-Api-Version"); got != github.APIVersion {
			t.Errorf("API version = %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"id":          "aa5a315d61ae9438b18d",
				"description": "Hello World Examples",
				"html_url":    "https://gist.github.com/aa5a315d61ae9438b18d",
				"public":      true,
				"created_at":  created,
				"updated_at":  updated,
				"files": map[string]any{
					"hello_world.rb": map[string]any{"filename": "hello_world.rb"},
				},
			},
		})
	}))
	t.Cleanup(upstream.Close)

	api := httptest.NewServer(server.New(github.NewClient(upstream.URL, ""), nil).Handler())
	t.Cleanup(api.Close)

	resp, err := http.Get(api.URL + "/octocat")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var body struct {
		Username string        `json:"username"`
		Count    int           `json:"count"`
		Gists    []github.Gist `json:"gists"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body.Username != "octocat" {
		t.Errorf("username = %q", body.Username)
	}
	if body.Count != 1 || len(body.Gists) != 1 {
		t.Fatalf("got %d gists, want 1", body.Count)
	}
	gist := body.Gists[0]
	if gist.ID != "aa5a315d61ae9438b18d" {
		t.Errorf("id = %q", gist.ID)
	}
	if gist.Description != "Hello World Examples" {
		t.Errorf("description = %q", gist.Description)
	}
	if gist.HTMLURL != "https://gist.github.com/aa5a315d61ae9438b18d" {
		t.Errorf("html_url = %q", gist.HTMLURL)
	}
	if len(gist.Files) != 1 || gist.Files[0] != "hello_world.rb" {
		t.Errorf("files = %v", gist.Files)
	}
}

func TestListGistsUserNotFound(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	}))
	t.Cleanup(upstream.Close)

	api := httptest.NewServer(server.New(github.NewClient(upstream.URL, ""), nil).Handler())
	t.Cleanup(api.Close)

	resp, err := http.Get(api.URL + "/octocat")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestListGistsInvalidUsername(t *testing.T) {
	api := httptest.NewServer(server.New(github.NewClient("http://example.invalid", ""), nil).Handler())
	t.Cleanup(api.Close)

	resp, err := http.Get(api.URL + "/bad_user!")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestHealthz(t *testing.T) {
	api := httptest.NewServer(server.New(github.NewClient("http://example.invalid", ""), nil).Handler())
	t.Cleanup(api.Close)

	resp, err := http.Get(api.URL + "/healthz")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestListGistsPagination(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		if page == "" || page == "1" {
			w.Header().Set("Link", `<http://`+r.Host+`/users/octocat/gists?per_page=100&page=2>; rel="next"`)
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "gist-1", "html_url": "https://gist.github.com/1", "public": true, "files": map[string]any{}},
			})
			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": "gist-2", "html_url": "https://gist.github.com/2", "public": true, "files": map[string]any{}},
		})
	}))
	t.Cleanup(upstream.Close)

	api := httptest.NewServer(server.New(github.NewClient(upstream.URL, ""), nil).Handler())
	t.Cleanup(api.Close)

	resp, err := http.Get(api.URL + "/octocat")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var body struct {
		Count int           `json:"count"`
		Gists []github.Gist `json:"gists"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Count != 2 {
		t.Fatalf("count = %d, want 2", body.Count)
	}
	if body.Gists[0].ID != "gist-1" || body.Gists[1].ID != "gist-2" {
		t.Fatalf("gists = %+v", body.Gists)
	}
}
