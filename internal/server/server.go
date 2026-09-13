package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"gist-api/internal/github"
)

type GistLister interface {
	ListUserGists(ctx context.Context, username string) ([]github.Gist, error)
}

type Server struct {
	gists  GistLister
	logger *slog.Logger
}

func New(gists GistLister, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{gists: gists, logger: logger}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /{username}", s.listGists)
	return mux
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) listGists(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	gists, err := s.gists.ListUserGists(r.Context(), username)
	if err != nil {
		status := statusCode(err)
		s.logger.Error("list gists failed", "username", username, "status", status, "err", err)
		writeJSON(w, status, map[string]string{"error": errorMessage(err)})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"username": username,
		"count":    len(gists),
		"gists":    gists,
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func statusCode(err error) int {
	switch {
	case errors.Is(err, github.ErrInvalidUsername):
		return http.StatusBadRequest
	case errors.Is(err, github.ErrUserNotFound):
		return http.StatusNotFound
	default:
		return http.StatusBadGateway
	}
}

func errorMessage(err error) string {
	switch {
	case errors.Is(err, github.ErrInvalidUsername):
		return "invalid username"
	case errors.Is(err, github.ErrUserNotFound):
		return "user not found"
	default:
		return "failed to fetch gists"
	}
}
