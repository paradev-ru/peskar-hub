package main

import "time"

type Worker struct {
	IP       string `json:"ip,omitempty"`
	State    string `json:"state,omitempty"`
	UserAget string `json:"user_agent,omitempty"`

	lastVisit time.Time `json:"-"`
}

func (w *Worker) IsZombie() bool {
	if w.IsActive() && time.Since(w.lastVisit) > 5*time.Minute {
		return true
	}

	return false
}

func (w *Worker) IsActive() bool {
	if w.State == "active" {
		return true
	}
	return false
}
