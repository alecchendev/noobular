package internal

import (
	"fmt"
	"log"

	"github.com/google/uuid"
)

type Logger struct {
	requestId *uuid.UUID
}

func (l Logger) RequestId(requestId uuid.UUID) Logger {
	l.requestId = &requestId
	return l
}

type Level string

const (
	infoLevel Level = "INFO"
	errorLevel = "ERROR"
	debugLevel = "DEBUG"
)

func (l Logger) log(level Level, format string, v ...any) {
	context := string(level)
	if l.requestId != nil {
		context += fmt.Sprintf(" %s ", l.requestId)
	}
	log.Printf(context + format, v)
}

func (l Logger) Info(format string, v ...any) {
	l.log(infoLevel, format, v)
}

func (l Logger) Error(format string, v ...any) {
	l.log(errorLevel, format, v)
}

func (l Logger) Debug(format string, v ...any) {
	l.log(debugLevel, format, v)
}
