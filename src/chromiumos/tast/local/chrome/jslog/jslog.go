// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package jslog provides JavaScript console logger for chrome package.
package jslog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mafredri/cdp/protocol/runtime"
)

// Aggregator records JavaScript console logs across multiple DevTool targets, and
// saves them as a single text file suitable for inspection.
type Aggregator struct {
	targets map[string]*target // keyed by target ID
	mutex   sync.Mutex         // protects targets

	logCh chan *entry   // log entries from workers are sent via this channel
	finCh chan struct{} // a message is sent to stop the background goroutine
}

// target accumulates formatted text logs for a target.
type target struct {
	initURL  string
	openTime time.Time

	buf bytes.Buffer
}

// entry is a log entry sent from workers to the aggregator.
type entry struct {
	targetID string
	ts       time.Time
	typ      string
	msg      string
	stack    *runtime.StackTrace // set only for errors
}

// writeTo writes a formatted log to w.
func (e *entry) writeTo(w io.Writer) {
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

// NewAggregator creates a new Aggregator and starts a background goroutine to
// collect log entries from workers. On cleanup, Close must be called to stop
// the background goroutine.
func NewAggregator() *Aggregator {
	agg := &Aggregator{
		targets: make(map[string]*target),
		logCh:   make(chan *entry),
		finCh:   make(chan struct{}),
	}
	go agg.run()
	return agg
}

// Close stops the background goroutine to collect logs from workers.
// This method does not wait for workers to stop.
func (a *Aggregator) Close() {
	close(a.finCh)
}

// Save saves the collected logs to path as a single text file.
// Logs in memory are cleared when this method successfully returns.
func (a *Aggregator) Save(path string) error {
	// Pause the background goroutine to avoid data races. This panics if a has
	// been already closed.
	a.finCh <- struct{}{}
	defer func() {
		go a.run()
	}()

	a.mutex.Lock() // protect a.targets (technically unneeded due to above pausing)
	stores := make([]*target, 0, len(a.targets))
	for _, s := range a.targets {
		if s.buf.Len() > 0 {
			stores = append(stores, s)
		}
	}
	a.mutex.Unlock()

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
func (a *Aggregator) run() {
	for {
		select {
		case <-a.finCh:
			return
		case e := <-a.logCh:
			a.mutex.Lock() // protect a.targets
			buf := &a.targets[e.targetID].buf
			a.mutex.Unlock()
			e.writeTo(buf)
		}
	}
}

// NewWorker creates a Worker that collects JavaScript console logs of a target.
func (a *Aggregator) NewWorker(targetID, initURL string, ev runtime.ConsoleAPICalledClient) *Worker {
	a.mutex.Lock() // protect a.targets
	if _, ok := a.targets[targetID]; !ok {
		a.targets[targetID] = &target{initURL: initURL, openTime: time.Now()}
	}
	a.mutex.Unlock()

	worker := &Worker{targetID, a.logCh, ev, make(chan struct{})}
	go worker.run()
	return worker
}

// Worker collects JavaScript console logs of a target. Collected logs are
// sent to Aggregator via logCh.
type Worker struct {
	targetID string
	logCh    chan *entry
	ev       runtime.ConsoleAPICalledClient

	doneCh chan struct{} // closed to indicate the background goroutine finished
}

// Close closes the stream to receive console API notifications. Once this method
// returns, you can assume all logs are flushed to Aggregator.
func (w *Worker) Close() {
	w.ev.Close()
	<-w.doneCh
}

// run is executed on a background goroutine to collect logs from a target.
func (w *Worker) run() {
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
		w.Report(r.Timestamp.Time(), r.Type, formatObjects(r.Args), stack)
	}

	close(w.doneCh)
}

// Report sends a log to Aggregator. This method can be called from Conn to Report
// eval failures.
func (w *Worker) Report(ts time.Time, typ, msg string, stack *runtime.StackTrace) {
	w.logCh <- &entry{
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
