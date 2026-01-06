package server

import (
	"encoding/json"
	"log"
	"net/http"
	"transcoder-worker/pkg/models"
)

type JobServer struct {
	port    string
	jobChan chan<- models.TranscodeJob // Send-only channel
}

func NewJobServer(port string, jobChan chan<- models.TranscodeJob) *JobServer {
	return &JobServer{
		port:    port,
		jobChan: jobChan,
	}
}

func (s *JobServer) Start() {
	http.HandleFunc("/jobs", s.handleJobAssignment)
	log.Printf("Listening for jobs on port %s", s.port)
	http.ListenAndServe(":"+s.port, nil)
}

func (s *JobServer) handleJobAssignment(w http.ResponseWriter, r *http.Request) {
	var job models.TranscodeJob
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Toss the job into the channel for the scheduler to pick up
	s.jobChan <- job

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"message": "Job queued"})
}