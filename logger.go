package main

import (
	"log"
	"os"
)

// SetupLogger creates and configures the logger
func SetupLogger(logFile string) (*log.Logger, *os.File, error) {
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, nil, err
	}

	logger := log.New(file, "", log.LstdFlags)
	return logger, file, nil
}
