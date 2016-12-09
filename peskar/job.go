package peskar

import (
	"path/filepath"
	"strings"
	"time"
)

type Job struct {
	ID          string `json:"id,omitempty"`
	State       string `json:"state,omitempty"`
	DownloadURL string `json:"download_url,omitempty"`
	InfoURL     string `json:"info_url,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`

	AddedAt    time.Time `json:"added_at,omitempty"`
	StartedAt  time.Time `json:"started_at,omitempty"`
	FinishedAt time.Time `json:"finished_at,omitempty"`

	stateHistory []StateHistoryItem `json:"-"`
	log          []LogItem          `json:"-"`
	updatedAt    time.Time          `json:"-"`
	requestedAt  time.Time          `json:"-"`
}

type LogItem struct {
	Initiator string    `json:"initiator"`
	AddedAt   time.Time `json:"added_at"`
	JobID     string    `json:"job_id,omitempty"`
	Message   string    `json:"message"`
}

type StateHistoryItem struct {
	Initiator string    `json:"initiator"`
	ChangedAt time.Time `json:"changed_at"`
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
	j.stateHistory = append(j.stateHistory, h)
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

func (j *Job) AddLogItem(log LogItem) {
	log.AddedAt = time.Now().UTC()
	j.log = append(j.log, log)
}

func (j *Job) Log(initiator, message string) {
	j.AddLogItem(LogItem{
		Initiator: initiator,
		Message:   message,
	})
}

func (j *Job) Updated() {
	j.updatedAt = time.Now().UTC()
}

func (j *Job) DeleteLog() {
	j.log = []LogItem{}
}

func (j *Job) DeleteStateHistory() {
	j.stateHistory = []StateHistoryItem{}
}

func (j *Job) Directory() string {
	fileBase := filepath.Base(j.DownloadURL)
	fileExt := filepath.Ext(fileBase)
	return strings.TrimSuffix(fileBase, fileExt)
}

func (j *Job) LogList() []LogItem {
	return j.log
}

func (j *Job) StateHistoryList() []StateHistoryItem {
	return j.stateHistory
}

func (j *Job) Requested() {
	j.requestedAt = time.Now()
}

func (j *Job) Added() {
	j.AddedAt = time.Now().UTC()
}
