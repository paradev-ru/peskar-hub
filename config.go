package main

import (
	"errors"
	"flag"

	"github.com/Sirupsen/logrus"
)

var (
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
}

func init() {
	flag.IntVar(&parallelJobCount, "parallel-jobs", 0, "number of parallel jobs")
	flag.StringVar(&listenAddr, "listen-addr", "", "listen address")
	flag.StringVar(&logLevel, "log-level", "", "level which confd should log messages")
	flag.BoolVar(&printVersion, "version", false, "print version and exit")
}

func initConfig() error {
	config = Config{
		ParallelJobCount: 1,
	}

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

	return nil
}

func processFlags() {
	flag.Visit(setConfigFromFlag)
}

func setConfigFromFlag(f *flag.Flag) {
	switch f.Name {
	case "parallel-jobs":
		config.ParallelJobCount = parallelJobCount
	case "listen-addr":
		config.ListenAddr = listenAddr
	case "log-level":
		config.LogLevel = logLevel
	}
}
