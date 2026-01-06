package scheduler

import (
	"log"
	"transcoder-worker/pkg/models"
	"transcoder-worker/internal/transcoder"
)

type Scheduler struct {
	jobChan <-chan models.TranscodeJob // Receive-only channel
	engine  *transcoder.Engine
}

func New(ch <-chan models.TranscodeJob, eng *transcoder.Engine) *Scheduler {
	return &Scheduler{
		jobChan: ch,
		engine:  eng,
	}
}

// Run starts the background worker that processes jobs one by one
func (s *Scheduler) Run() {
	log.Println("Scheduler is standing by for jobs...")
	
	// This loop waits for data to arrive on the channel
	for job := range s.jobChan {
		log.Printf("Scheduler: Picking up job %s", job.ID)
		
		// 1. Build the command
		cmd := s.engine.BuildCommand(job)
		
		// 2. Execute and wait for finish (the Muscle)
		err := s.engine.Transcode(cmd)
		if err != nil {
			log.Printf("Scheduler: Job %s failed: %v", job.ID, err)
		}
	}
}