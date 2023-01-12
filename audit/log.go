package audit

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"time"
)

type Severity string

const (
	Error   Severity = "ERROR"
	Warning Severity = "WARNING"
	Info    Severity = "INFO"
)

type message struct {
	Severity Severity
	Text     string
}

func pushLog(m message) {
	switch m.Severity {
	case Error:
		log.Printf("AUDIT|%v|%v|%s", m.Severity, time.Now().Format(time.RFC1123), m.Text)
	case Warning:
		log.Printf("AUDIT|%v|%v|%s", m.Severity, time.Now().Format(time.RFC1123), m.Text)
	case Info:
		log.Printf("AUDIT|%v|%v|%s", m.Severity, time.Now().Format(time.RFC1123), m.Text)
	}
}

func LogTX(severity Severity, location string, hash []byte, text string) {
	pushLog(message{
		Severity: severity,
		Text:     fmt.Sprintf("%s|TX:%x|%s", location, hash, text),
	})
}

func Log(severity Severity, location, event, text string) {
	pushLog(message{
		Severity: severity,
		Text:     fmt.Sprintf("%s|%s|%s", location, event, text),
	})
}
