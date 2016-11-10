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

	StateHistory []StateHistoryItem `json:"state_history,omitempty"`

	updatedAt   time.Time `json:"-"`
	requestedAt time.Time `json:"-"`
}

type StateHistoryItem struct {
	ChangedAt time.Time `json:"changed_at"`
	Initiator string    `json:"initiator"`
	FromState string    `json:"from_state"`
	ToState   string    `json:"to_state"`
}

func (j *Job) SetState(state, initiator string) error {
	h := StateHistoryItem{
		ChangedAt: time.Now().UTC(),
		Initiator: initiator,
		FromState: j.State,
		ToState:   state,
	}
	j.State = state
	j.StateHistory = append(j.StateHistory, h)
	return nil
}

func (j *Job) SetStateUser(state string) error {
	return j.SetState(state, "user")
}

func (j *Job) SetStateSystem(state string) error {
	return j.SetState(state, "system")
}

func (j *Job) IsAvailable() bool {
	if j.State == "pending" {
		return true
	}
	return false
}

func (j *Job) IsDone() bool {
	if j.State == "failed" || j.State == "finished" || j.State == "canceled" {
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
