package logger

import (
	"fmt"
	"log"
	"os"
	"time"
)

type Level int

const (
	Info Level = iota
	Warn
	Error
	Fatal
)

type Logger struct {
	fileLogger *log.Logger
	logFile    *os.File
}

func New() *Logger {
	filename := fmt.Sprintf("%d.log", time.Now().Unix())
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}

	return &Logger{
		fileLogger: log.New(file, "", log.LstdFlags),
		logFile:    file,
	}
}

func (l *Logger) LogFileOnly(msg string, level Level) {
	prefix := levelToString(level)
	l.fileLogger.Printf("[%s] %s\n", prefix, msg)
}

func (l *Logger) LogFileWithStdout(msg string, level Level) {
	l.LogFileOnly(msg, level)
	fmt.Printf("[%s] %s\n", levelToString(level), msg)
	if level == Fatal {
		os.Exit(1)
	}
}

func (l *Logger) Close() {
	if l.logFile != nil {
		l.logFile.Close()
	}
}

func levelToString(l Level) string {
	switch l {
	case Info:
		return "INFO"
	case Warn:
		return "WARN"
	case Error:
		return "ERROR"
	case Fatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}
