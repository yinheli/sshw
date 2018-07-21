package sshw

import (
	"fmt"
	"log"
	"os"
)

type Logger interface {
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
}

type logger struct{}

var (
	l      Logger = &logger{}
	stdlog        = log.New(os.Stdout, "[sshw] ", log.LstdFlags)
)

func GetLogger() Logger {
	return l
}

func SetLogger(logger Logger) {
	l = logger
}

func (l *logger) Info(args ...interface{}) {
	l.println("[info]", args...)
}

func (l *logger) Infof(format string, args ...interface{}) {
	l.printlnf("[info]", format, args...)
}

func (l *logger) Error(args ...interface{}) {
	l.println("[error]", args...)
}

func (l *logger) Errorf(format string, args ...interface{}) {
	l.printlnf("[level]", format, args...)
}

func (l *logger) println(level string, args ...interface{}) {
	stdlog.Println(level, fmt.Sprintln(args...))
}

func (l *logger) printlnf(level string, format string, args ...interface{}) {
	stdlog.Println(level, fmt.Sprintf(format, args...))
}
