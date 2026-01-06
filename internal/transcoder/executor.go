package transcoder

import (
	"log"
	"os/exec"
	"fmt"
)

// Execute takes an exec.Cmd and runs it.
func (e *Engine) Execute(cmd *exec.Cmd) error {
	// 1. We use Start() instead of Run() because Start is non-blocking -> Go stays alive while FFmpeg works in the background.
	if err := cmd.Start(); err != nil {
		return err
	}

	log.Printf("FFmpeg started with PID: %d", cmd.Process.Pid)

	// 2. We Wait() for the process to finish.
	// Later, I will use a Context here to kill it if needed.
	err := cmd.Wait()
	if err != nil {
		return fmt.Errorf("ffmpeg execution failed: %w", err)
	}

	return nil
}