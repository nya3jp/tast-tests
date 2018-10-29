// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mafredri/cdp/protocol/runtime"
)

// logMaster records JavaScript console logs across multiple DevTool targets, and
// saves them as a single text file suitable for inspection.
type logMaster struct {
	logs map[string]*logStore // map from target ID to logs

	logCh chan *logEntry // log entries from workers are sent via this channel
	finCh chan struct{}  // a message is sent to stop the background goroutine
}

// logStore accumulates formatted text logs for a target.
type logStore struct {
	targetID string
	initURL  string
	openTime time.Time

	buf bytes.Buffer
}

// logEntry is a log entry sent from workers to the master.
type logEntry struct {
	targetID string
	ts       time.Time
	typ      string
	msg      string
	stack    *runtime.StackTrace // set only for errors
}

// writeTo writes a formatted log to w.
func (e *logEntry) writeTo(w io.Writer) {
	const format = "2006-01-02 15:04:05"
	fmt.Fprintf(w, "%s [%s] %s\n", e.ts.Local().Format(format), e.typ, e.msg)
	if e.stack != nil {
		for _, f := range e.stack.CallFrames {
			fn := f.FunctionName
			if fn == "" {
				fn = "???"
			}
			fmt.Fprintf(w, "\tat %s (%s [%d:%d])\n", fn, f.URL, f.LineNumber, f.ColumnNumber)
		}
	}
}

// newLogMaster creates a new logMaster and starts a background goroutine to
// collect log entries from workers. On cleanup, stop must be called to stop
// the background goroutine.
func newLogMaster() *logMaster {
	master := &logMaster{
		logs:  make(map[string]*logStore),
		logCh: make(chan *logEntry),
		finCh: make(chan struct{}),
	}
	go master.run()
	return master
}

// close stops the background goroutine to collect logs from workers.
// This method does not wait for workers to stop.
func (m *logMaster) close() {
	close(m.finCh)
}

// save saves the collected logs to path as a single text file.
// Logs in memory are cleared when this method successfully returns.
func (m *logMaster) save(path string) error {
	// Pause the background goroutine to avoid data races. This panics if m has
	// been already closed.
	m.finCh <- struct{}{}
	defer func() {
		go m.run()
	}()

	stores := make([]*logStore, 0, len(m.logs))
	for _, s := range m.logs {
		if s.buf.Len() > 0 {
			stores = append(stores, s)
		}
	}
	sort.Slice(stores, func(i, j int) bool {
		return stores[i].openTime.Before(stores[j].openTime)
	})

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, s := range stores {
		fmt.Fprintf(f, "%s %s\n", strings.Repeat("=", 66), s.initURL)
		io.Copy(f, &s.buf)
		fmt.Fprintln(f)
		s.buf.Reset()
	}
	return nil
}

// run is executed on a background goroutine to collect logs from workers.
func (m *logMaster) run() {
	for {
		select {
		case <-m.finCh:
			return
		case e := <-m.logCh:
			buf := &m.logs[e.targetID].buf
			e.writeTo(buf)
		}
	}
}

// newLogWorkers creates a logWorker that collects JavaScript console logs of a target.
func (m *logMaster) newLogWorker(targetID, initURL string, ev runtime.ConsoleAPICalledClient) *logWorker {
	if _, ok := m.logs[targetID]; !ok {
		m.logs[targetID] = &logStore{targetID: targetID, initURL: initURL, openTime: time.Now()}
	}

	worker := &logWorker{targetID, m.logCh, ev, make(chan struct{})}
	go worker.run()
	return worker
}

// logWorker collects JavaScript console logs of a target. Collected logs are
// sent to logMaster via logCh.
type logWorker struct {
	targetID string
	logCh    chan *logEntry
	ev       runtime.ConsoleAPICalledClient

	doneCh chan struct{} // closed to indicate the background goroutine finished
}

// close closes the stream to receive console API notifications. Once this method
// returns, you can assume all logs are flushed to logMaster.
func (w *logWorker) close() {
	w.ev.Close()
	<-w.doneCh
}

// run is executed on a background goroutine to collect logs from a target.
func (w *logWorker) run() {
	for {
		r, err := w.ev.Recv()
		if err != nil {
			break
		}

		// Report stack trace only for errors.
		var stack *runtime.StackTrace
		if r.Type == "error" {
			stack = r.StackTrace
		}
		w.report(r.Timestamp.Time(), r.Type, formatObjects(r.Args), stack)
	}

	close(w.doneCh)
}

// report sends a log to logMaster. This method can be called from Conn to report
// eval failures.
func (w *logWorker) report(ts time.Time, typ, msg string, stack *runtime.StackTrace) {
	w.logCh <- &logEntry{
		targetID: w.targetID,
		ts:       ts,
		typ:      typ,
		msg:      msg,
		stack:    stack,
	}
}

// formatObjects serializes a list of RemoteObject to a string.
func formatObjects(objs []runtime.RemoteObject) string {
	var parts []string
	for _, obj := range objs {
		parts = append(parts, formatObject(obj))
	}
	return strings.Join(parts, " ")
}

// formatObject serializes a RemoteObject to a string.
func formatObject(obj runtime.RemoteObject) string {
	if obj.Value != nil {
		var s string
		if err := json.Unmarshal([]byte(obj.Value), &s); err != nil {
			s = string(obj.Value)
		}
		return s
	} else if obj.UnserializableValue != nil {
		return string(*obj.UnserializableValue)
	} else if obj.Description != nil {
		return *obj.Description
	}
	return "???"
}
