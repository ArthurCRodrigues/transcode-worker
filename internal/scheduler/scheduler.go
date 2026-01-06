package scheduler

import (
	"encoding/json"
	"net/http"
	"log"
	"pkg/models" // Assuming our shared structs are here
)

type Server struct {
	port string
}

func NewServer(port string) *Server {
	return &Server{port: port}
}

// Start opens the HTTP port to listen for jobs
func (s *Server) Start() {
	http.HandleFunc("/jobs", s.handleJobAssignment)
	
	log.Printf("Job server listening on port %s", s.port)
	if err := http.ListenAndServe(":"+s.port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func (s *Server) handleJobAssignment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var job models.TranscodeJob
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	log.Printf("Received job assignment: %s for file %s", job.JobID, job.SourcePath)

	// TODO: Send this job to the Transcoder Engine
	
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}