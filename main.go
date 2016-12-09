package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Sirupsen/logrus"
)

const (
	BaseName = "peskar-hub"
)

func main() {
	flag.Parse()
	if printVersion {
		fmt.Printf("%s %s\n", BaseName, Version)
		os.Exit(0)
	}

	if err := initConfig(); err != nil {
		logrus.Fatal(err.Error())
	}

	logrus.Infof("Starting %s", BaseName)
	logrus.Infof("HTTP listening on %s", config.ListenAddr)

	s := NewServer(BaseName, &config)

	if err := s.redis.Check(); err != nil {
		logrus.Error(err)
		os.Exit(1)
	}

	if err := s.Load(); err != nil {
		logrus.Error(err)
	}

	go s.Subscribe()
	go s.Work()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case sign := <-signalChan:
			logrus.Info(fmt.Sprintf("Captured %v. Exiting...", sign))
			if err := s.Shutdown(); err != nil {
				logrus.Panic(err)
			}
			os.Exit(0)
		}
	}
}
