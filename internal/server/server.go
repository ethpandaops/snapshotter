package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/ethpandaops/eth-snapshotter/internal/config"
	"github.com/ethpandaops/eth-snapshotter/internal/db"
	"github.com/ethpandaops/eth-snapshotter/internal/types"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

type Server struct {
	cfg       *config.Config
	db        *db.DB
	getStatus func() *types.SnapshotterStatus
}

func New(cfg *config.Config, database *db.DB, getStatusFn func() *types.SnapshotterStatus) *Server {
	return &Server{
		cfg:       cfg,
		db:        database,
		getStatus: getStatusFn,
	}
}

func (s *Server) Start() error {
	r := mux.NewRouter()
	publicRouter := r.PathPrefix("/api/v1").Subrouter()
	publicRouter.HandleFunc("/runs", s.handleGetRuns).Methods("GET")
	publicRouter.HandleFunc("/status", s.handleGetStatus).Methods("GET")
	publicRouter.HandleFunc("/runs/{id}", s.handleGetRun).Methods("GET")
	publicRouter.HandleFunc("/targets/{id}", s.handleGetTargetSnapshot).Methods("GET")
	publicRouter.HandleFunc("/targets", s.handleGetTargets).Methods("GET")

	// Create a subrouter for authenticated endpoints
	authRouter := r.PathPrefix("/api/v1").Subrouter()
	authRouter.Use(s.authMiddleware)
	authRouter.HandleFunc("/runs/{id}/persist", s.handleSetPersisted).Methods("POST")
	authRouter.HandleFunc("/runs/{id}/unpersist", s.handleSetUnpersisted).Methods("POST")
	authRouter.HandleFunc("/targets/{id}/persist", s.handleSetTargetPersisted).Methods("POST")
	authRouter.HandleFunc("/targets/{id}/unpersist", s.handleSetTargetUnpersisted).Methods("POST")

	listenAddr := s.cfg.Server.ListenAddr
	if listenAddr == "" {
		listenAddr = "0.0.0.0:5001"
	}

	// Log API authentication status
	if s.cfg.Server.Auth.APIToken == "" {
		log.Fatal("API authentication needs to be set - no API token configured")
	}

	log.WithField("addr", listenAddr).Info("starting HTTP server")
	return http.ListenAndServe(listenAddr, r)
}

// authMiddleware is a middleware function that checks for a valid API token
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for token in Authorization header
		token := r.Header.Get("Authorization")
		if token == "" {
			http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
			return
		}

		// Handle "Bearer <token>" format
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}

		// Validate token
		if token != s.cfg.Server.Auth.APIToken {
			http.Error(w, "Invalid API token", http.StatusUnauthorized)
			return
		}

		// Token is valid, proceed to the next handler
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleGetRuns(w http.ResponseWriter, r *http.Request) {
	page := 1
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	limit := 20
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	if limit > 20 {
		limit = 20
	}

	// Get filter parameters
	includeDeleted := false
	if filterStr := r.URL.Query().Get("include_deleted"); filterStr != "" {
		includeDeleted = filterStr == "true"
	}

	onlyPersisted := false
	if filterStr := r.URL.Query().Get("only_persisted"); filterStr != "" {
		onlyPersisted = filterStr == "true"
	}

	offset := (page - 1) * limit
	runs, err := s.db.GetPaginatedRuns(offset, limit, includeDeleted, onlyPersisted)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"page":  page,
		"limit": limit,
		"runs":  runs,
	}); err != nil {
		log.WithError(err).Error("failed to encode runs")
		http.Error(w, "failed to encode runs", http.StatusInternalServerError)
		return
	}
}

func (s *Server) handleGetRun(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid run ID", http.StatusBadRequest)
		return
	}

	run, err := s.db.GetSnapshotRunByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if run == nil {
		http.Error(w, "snapshot run not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(run); err != nil {
		log.WithError(err).Error("failed to encode run")
		http.Error(w, "failed to encode run", http.StatusInternalServerError)
		return
	}
}

func (s *Server) handleSetPersisted(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid run ID", http.StatusBadRequest)
		return
	}

	// Check if the run exists
	run, err := s.db.GetSnapshotRunByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if run == nil {
		http.Error(w, "snapshot run not found", http.StatusNotFound)
		return
	}

	// Set the persisted flag (this also persists all associated targets)
	if err := s.db.SetSnapshotRunPersisted(id, true); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.WithFields(log.Fields{
		"run_id":        id,
		"targets_count": len(run.TargetsSnapshot),
	}).Info("marked run and its targets as persisted")

	// Get the updated run
	run, err = s.db.GetSnapshotRunByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(run); err != nil {
		log.WithError(err).Error("failed to encode run")
		http.Error(w, "failed to encode run", http.StatusInternalServerError)
		return
	}
}

func (s *Server) handleSetUnpersisted(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid run ID", http.StatusBadRequest)
		return
	}

	// Check if the run exists
	run, err := s.db.GetSnapshotRunByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if run == nil {
		http.Error(w, "snapshot run not found", http.StatusNotFound)
		return
	}

	// Set the persisted flag (this also unpersists all associated targets)
	if err := s.db.SetSnapshotRunPersisted(id, false); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.WithFields(log.Fields{
		"run_id":        id,
		"targets_count": len(run.TargetsSnapshot),
	}).Info("marked run and its targets as not persisted")

	// Get the updated run
	run, err = s.db.GetSnapshotRunByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(run); err != nil {
		log.WithError(err).Error("failed to encode run")
		http.Error(w, "failed to encode run", http.StatusInternalServerError)
		return
	}
}

func (s *Server) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	run, err := s.db.GetMostRecentRun()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// create anonymous struct to hide the run object
	resp := struct {
		LatestRun *db.SnapshotRun          `json:"latestRun"`
		Status    *types.SnapshotterStatus `json:"status"`
	}{
		LatestRun: run,
		Status:    s.getStatus(),
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.WithError(err).Error("failed to encode status")
		http.Error(w, "failed to encode status", http.StatusInternalServerError)
		return
	}
}

func (s *Server) handleGetTargetSnapshot(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid target ID", http.StatusBadRequest)
		return
	}

	target, err := s.db.GetTargetSnapshotByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if target == nil {
		http.Error(w, "target snapshot not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(target); err != nil {
		log.WithError(err).Error("failed to encode target snapshot")
		http.Error(w, "failed to encode target snapshot", http.StatusInternalServerError)
		return
	}
}

func (s *Server) handleSetTargetPersisted(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid target ID", http.StatusBadRequest)
		return
	}

	// Check if the target exists
	target, err := s.db.GetTargetSnapshotByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if target == nil {
		http.Error(w, "target snapshot not found", http.StatusNotFound)
		return
	}

	// Set the persisted flag
	if err := s.db.SetTargetSnapshotPersisted(id, true); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get the updated target
	target, err = s.db.GetTargetSnapshotByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(target); err != nil {
		log.WithError(err).Error("failed to encode target snapshot")
		http.Error(w, "failed to encode target snapshot", http.StatusInternalServerError)
		return
	}
}

func (s *Server) handleSetTargetUnpersisted(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid target ID", http.StatusBadRequest)
		return
	}

	// Check if the target exists
	target, err := s.db.GetTargetSnapshotByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if target == nil {
		http.Error(w, "target snapshot not found", http.StatusNotFound)
		return
	}

	// Set the persisted flag
	if err := s.db.SetTargetSnapshotPersisted(id, false); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get the updated target
	target, err = s.db.GetTargetSnapshotByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(target); err != nil {
		log.WithError(err).Error("failed to encode target snapshot")
		http.Error(w, "failed to encode target snapshot", http.StatusInternalServerError)
		return
	}
}

func (s *Server) handleGetTargets(w http.ResponseWriter, r *http.Request) {
	// Check if alias is provided
	alias := r.URL.Query().Get("alias")
	if alias == "" {
		http.Error(w, "alias parameter is required", http.StatusBadRequest)
		return
	}

	// Handle pagination
	page := 1
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	limit := 20
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	if limit > 20 {
		limit = 20
	}

	// Get filter parameters
	includeDeleted := false
	if filterStr := r.URL.Query().Get("include_deleted"); filterStr != "" {
		includeDeleted = filterStr == "true"
	}

	onlyPersisted := false
	if filterStr := r.URL.Query().Get("only_persisted"); filterStr != "" {
		onlyPersisted = filterStr == "true"
	}

	offset := (page - 1) * limit
	targets, err := s.db.GetTargetSnapshotsByAlias(alias, limit, offset, includeDeleted, onlyPersisted)
	if err != nil {
		log.WithError(err).Error("failed to get targets by alias")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"page":    page,
		"limit":   limit,
		"alias":   alias,
		"targets": targets,
	}); err != nil {
		log.WithError(err).Error("failed to encode targets")
		http.Error(w, "failed to encode targets", http.StatusInternalServerError)
		return
	}
}
