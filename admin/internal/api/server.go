package api

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

	"github.com/anadubesko/go-do-parser/admin/internal/auth"
	"github.com/anadubesko/go-do-parser/admin/internal/db"
)

//go:embed web/*
var webFS embed.FS

type Server struct {
	db   *db.DB
	auth *auth.Service
}

func New(database *db.DB, authService *auth.Service) *Server {
	return &Server{db: database, auth: authService}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("POST /api/login", s.handleLogin)
	mux.Handle("GET /api/stats", s.authRequired(http.HandlerFunc(s.handleStats)))
	mux.Handle("GET /api/channels", s.authRequired(http.HandlerFunc(s.handleChannels)))
	mux.Handle("GET /api/vacancies", s.authRequired(http.HandlerFunc(s.handleVacancies)))
	mux.Handle("GET /api/job-seekers", s.authRequired(http.HandlerFunc(s.handleJobSeekers)))

	webRoot, _ := fs.Sub(webFS, "web")
	fileServer := http.FileServer(http.FS(webRoot))
	mux.Handle("GET /{$}", fileServer)
	mux.Handle("GET /assets/{path...}", fileServer)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	token, err := s.auth.Login(strings.TrimSpace(req.Password))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid password")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.db.Stats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) handleChannels(w http.ResponseWriter, r *http.Request) {
	kind := strings.TrimSpace(r.URL.Query().Get("type"))
	var channels []string
	var err error
	switch kind {
	case "vacancies":
		channels, err = s.db.ListVacancyChannels(r.Context())
	case "seekers":
		channels, err = s.db.ListJobSeekerChannels(r.Context())
	default:
		writeError(w, http.StatusBadRequest, "invalid type")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if channels == nil {
		channels = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"channels": channels})
}

func (s *Server) handleVacancies(w http.ResponseWriter, r *http.Request) {
	limit, offset := pageParams(r)
	filter := listFilterParams(r)
	items, total, err := s.db.ListVacancies(r.Context(), filter, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	writeJSON(w, http.StatusOK, pageResponse(items, total, limit, offset))
}

func (s *Server) handleJobSeekers(w http.ResponseWriter, r *http.Request) {
	limit, offset := pageParams(r)
	filter := listFilterParams(r)
	items, total, err := s.db.ListJobSeekers(r.Context(), filter, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	writeJSON(w, http.StatusOK, pageResponse(items, total, limit, offset))
}

func (s *Server) authRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if err := s.auth.Validate(token); err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func bearerToken(r *http.Request) string {
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
	}
	return ""
}

func listFilterParams(r *http.Request) db.ListFilter {
	q := r.URL.Query()
	return db.ListFilter{
		Search:   strings.TrimSpace(q.Get("q")),
		Channel:  strings.TrimSpace(q.Get("channel")),
		HasDM:    strings.TrimSpace(q.Get("has_dm")),
		DateFrom: strings.TrimSpace(q.Get("date_from")),
		DateTo:   strings.TrimSpace(q.Get("date_to")),
	}
}

func pageParams(r *http.Request) (limit, offset int) {
	limit = 50
	offset = 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			offset = n
		}
	}
	return limit, offset
}

func pageResponse(items any, total int64, limit, offset int) map[string]any {
	return map[string]any{
		"items":  items,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
