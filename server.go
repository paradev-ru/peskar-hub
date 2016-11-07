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
}

type Error struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

func NewServer(config *Config) *Server {
	s := &Server{
		config: config,
		j:      make(map[string]Job),
	}
	s.r = mux.NewRouter()
	s.r.HandleFunc("/health/", s.HealthHandler)
	s.r.HandleFunc("/ping/", s.JobNextHandler).Methods("GET")
	s.r.HandleFunc("/job/", s.JobNewHandler).Methods("POST")
	s.r.HandleFunc("/job/", s.JobListHandler).Methods("GET")
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

func (s *Server) JobNextHandler(w http.ResponseWriter, r *http.Request) {
	logrus.Info("Got job-next request")
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
	logrus.Info("Got job-new request")
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

	encoder.Encode(j)
}

func (s *Server) JobListHandler(w http.ResponseWriter, r *http.Request) {
	logrus.Info("Got job-list request")
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
	job.AddedAt = time.Now().UTC().String()
	job.State = "pending"

	s.j[job.ID] = job

	return job, nil
}

func (s *Server) JobInfoHandler(w http.ResponseWriter, r *http.Request) {
	logrus.Info("Got job-info request")
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
	logrus.Info("Got job-delete request")
	vars := mux.Vars(r)
	encoder := json.NewEncoder(w)
	if job, ok := s.j[vars["id"]]; ok {
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

	if j.State == "pending" && job.State == "working" {
		j.StartedAt = time.Now().UTC().String()
	}

	j.updatedAt = time.Now()
	j.State = job.State
	j.Log += job.Log
	if job.State == "finished" {
		j.FinishedAt = time.Now().UTC().String()
	}
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
	zombieTicker := time.NewTicker(5 * time.Minute)
	for {
		select {
		case <-zombieTicker.C:
			for id, job := range s.j {
				if !job.IsZombie() {
					continue
				}
				job.State = "pending"
				s.j[id] = job
			}
		}
	}
}

func (s *Server) Work() {
	s.startedAt = time.Now()
	http.Handle("/", s.r)
	logrus.Fatal(http.ListenAndServe(s.config.ListenAddr, nil))
}
