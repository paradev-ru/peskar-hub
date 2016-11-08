package main

import (
	"errors"
	"flag"
	"os"

	"github.com/Sirupsen/logrus"
)

const (
	DefaultDataDir          = "/opt/peskar/data"
	DefaultListenAddr       = "0.0.0.0:8080"
	DefaultParallelJobCount = 1
)

var (
	datadir          string
	listenAddr       string
	logLevel         string
	parallelJobCount int
	printVersion     bool
	config           Config
)

type Config struct {
	ParallelJobCount int
	ListenAddr       string
	LogLevel         string
	DataDir          string
}

func init() {
	flag.StringVar(&datadir, "datadir", "", "data directory")
	flag.IntVar(&parallelJobCount, "parallel-jobs", 0, "number of parallel jobs")
	flag.StringVar(&listenAddr, "listen-addr", "", "listen address")
	flag.StringVar(&logLevel, "log-level", "", "level which confd should log messages")
	flag.BoolVar(&printVersion, "version", false, "print version and exit")
}

func initConfig() error {
	config = Config{
		DataDir:          DefaultDataDir,
		ListenAddr:       DefaultListenAddr,
		ParallelJobCount: DefaultParallelJobCount,
	}

	processEnv()

	processFlags()

	if config.LogLevel != "" {
		level, err := logrus.ParseLevel(config.LogLevel)
		if err != nil {
			return err
		}
		logrus.SetLevel(level)
	}

	if config.ParallelJobCount == 0 {
		return errors.New("Must specify number of parallel jobs using -parallel-jobs")
	}

	if config.ListenAddr == "" {
		return errors.New("Must specify HTTP listen address using -listen-addr")
	}

	if config.DataDir == "" {
		return errors.New("Must specify data directory using -datadir")
	}

	return nil
}

func processEnv() {
	listenAddr := os.Getenv("PESKAR_LISTEN_ADDR")
	if len(listenAddr) > 0 {
		config.ListenAddr = listenAddr
	}

	dataDir := os.Getenv("PESKAR_DATADIR")
	if len(dataDir) > 0 {
		config.DataDir = dataDir
	}
}

func processFlags() {
	flag.Visit(setConfigFromFlag)
}

func setConfigFromFlag(f *flag.Flag) {
	switch f.Name {
	case "datadir":
		config.DataDir = datadir
	case "parallel-jobs":
		config.ParallelJobCount = parallelJobCount
	case "listen-addr":
		config.ListenAddr = listenAddr
	case "log-level":
		config.LogLevel = logLevel
	}
}
