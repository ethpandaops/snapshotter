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
	r.HandleFunc("/api/runs", s.handleGetRuns).Methods("GET")
	r.HandleFunc("/api/status", s.handleGetStatus).Methods("GET")

	listenAddr := s.cfg.Server.ListenAddr
	if listenAddr == "" {
		listenAddr = "0.0.0.0:5000"
	}

	log.WithField("addr", listenAddr).Info("starting HTTP server")
	return http.ListenAndServe(listenAddr, r)
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

	offset := (page - 1) * limit
	runs, err := s.db.GetPaginatedRuns(offset, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"page":  page,
		"limit": limit,
		"runs":  runs,
	})
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
	json.NewEncoder(w).Encode(resp)
}
