package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/leominov/pechkin/lib"
)

type Server struct {
	startedAt time.Time
	config    *Config
	r         *mux.Router
	j         map[string]Job
	w         map[string]Worker
	c         *Client
	redis     *lib.RedisStore
}

type Error struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type HttpStatus struct {
	StatusCode    int    `json:"status_code"`
	Status        string `json:"status"`
	ContentLength int64  `json:"content_length"`
}

func NewServer(config *Config) *Server {
	client := NewBackend(config.DataDir)
	redis := lib.NewRedis(config.RedisMaxIdle, config.RedisIdleTimeout, config.RedisAddr)
	s := &Server{
		config: config,
		j:      make(map[string]Job),
		w:      make(map[string]Worker),
		c:      client,
		redis:  redis,
	}
	s.r = mux.NewRouter()
	s.r.HandleFunc("/http_status/", s.HttpStatusHandler).Methods("GET")
	s.r.HandleFunc("/version/", s.VersionHandler).Methods("GET")
	s.r.HandleFunc("/health/", s.HealthHandler).Methods("GET")
	s.r.HandleFunc("/ping/", s.JobNextHandler).Methods("GET")
	s.r.HandleFunc("/worker/", s.WorkerListHandler).Methods("GET")
	s.r.HandleFunc("/job/", s.JobListHandler).Methods("GET")
	s.r.HandleFunc("/job/", s.JobNewHandler).Methods("POST")
	s.r.HandleFunc("/job/{id}/", s.ValidateJob(s.JobInfoHandler)).Methods("GET")
	s.r.HandleFunc("/job/{id}/", s.ValidateJob(s.JobUpdateHandler)).Methods("PUT")
	s.r.HandleFunc("/job/{id}/", s.ValidateJob(s.JobDeleteHandler)).Methods("DELETE")
	s.r.HandleFunc("/job/{id}/log/", s.ValidateJob(s.LogHandler)).Methods("GET", "DELETE")
	s.r.HandleFunc("/job/{id}/state_history/", s.ValidateJob(s.StateHistoryHandler)).Methods("GET", "DELETE")
	s.r.NotFoundHandler = http.HandlerFunc(s.NotFoundHandler)
	return s
}

func (s *Server) ValidateJob(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		if _, ok := s.j[vars["id"]]; !ok {
			w.WriteHeader(http.StatusNotFound)
			logrus.Errorf("Job '%s' not found", vars["id"])
			encoder := json.NewEncoder(w)
			encoder.Encode(Error{
				Code:    http.StatusNotFound,
				Message: "Job not found",
			})
			return
		}
		fn(w, r)
	}
}

func (s *Server) CountActiveJobs() int {
	var c int
	for _, job := range s.j {
		if job.IsActive() {
			c++
		}
	}
	return c
}

func (s *Server) NextJob() *Job {
	for id, job := range s.j {
		if job.IsAvailable() {
			job.SetStateSystem("requested")
			job.requestedAt = time.Now()
			s.j[id] = job
			return &job
		}
	}
	return nil
}

func (s *Server) LogHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	job := s.j[vars["id"]]
	encoder := json.NewEncoder(w)

	switch r.Method {
	case "DELETE":
		logrus.Debug("Got log-delete request")
		job.Log = ""
		job.updatedAt = time.Now().UTC()
		s.j[vars["id"]] = job
		w.WriteHeader(http.StatusOK)
		return
	default:
	case "GET":
		encoder.Encode(job.Log)
	}
}

func (s *Server) StateHistoryHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	job := s.j[vars["id"]]
	encoder := json.NewEncoder(w)

	switch r.Method {
	case "DELETE":
		logrus.Debug("Got state_history-delete request")
		job.StateHistory = nil
		job.updatedAt = time.Now().UTC()
		s.j[vars["id"]] = job
		w.WriteHeader(http.StatusOK)
		return
	default:
	case "GET":
		encoder.Encode(job.StateHistory)
	}
}

func (s *Server) HttpStatusHandler(w http.ResponseWriter, r *http.Request) {
	link := r.URL.Query().Get("url")
	encoder := json.NewEncoder(w)
	if link == "" {
		logrus.Error("Empty url parameter")
		w.WriteHeader(http.StatusBadRequest)
		encoder.Encode(Error{
			Code:    http.StatusBadRequest,
			Message: "Empty url parameter",
		})
		return
	}
	resp, err := http.Head(link)
	if err != nil {
		logrus.Errorf("HTTP request error: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		encoder.Encode(Error{
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("HTTP request error: %v", err),
		})
		return
	}
	h := HttpStatus{
		StatusCode:    resp.StatusCode,
		Status:        resp.Status,
		ContentLength: resp.ContentLength,
	}
	encoder.Encode(h)
}

func (s *Server) VersionHandler(w http.ResponseWriter, r *http.Request) {
	encoder := json.NewEncoder(w)
	encoder.Encode(Version)
}

func (s *Server) WorkerListHandler(w http.ResponseWriter, r *http.Request) {
	logrus.Debug("Got job-list request")
	encoder := json.NewEncoder(w)
	encoder.Encode(s.w)
}

func (s *Server) UpdateWorkerInfo(r *http.Request) {
	ip := getIP(r)
	s.w[ip] = Worker{
		IP:         ip,
		State:      "active",
		UserAget:   r.Header.Get("User-Agent"),
		LastSeenAt: time.Now().UTC(),
	}
}

func (s *Server) JobNextHandler(w http.ResponseWriter, r *http.Request) {
	logrus.Debug("Got job-next request")
	s.UpdateWorkerInfo(r)
	c := s.CountActiveJobs()
	encoder := json.NewEncoder(w)
	if c >= s.config.ParallelJobCount {
		w.WriteHeader(http.StatusConflict)
		encoder.Encode(Error{
			Code:    http.StatusConflict,
			Message: fmt.Sprintf("Only %d job(s) cant run parallel, current running %d job(s)", s.config.ParallelJobCount, c),
		})
		return
	}
	j := s.NextJob()
	if j == nil {
		w.WriteHeader(http.StatusNotFound)
		encoder.Encode(Job{})
		return
	}
	encoder.Encode(j)
}

func (s *Server) JobNewHandler(w http.ResponseWriter, r *http.Request) {
	logrus.Debug("Got job-new request")
	var job Job
	decoder := json.NewDecoder(r.Body)
	encoder := json.NewEncoder(w)
	if err := decoder.Decode(&job); err != nil {
		logrus.Error(err)
		w.WriteHeader(http.StatusBadRequest)
		encoder.Encode(Error{
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("Error with decoding request body: %v", err),
		})
		return
	}
	j, err := s.AddJob(job)
	if err != nil {
		logrus.Error(err)
		w.WriteHeader(http.StatusConflict)
		encoder.Encode(Error{
			Code:    http.StatusConflict,
			Message: fmt.Sprintf("Error with saving job: %v", err),
		})
		return
	}
	w.WriteHeader(http.StatusCreated)
	logrus.Infof("Job '%s' created", j.ID)
	encoder.Encode(j)
}

func (s *Server) JobListHandler(w http.ResponseWriter, r *http.Request) {
	logrus.Debug("Got job-list request")
	encoder := json.NewEncoder(w)
	encoder.Encode(s.j)
}

func (s *Server) AddJob(job Job) (Job, error) {
	if job.DownloadURL == "" {
		return Job{}, errors.New("Download URL cant be empty")
	}
	for _, jb := range s.j {
		if !jb.IsDone() && jb.DownloadURL == job.DownloadURL {
			return Job{}, fmt.Errorf("Job for '%s' already exists", jb.DownloadURL)
		}
	}
	jobID, err := RandomUuid()
	if err != nil {
		return Job{}, errors.New("Error generating job ID")
	}

	job.ID = jobID
	job.AddedAt = time.Now().UTC()
	job.SetStateSystem("pending")

	s.j[job.ID] = job
	return job, nil
}

func (s *Server) JobInfoHandler(w http.ResponseWriter, r *http.Request) {
	logrus.Debug("Got job-info request")
	vars := mux.Vars(r)
	encoder := json.NewEncoder(w)
	job := s.j[vars["id"]]
	encoder.Encode(job)
}

func (s *Server) JobDeleteHandler(w http.ResponseWriter, r *http.Request) {
	logrus.Debug("Got job-delete request")
	vars := mux.Vars(r)
	encoder := json.NewEncoder(w)

	job := s.j[vars["id"]]

	if !job.IsAvailable() && !job.IsDone() {
		logrus.Errorf("Cant delete active job '%s'", job.ID)
		w.WriteHeader(http.StatusForbidden)
		encoder.Encode(Error{
			Code:    http.StatusForbidden,
			Message: "Cant delete active job",
		})
		return
	}

	logrus.Infof("Job '%s' deleted", job.ID)
	delete(s.j, job.ID)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) JobUpdateHandler(w http.ResponseWriter, r *http.Request) {
	logrus.Debug("Got job-update request")
	vars := mux.Vars(r)
	decoder := json.NewDecoder(r.Body)
	encoder := json.NewEncoder(w)
	var job Job
	if err := decoder.Decode(&job); err != nil {
		logrus.Error(err)
		w.WriteHeader(http.StatusBadRequest)
		encoder.Encode(Error{
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("Error with decoding request body: %v", err),
		})
		return
	}

	j := s.j[vars["id"]]

	j.updatedAt = time.Now()

	if job.Log != "" {
		j.Log += strings.TrimSpace(job.Log) + "\n"
	}

	if job.InfoURL != "" {
		j.InfoURL = job.InfoURL
	}

	if job.Name != "" {
		j.Name = job.Name
	}

	if job.Description != "" {
		j.Description = job.Description
	}

	if job.State != "" && job.State != j.State {
		if job.State == "requested" {
			logrus.Error("Cant change state from '%s' to '%s'", j.State, job.State)
			w.WriteHeader(http.StatusBadRequest)
			encoder.Encode(Error{
				Code:    http.StatusBadRequest,
				Message: fmt.Sprintf("Cant change state from '%s' to '%s'", j.State, job.State),
			})
			return
		}

		if job.State == "pending" {
			j.StartedAt = time.Time{}
			j.FinishedAt = time.Time{}
		}

		if j.State == "requested" && job.State == "working" {
			j.StartedAt = time.Now().UTC()
		}

		if job.IsDone() {
			j.FinishedAt = time.Now().UTC()
		}

		j.SetStateUser(job.State)
		s.redis.Send("jobs", j)
	}
	logrus.Infof("Job '%s' updated", j.ID)
	s.j[vars["id"]] = j

	encoder.Encode(j)
}

func (s *Server) NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	logrus.Error("Page not found")
	encoder := json.NewEncoder(w)
	w.WriteHeader(http.StatusNotFound)
	encoder.Encode(Error{
		Code:    http.StatusNotFound,
		Message: "Page not found",
	})
}

func (s *Server) HealthHandler(w http.ResponseWriter, r *http.Request) {
	encoder := json.NewEncoder(w)
	encoder.Encode(map[string]interface{}{
		"uptime": time.Since(s.startedAt).String(),
	})
}

func (s *Server) InvalidateZombieJobs() {
	zombieTicker := time.NewTicker(time.Minute)
	for {
		select {
		case <-zombieTicker.C:
			for id, job := range s.j {
				if !job.IsZombie() {
					continue
				}
				logrus.Debugf("Switch state to 'pending' for job '%s'", job.ID)
				job.SetStateSystem("pending")
				s.j[id] = job
			}
		}
	}
}

func (s *Server) InvalidateZimbieWorkers() {
	zombieTicker := time.NewTicker(time.Minute)
	for {
		select {
		case <-zombieTicker.C:
			for id, worker := range s.w {
				if !worker.IsZombie() {
					continue
				}
				logrus.Debugf("Switch state to 'inactive' for worker '%s'", worker.IP)
				worker.State = "inactive"
				s.w[id] = worker
			}
		}
	}
}

func (s *Server) PeriodicSave() {
	next := time.After(15 * time.Minute)
	for {
		select {
		case <-next:
			if err := s.SaveData(); err != nil {
				logrus.Error(err)
			}
			next = time.After(30 * time.Minute)
		}
	}
}

func (s *Server) Work() {
	go s.InvalidateZombieJobs()
	go s.InvalidateZimbieWorkers()
	go s.PeriodicSave()

	s.startedAt = time.Now()
	http.Handle("/", &WithCORS{s.r})
	logrus.Fatal(http.ListenAndServe(s.config.ListenAddr, nil))
}

func (s *Server) Load() error {
	if err := s.LoadData(); err != nil {
		return err
	}
	return nil
}

func (s *Server) Shutdown() error {
	if err := s.SaveData(); err != nil {
		return err
	}
	return nil
}

func (s *Server) LoadData() error {
	if err := s.c.Load("jobs", &s.j); err != nil {
		return err
	}
	logrus.Infof("Jobs loaded: %d", len(s.j))
	if err := s.c.Load("workers", &s.w); err != nil {
		return err
	}
	logrus.Infof("Workers loaded: %d", len(s.w))
	return nil
}

func (s *Server) SaveData() error {
	if err := s.c.Save("jobs", s.j); err != nil {
		return err
	}
	logrus.Infof("Jobs saved: %d", len(s.j))
	if err := s.c.Save("workers", s.w); err != nil {
		return err
	}
	logrus.Infof("Workers saved: %d", len(s.w))
	return nil
}
