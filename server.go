package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
)

type Server struct {
	startedAt time.Time
	config    *Config
	r         *mux.Router
	j         map[string]Job
	w         map[string]Worker
	c         *Client
}

type Error struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

func NewServer(config *Config) *Server {
	client := NewBackend(config.DataDir)
	s := &Server{
		config: config,
		j:      make(map[string]Job),
		w:      make(map[string]Worker),
		c:      client,
	}
	s.r = mux.NewRouter()
	s.r.HandleFunc("/version/", s.VersionHandler).Methods("GET")
	s.r.HandleFunc("/health/", s.HealthHandler).Methods("GET")
	s.r.HandleFunc("/ping/", s.JobNextHandler).Methods("GET")
	s.r.HandleFunc("/worker/", s.WorkerListHandler).Methods("GET")
	s.r.HandleFunc("/job/", s.JobListHandler).Methods("GET")
	s.r.HandleFunc("/job/", s.JobNewHandler).Methods("POST")
	s.r.HandleFunc("/job/{id}/", s.ValidateJob(s.JobInfoHandler)).Methods("GET")
	s.r.HandleFunc("/job/{id}/", s.ValidateJob(s.JobUpdateHandler)).Methods("PUT")
	s.r.HandleFunc("/job/{id}/", s.ValidateJob(s.JobDeleteHandler)).Methods("DELETE")
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
			job.State = "requested"
			job.requestedAt = time.Now()
			s.j[id] = job
			return &job
		}
	}
	return nil
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
		IP:        ip,
		State:     "active",
		UserAget:  r.Header.Get("User-Agent"),
		lastVisit: time.Now(),
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
			return Job{}, fmt.Errorf("Job for %s already exists", jb.DownloadURL)
		}
	}
	jobID, err := RandomUuid()
	if err != nil {
		return Job{}, errors.New("Error generating job ID")
	}

	job.ID = jobID
	job.AddedAt = time.Now().UTC()
	job.State = "pending"

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

	logrus.Infof("Job '%s' deleted", job.ID)
	job.State = "deleted"

	s.j[vars["id"]] = job
	encoder.Encode(job)
}

func (s *Server) JobUpdateHandler(w http.ResponseWriter, r *http.Request) {
	logrus.Info("Got job-update request")
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

	if j.IsDone() {
		encoder.Encode(j)
		return
	}

	j.updatedAt = time.Now()

	if job.Log != "" {
		j.Log += job.Log
	}

	if job.InfoURL != "" {
		j.InfoURL = job.InfoURL
	}

	if job.Name != "" {
		j.Name = job.Name
	}

	if job.State != "" {
		if j.State == "pending" && job.State == "working" {
			j.StartedAt = time.Now().UTC()
		}

		if job.IsDone() {
			j.FinishedAt = time.Now().UTC()
		}

		j.State = job.State
	}

	logrus.Infof("Job '%s' updated", job.ID)
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
				job.State = "pending"
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
