package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

func main() {
	log.WithFields(logrus.Fields{
		"module": "main",
		"func":   "main",
	}).Info("Server CORE start")
	if err := DetectionEvents.Init(Storage.ServerDetection().EventsDBPath, Storage.ServerDetection().ExportDir); err != nil {
		log.WithFields(logrus.Fields{
			"module": "main",
			"func":   "main",
			"call":   "DetectionEvents.Init",
		}).Fatalln(err.Error())
		os.Exit(1)
	}
	defer func() {
		_ = DetectionEvents.Close()
	}()
	go HTTPAPIServer()
	go RTSPServer()
	go Storage.StreamChannelRunAll()
	signalChanel := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(signalChanel, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-signalChanel
		log.WithFields(logrus.Fields{
			"module": "main",
			"func":   "main",
		}).Info("Server receive signal", sig)
		done <- true
	}()
	log.WithFields(logrus.Fields{
		"module": "main",
		"func":   "main",
	}).Info("Server start success a wait signals")
	<-done
	Storage.StopAll()
	time.Sleep(2 * time.Second)
	log.WithFields(logrus.Fields{
		"module": "main",
		"func":   "main",
	}).Info("Server stop working by signal")
}
