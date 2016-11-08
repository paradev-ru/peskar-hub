package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Sirupsen/logrus"
)

func main() {
	flag.Parse()
	if printVersion {
		fmt.Printf("peskar-hub %s\n", Version)
		os.Exit(0)
	}

	if err := initConfig(); err != nil {
		logrus.Fatal(err.Error())
	}

	logrus.Info("Starting peskar-hub")
	logrus.Infof("HTTP listening on %s", config.ListenAddr)

	s := NewServer(&config)

	if err := s.Load(); err != nil {
		logrus.Panic(err)
	}

	go s.Work()
	go s.InvalidateZombieJobs()
	go s.InvalidateZimbieWorkers()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case sign := <-signalChan:
			logrus.Info(fmt.Sprintf("Captured %v. Exiting...", sign))
			if err := s.Shutdown(); err != nil {
				logrus.Panic(err)
			}
			logrus.Info("Done")
			os.Exit(0)
		}
	}
}
