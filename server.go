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

type WithCORS struct {
	r *mux.Router
}

func NewServer(config *Config) *Server {
	backend, err := NewBackend(config.DataDir)
	if err != nil {
		logrus.Panic(err)
	}

	s := &Server{
		config: config,
		j:      make(map[string]Job),
		w:      make(map[string]Worker),
		c:      backend,
	}
	s.r = mux.NewRouter()
	s.r.HandleFunc("/health/", s.HealthHandler)
	s.r.HandleFunc("/ping/", s.JobNextHandler).Methods("GET")
	s.r.HandleFunc("/worker/", s.WorkerListHandler).Methods("GET")
	s.r.HandleFunc("/job/", s.JobListHandler).Methods("GET")
	s.r.HandleFunc("/job/", s.JobNewHandler).Methods("POST")
	s.r.HandleFunc("/job/{id}/", s.JobInfoHandler).Methods("GET")
	s.r.HandleFunc("/job/{id}/", s.JobUpdateHandler).Methods("PUT")
	s.r.HandleFunc("/job/{id}/", s.JobDeleteHandler).Methods("DELETE")
	s.r.NotFoundHandler = http.HandlerFunc(s.NotFoundHandler)
	return s
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
		if job.State == "pending" {
			job.State = "requested"
			job.requestedAt = time.Now()
			s.j[id] = job
			return &job
		}
	}
	return nil
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
		encoder.Encode(Error{
			Code:    3,
			Message: fmt.Sprintf("Only %d job(s) cant run parallel"),
		})
		return
	}

	j := s.NextJob()
	if j == nil {
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
		encoder.Encode(Error{
			Code:    1,
			Message: fmt.Sprintf("Error with decoding request body: %v", err),
		})
		return
	}

	j, err := s.AddJob(job)
	if err != nil {
		logrus.Error(err)
		encoder.Encode(Error{
			Code:    2,
			Message: fmt.Sprintf("Error with saving job: %v", err),
		})
		return
	}

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
	if job, ok := s.j[vars["id"]]; ok {
		encoder.Encode(job)
		return
	}

	encoder.Encode(Error{
		Code:    404,
		Message: "Not found",
	})
}

func (s *Server) JobDeleteHandler(w http.ResponseWriter, r *http.Request) {
	logrus.Debug("Got job-delete request")
	vars := mux.Vars(r)
	encoder := json.NewEncoder(w)
	if job, ok := s.j[vars["id"]]; ok {
		logrus.Infof("Job '%s' deleted", job.ID)
		job.State = "deleted"
		s.j[vars["id"]] = job
		encoder.Encode(job)
		return
	}

	encoder.Encode(Error{
		Code:    404,
		Message: "Not found",
	})
}

func (s *Server) JobUpdateHandler(w http.ResponseWriter, r *http.Request) {
	logrus.Info("Got job-update request")
	vars := mux.Vars(r)
	decoder := json.NewDecoder(r.Body)
	encoder := json.NewEncoder(w)
	var job Job
	if err := decoder.Decode(&job); err != nil {
		logrus.Error(err)
		encoder.Encode(Error{
			Code:    1,
			Message: fmt.Sprintf("Error with decoding request body: %v", err),
		})
		return
	}

	j, ok := s.j[vars["id"]]
	if !ok {
		logrus.Error("Job not found")
		encoder.Encode(Error{
			Code:    404,
			Message: "Job not found",
		})
		return
	}

	if j.IsDone() {
		encoder.Encode(j)
		return
	}

	j.updatedAt = time.Now()
	if job.Log != "" {
		j.Log += job.Log
	}

	if job.State != "" {
		if j.State == "pending" && job.State == "working" {
			j.StartedAt = time.Now().UTC()
		}

		if job.State == "finished" {
			j.FinishedAt = time.Now().UTC()
		}
	}

	logrus.Infof("Job '%s' updated", job.ID)
	s.j[vars["id"]] = j

	encoder.Encode(j)
}

func (s *Server) NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	logrus.Error("Page not found")
	encoder := json.NewEncoder(w)
	encoder.Encode(Error{
		Code:    404,
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
				logrus.Debugf("Switch state to 'pendign' for job '%s'", job.ID)
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

func (s *WithCORS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if origin := r.Header.Get("Origin"); origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods",
			"POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers",
			"Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
	}

	if r.Method == "OPTIONS" {
		return
	}

	s.r.ServeHTTP(w, r)
}

func (s *Server) Work() {
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
	logrus.Info("Loading data...")
	if err := s.c.Load("jobs", &s.j); err != nil {
		return err
	}
	logrus.Infof("Jobs loaded: %d", len(s.j))
	if err := s.c.Load("workers", &s.j); err != nil {
		return err
	}
	logrus.Infof("Workers loaded: %d", len(s.w))
	return nil
}

func (s *Server) SaveData() error {
	logrus.Info("Saving data...")
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
