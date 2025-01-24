package server

import (
	"encoding/json"
	"net/http"

	"github.com/ethpandaops/eth-snapshotter/internal/config"
	"github.com/ethpandaops/eth-snapshotter/internal/db"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

type Server struct {
	cfg *config.Config
	db  *db.DB
}

func New(cfg *config.Config, database *db.DB) *Server {
	return &Server{
		cfg: cfg,
		db:  database,
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
	runs, err := s.db.GetAllRuns()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(runs)
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
		LatestRun *db.SnapshotRun `json:"latestRun"`
	}{
		LatestRun: run,
	}
	json.NewEncoder(w).Encode(resp)
}
