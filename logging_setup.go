package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
)

func fallbackLogPath() string {
	switch runtime.GOOS {
	case "linux":
		return "/tmp/cyberstab-installer.log"
	case "windows":
		return `C:\ProgramData\cyberstab-installer.log`
	default:
		return ""
	}
}

func setupLogging() string {
	logFilePath := filepath.Join(os.TempDir(), "cyberstab-installer.log")
	f, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		logFilePath = fallbackLogPath()
		f, err = os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	}
	if err == nil {
		log.SetOutput(io.MultiWriter(os.Stdout, f))
		log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	}
	log.Printf("=========================================")
	log.Printf("Cyberstab started (PID: %d)", os.Getpid())
	log.Printf("Log file: %s", logFilePath)
	log.Printf("=========================================")
	return logFilePath
}
