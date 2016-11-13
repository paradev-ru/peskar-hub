package main

import (
	"errors"
	"flag"
	"os"
	"time"

	"github.com/Sirupsen/logrus"
)

const (
	DefaultDataDir          = "/opt/peskar/data"
	DefaultListenAddr       = "0.0.0.0:8080"
	DefaultParallelJobCount = 1
	DefaultRedisAddr        = "redis://localhost:6379/0"
	DefaultRedisIdleTimeout = 240 * time.Second
	DefaultRedisMaxIdle     = 3
)

var (
	datadir          string
	listenAddr       string
	logLevel         string
	parallelJobCount int
	printVersion     bool
	config           Config
	redisAddr        string
	redisIdleTimeout time.Duration
	redisMaxIdle     int
)

type Config struct {
	ParallelJobCount int
	ListenAddr       string
	LogLevel         string
	DataDir          string
	RedisAddr        string
	RedisIdleTimeout time.Duration
	RedisMaxIdle     int
}

func init() {
	flag.StringVar(&datadir, "datadir", "", "data directory")
	flag.IntVar(&parallelJobCount, "parallel-jobs", 0, "number of parallel jobs")
	flag.StringVar(&listenAddr, "listen-addr", "", "listen address")
	flag.StringVar(&logLevel, "log-level", "", "level which confd should log messages")
	flag.BoolVar(&printVersion, "version", false, "print version and exit")
	flag.StringVar(&redisAddr, "redis", "", "Redis server URL")
	flag.DurationVar(&redisIdleTimeout, "redis-idle-timeout", 0*time.Second, "close Redis connections after remaining idle for this duration")
	flag.IntVar(&redisMaxIdle, "redis-max-idle", 0, "Maximum number of idle connections in the Redis pool")
}

func initConfig() error {
	config = Config{
		DataDir:          DefaultDataDir,
		ListenAddr:       DefaultListenAddr,
		ParallelJobCount: DefaultParallelJobCount,
		RedisAddr:        DefaultRedisAddr,
		RedisIdleTimeout: DefaultRedisIdleTimeout,
		RedisMaxIdle:     DefaultRedisMaxIdle,
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

	if config.RedisAddr == "" {
		return errors.New("Must specify Redis server URL using -redis")
	}

	if config.RedisIdleTimeout == 0*time.Second {
		return errors.New("Must specify Redis idle timeout using -redis-idle-timeout")
	}

	if config.RedisMaxIdle == 0 {
		return errors.New("Must specify Redis max idle using -redis-max-idle")
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
	redisAddrEnv := os.Getenv("PESKAR_REDIS_ADDR")
	if len(redisAddrEnv) > 0 {
		config.RedisAddr = redisAddrEnv
	}
	listenAddrEnv := os.Getenv("PESKAR_LISTEN_ADDR")
	if len(listenAddrEnv) > 0 {
		config.ListenAddr = listenAddrEnv
	}
	dataDirEnv := os.Getenv("PESKAR_DATADIR")
	if len(dataDirEnv) > 0 {
		config.DataDir = dataDirEnv
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
	case "redis":
		config.RedisAddr = redisAddr
	case "redis-idle-timeout":
		config.RedisIdleTimeout = redisIdleTimeout
	case "redis-max-idle":
		config.RedisMaxIdle = redisMaxIdle
	case "log-level":
		config.LogLevel = logLevel
	}
}
