package logger

import (
	"bytes"
	"sync"

	"github.com/GoogleCloudPlatform/compute-image-tools/daisy"
)

type TLogger struct {
	Buf bytes.Buffer
	Mx  sync.Mutex
}

func (l *TLogger) WriteLogEntry(e *daisy.LogEntry) {
	l.Mx.Lock()
	defer l.Mx.Unlock()
	l.Buf.WriteString(e.String())
}

func (l *TLogger) WriteSerialPortLogs(w *daisy.Workflow, instance string, buf bytes.Buffer) {
	return
}

func (l *TLogger) ReadSerialPortLogs() []string {
	return []string{}
}

func (l *TLogger) Flush() { return }
