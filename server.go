package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/leominov/peskar-hub/lib"
	"github.com/leominov/peskar-hub/peskar"
	"github.com/leominov/peskar-hub/weburg"
)

type Server struct {
	Name       string
	startedAt  time.Time
	config     *Config
	r          *mux.Router
	j          map[string]peskar.Job
	w          map[string]peskar.Worker
	c          *Client
	redis      *lib.RedisStore
	indexerSub *lib.Subscribe
	weburgMS   *weburg.MovieService
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

func NewServer(name string, config *Config) *Server {
	weburgCli := weburg.NewClient(http.DefaultClient)
	client := NewBackend(config.DataDir)
	redis := lib.NewRedis(config.RedisMaxIdle, config.RedisIdleTimeout, config.RedisAddr)
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "na"
	}
	s := &Server{
		Name:   fmt.Sprintf("%s-%s", name, hostname),
		config: config,
		j:      make(map[string]peskar.Job),
		w:      make(map[string]peskar.Worker),
		c:      client,
		redis:  redis,
		weburgMS: &weburg.MovieService{
			Client: weburgCli,
		},
	}
	s.r = mux.NewRouter()
	s.r.NotFoundHandler = http.HandlerFunc(s.NotFoundHandler)
	v1 := s.r.PathPrefix("/v1").Subrouter()
	v1.HandleFunc("/work_time/", s.WorkTimeHandler).Methods("GET")
	v1.HandleFunc("/http_status/", s.HttpStatusHandler).Methods("GET")
	v1.HandleFunc("/weburg_movie_info/", s.WeburgMovieInfoHandler).Methods("GET")
	v1.HandleFunc("/version/", s.VersionHandler).Methods("GET")
	v1.HandleFunc("/health/", s.HealthHandler).Methods("GET")
	v1.HandleFunc("/ping/", s.JobNextHandler).Methods("GET")
	v1.HandleFunc("/worker/", s.WorkerListHandler).Methods("GET")
	v1.HandleFunc("/job/", s.JobListHandler).Methods("GET")
	v1.HandleFunc("/job/", s.JobNewHandler).Methods("POST")
	v1.HandleFunc("/job/{id}/", s.ValidateJob(s.JobInfoHandler)).Methods("GET")
	v1.HandleFunc("/job/{id}/", s.ValidateJob(s.JobUpdateHandler)).Methods("PUT")
	v1.HandleFunc("/job/{id}/", s.ValidateJob(s.JobDeleteHandler)).Methods("DELETE")
	v1.HandleFunc("/job/{id}/log/", s.ValidateJob(s.LogHandler)).Methods("GET", "DELETE")
	v1.HandleFunc("/job/{id}/log/", s.ValidateJob(s.LogNewHandler)).Methods("POST")
	v1.HandleFunc("/job/{id}/state_history/", s.ValidateJob(s.StateHistoryHandler)).Methods("GET", "DELETE")
	return s
}

func (s *Server) Subscribe() error {
	if err := s.redis.Check(); err != nil {
		return err
	}
	s.indexerSub = s.redis.NewSubscribe("index")
	s.indexerSub.SuccessReceivedCallback = s.IndexSuccessReceived
	return s.indexerSub.Run()
}

func (s *Server) IndexSuccessReceived(result []byte) error {
	var incommingLog peskar.LogItem
	var job peskar.Job
	if err := json.Unmarshal(result, &incommingLog); err != nil {
		return fmt.Errorf("Unmarshal error: %v (%s)", err, string(result))
	}
	if _, ok := s.j[incommingLog.JobID]; !ok {
		return fmt.Errorf("Job id '%s' not found", incommingLog.JobID)
	}
	job = s.j[incommingLog.JobID]
	if incommingLog.Message != "" {
		job.AddLogItem(incommingLog)
		s.j[incommingLog.JobID] = job
		return nil
	}
	return fmt.Errorf("Empty message for job '%s'", incommingLog.JobID)
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

func (s *Server) NextJob() *peskar.Job {
	for id, job := range s.j {
		if job.IsAvailable() {
			job.SetStateSystem("requested")
			job.Requested()
			s.j[id] = job
			return &job
		}
	}
	return nil
}

func (s *Server) WorkTimeHandler(w http.ResponseWriter, r *http.Request) {
	var wt bool
	wt = true
	if s.config.DndEnable {
		wt = lib.IsAvailable(time.Now(), s.config.DndStartsAt, s.config.DndEndsAt)
	}
	encoder := json.NewEncoder(w)
	encoder.Encode(map[string]interface{}{
		"local_time":     time.Now(),
		"local_time_utc": time.Now().UTC(),
		"dnd_starts_at":  s.config.DndStartsAt,
		"dnd_ends_at":    s.config.DndEndsAt,
		"is_work_time":   wt,
		"dnd_enable":     s.config.DndEnable,
	})
}

func (s *Server) LogNewHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	job := s.j[vars["id"]]
	var incommingLog peskar.LogItem
	decoder := json.NewDecoder(r.Body)
	encoder := json.NewEncoder(w)
	if err := decoder.Decode(&incommingLog); err != nil {
		logrus.Error(err)
		w.WriteHeader(http.StatusBadRequest)
		encoder.Encode(Error{
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("Error with decoding request body: %v", err),
		})
		return
	}
	if incommingLog.Message == "" {
		w.WriteHeader(http.StatusBadRequest)
		encoder.Encode(Error{
			Code:    http.StatusBadRequest,
			Message: "Empty log message",
		})
		return
	}
	job.AddLogItem(incommingLog)
	job.Updated()
	s.j[vars["id"]] = job
	w.WriteHeader(http.StatusCreated)
	encoder.Encode(incommingLog)
}

func (s *Server) LogHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	job := s.j[vars["id"]]
	encoder := json.NewEncoder(w)
	switch r.Method {
	case "DELETE":
		logrus.Debug("Got log-delete request")
		job.DeleteLog()
		job.Updated()
		s.j[vars["id"]] = job
		w.WriteHeader(http.StatusOK)
		return
	default:
	case "GET":
		encoder.Encode(job.LogList())
	}
}

func (s *Server) StateHistoryHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	job := s.j[vars["id"]]
	encoder := json.NewEncoder(w)

	switch r.Method {
	case "DELETE":
		logrus.Debug("Got state_history-delete request")
		job.DeleteStateHistory()
		job.Updated()
		s.j[vars["id"]] = job
		w.WriteHeader(http.StatusOK)
		return
	default:
	case "GET":
		encoder.Encode(job.StateHistoryList())
	}
}

func (s *Server) WeburgMovieInfoHandler(w http.ResponseWriter, r *http.Request) {
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
	res, err := s.weburgMS.Info(link)
	if err != nil {
		logrus.Errorf("Error with getting info from Weburg: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		encoder.Encode(Error{
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("Error with getting info from Weburg: %v", err),
		})
		return
	}
	encoder.Encode(res)
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
	logrus.Debug("Got worker-list request")
	encoder := json.NewEncoder(w)
	workerList := []peskar.Worker{}
	for _, worker := range s.w {
		workerList = append(workerList, worker)
	}
	encoder.Encode(workerList)
}

func (s *Server) UpdateWorkerInfo(r *http.Request) {
	ip := getIP(r)
	s.w[ip] = peskar.Worker{
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
		encoder.Encode(peskar.Job{})
		return
	}
	encoder.Encode(j)
}

func (s *Server) JobNewHandler(w http.ResponseWriter, r *http.Request) {
	logrus.Debug("Got job-new request")
	var job peskar.Job
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
	jobList := []peskar.Job{}
	for _, job := range s.j {
		jobList = append(jobList, job)
	}
	encoder.Encode(jobList)
}

func (s *Server) AddJob(job peskar.Job) (peskar.Job, error) {
	if job.DownloadURL == "" {
		return peskar.Job{}, errors.New("Download URL cant be empty")
	}
	for _, jb := range s.j {
		if !jb.IsDone() && jb.DownloadURL == job.DownloadURL {
			return peskar.Job{}, fmt.Errorf("Job for '%s' already exists", jb.DownloadURL)
		}
	}
	jobID, err := RandomUuid()
	if err != nil {
		return peskar.Job{}, errors.New("Error generating job ID")
	}

	job.ID = jobID
	job.Added()
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
	var job peskar.Job
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

	j.Updated()

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
