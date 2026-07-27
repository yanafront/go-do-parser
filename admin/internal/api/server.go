package api

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

	"github.com/anadubesko/go-do-parser/admin/internal/auth"
	"github.com/anadubesko/go-do-parser/admin/internal/db"
	"github.com/anadubesko/go-do-parser/admin/internal/seekermsg"
)

//go:embed web/*
var webFS embed.FS

type ctxKey int

const nicknameKey ctxKey = 1

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
	mux.Handle("GET /api/seeker-agents", s.authRequired(http.HandlerFunc(s.handleSeekerAgents)))
	mux.Handle("GET /api/channels", s.authRequired(http.HandlerFunc(s.handleChannels)))
	mux.Handle("GET /api/vacancies", s.authRequired(http.HandlerFunc(s.handleVacancies)))
	mux.Handle("PATCH /api/vacancies/{id}/dm", s.authRequired(http.HandlerFunc(s.handleUpdateVacancyDM)))
	mux.Handle("GET /api/job-seekers", s.authRequired(http.HandlerFunc(s.handleJobSeekers)))
	mux.Handle("PATCH /api/job-seekers/{id}/dm", s.authRequired(http.HandlerFunc(s.handleUpdateJobSeekerDM)))
	mux.Handle("GET /api/onliner-posts", s.authRequired(http.HandlerFunc(s.handleOnlinerPosts)))
	mux.Handle("PATCH /api/onliner-posts/{id}/dm", s.authRequired(http.HandlerFunc(s.handleUpdateOnlinerDM)))

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
		Nickname string `json:"nickname"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	token, nick, err := s.auth.Login(strings.TrimSpace(req.Password), req.Nickname)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidNickname) {
			writeError(w, http.StatusBadRequest, "nickname required")
			return
		}
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token, "nickname": nick})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.db.Stats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) handleSeekerAgents(w http.ResponseWriter, r *http.Request) {
	statuses, err := s.db.ListSeekerAgentStatuses(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	blocks, err := s.db.ListSeekerAgentBlocks(r.Context(), 20)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if statuses == nil {
		statuses = []db.SeekerAgentStatus{}
	}
	if blocks == nil {
		blocks = []db.SeekerAgentBlock{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"agents": statuses,
		"blocks": blocks,
	})
}

func (s *Server) handleChannels(w http.ResponseWriter, r *http.Request) {
	kind := strings.TrimSpace(r.URL.Query().Get("type"))
	var channels []db.Channel
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
		channels = []db.Channel{}
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

func (s *Server) handleOnlinerPosts(w http.ResponseWriter, r *http.Request) {
	limit, offset := pageParams(r)
	filter := onlinerFilterParams(r)
	items, total, err := s.db.ListOnlinerPosts(r.Context(), filter, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	writeJSON(w, http.StatusOK, pageResponse(items, total, limit, offset))
}

func onlinerFilterParams(r *http.Request) db.OnlinerListFilter {
	q := r.URL.Query()
	return db.OnlinerListFilter{
		Search:        strings.TrimSpace(q.Get("q")),
		HasContact:    strings.TrimSpace(q.Get("has_contact")),
		MessageStatus: strings.TrimSpace(q.Get("message_status")),
		DateFrom:      strings.TrimSpace(q.Get("date_from")),
		DateTo:        strings.TrimSpace(q.Get("date_to")),
		SortBy:        strings.TrimSpace(q.Get("sort_by")),
		SortDir:       strings.TrimSpace(q.Get("sort")),
	}
}

func (s *Server) handleJobSeekers(w http.ResponseWriter, r *http.Request) {
	limit, offset := pageParams(r)
	filter := listFilterParams(r)
	items, total, err := s.db.ListJobSeekers(r.Context(), filter, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	for i := range items {
		if items[i].DMMessage != nil && strings.TrimSpace(*items[i].DMMessage) != "" {
			continue
		}
		if items[i].DMContact != nil {
			c := strings.TrimSpace(*items[i].DMContact)
			if c != "" && c != "none" {
				continue
			}
		}
		link := ""
		if items[i].SourceMessageLink != nil {
			link = *items[i].SourceMessageLink
		}
		preview := seekermsg.Preview(items[i].SourceChannel, link, items[i].SourceMessageID)
		items[i].DMMessage = &preview
	}
	writeJSON(w, http.StatusOK, pageResponse(items, total, limit, offset))
}

func (s *Server) handleUpdateVacancyDM(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	item, err := s.db.UpdateVacancyDMStatus(r.Context(), id, req.Status, nicknameFrom(r))
	if err != nil {
		msg := err.Error()
		if msg == "not found" {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		if msg == "invalid status" {
			writeError(w, http.StatusBadRequest, "invalid status")
			return
		}
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleUpdateJobSeekerDM(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	item, err := s.db.UpdateJobSeekerDMStatus(r.Context(), id, req.Status, nicknameFrom(r))
	if err != nil {
		msg := err.Error()
		if msg == "not found" {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		if msg == "invalid status" {
			writeError(w, http.StatusBadRequest, "invalid status")
			return
		}
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleUpdateOnlinerDM(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	item, err := s.db.UpdateOnlinerDMStatus(r.Context(), id, req.Status, nicknameFrom(r))
	if err != nil {
		msg := err.Error()
		if msg == "not found" {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		if msg == "invalid status" {
			writeError(w, http.StatusBadRequest, "invalid status")
			return
		}
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) authRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		claims, err := s.auth.Validate(token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		ctx := context.WithValue(r.Context(), nicknameKey, claims.Nickname)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func nicknameFrom(r *http.Request) string {
	v, _ := r.Context().Value(nicknameKey).(string)
	return v
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
	search := strings.TrimSpace(q.Get("q"))
	search = strings.TrimPrefix(search, "@")
	return db.ListFilter{
		Search:        search,
		Channel:       strings.TrimSpace(strings.TrimPrefix(q.Get("channel"), "@")),
		HasDM:         strings.TrimSpace(q.Get("has_dm")),
		MessageStatus: strings.TrimSpace(q.Get("message_status")),
		DateFrom:      strings.TrimSpace(q.Get("date_from")),
		DateTo:        strings.TrimSpace(q.Get("date_to")),
		SortBy:        strings.TrimSpace(q.Get("sort_by")),
		SortDir:       strings.TrimSpace(q.Get("sort")),
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
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
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
