package main

import "time"

type Job struct {
	ID          string `json:"id,omitempty"`
	State       string `json:"state,omitempty"`
	DownloadURL string `json:"download_url,omitempty"`
	InfoURL     string `json:"info_url,omitempty"`
	Name        string `json:"name,omitempty"`
	Log         string `json:"log,omitempty"`
	Description string `json:"description,omitempty"`

	AddedAt    time.Time `json:"added_at,omitempty"`
	StartedAt  time.Time `json:"started_at,omitempty"`
	FinishedAt time.Time `json:"finished_at,omitempty"`

	updatedAt   time.Time `json:"-"`
	requestedAt time.Time `json:"-"`
}

func (j *Job) IsAvailable() bool {
	if j.State == "pending" {
		return true
	}
	return false
}

func (j *Job) IsCanceled() bool {
	if j.State == "canceled" || j.State == "deleted" {
		return true
	}
	return false
}

func (j *Job) IsDone() bool {
	if j.State == "failed" || j.State == "finished" || j.IsCanceled() {
		return true
	}
	return false
}

func (j *Job) IsActive() bool {
	if j.State == "working" || j.State == "requested" || (!j.IsAvailable() && !j.IsDone()) {
		return true
	}
	return false
}

func (j *Job) IsZombie() bool {
	if j.State == "requested" && time.Since(j.requestedAt) > 5*time.Minute {
		return true
	}

	return false
}
