// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hostapd

import (
	"bytes"
	"io"
	"regexp"
	"sync"
)

// EventType is the enum of type of Event.
type EventType string

// EventType enums.
const (
	EventDeauth = "deauth"
)

// Event is the type of hostapd event detected from its stdout.
type Event struct {
	// Type is the type of the event.
	Type EventType
	// Client is the mac address of client.
	Client string
	// Msg is the full log of the event.
	Msg string
}

// EventWriter stores the stdout of hostapd and detects the hostapd
// events.
type EventWriter struct {
	lock   sync.Mutex
	buf    []byte
	events []*Event
}

var _ io.WriteCloser = (*EventWriter)(nil)

// NewEventWriter creates a EventWriter object.
func NewEventWriter() *EventWriter {
	return &EventWriter{}
}

// deauthRE is the regex of deauth message: "$interface: deauthentication: STA=$client_mac".
var deauthRE = regexp.MustCompile(`\S+: deauthentication: STA=([0-9A-Fa-f:]+)`)

// detectEvent parses one line in log and return the detected
// event if available. Otherwise, nil is returned.
func (w *EventWriter) detectEvent(line []byte) *Event {
	if m := deauthRE.FindSubmatch(line); len(m) != 0 {
		return &Event{
			Type:   EventDeauth,
			Client: string(m[1]),
			Msg:    string(line),
		}
	}
	return nil
}

// Write writes p to the buffer and detect hostapd events.
// It implements io.Writer interface.
func (w *EventWriter) Write(p []byte) (int, error) {
	w.lock.Lock()
	defer w.lock.Unlock()

	// Detect the events line by line.
	w.buf = append(w.buf, p...)
	lines := bytes.Split(w.buf, []byte("\n"))
	// The last split is not a complete line yet, only
	// detect the complete lines.
	last := len(lines) - 1
	for _, line := range lines[:last] {
		if ev := w.detectEvent(line); ev != nil {
			w.events = append(w.events, ev)
		}
	}
	// Write the incomplete part back to buffer.
	w.buf = append(w.buf[:0], lines[last]...)
	return len(p), nil
}

// Close closes the writer.
func (w *EventWriter) Close() error {
	if len(w.buf) != 0 {
		// Flush the remaining log as the last line.
		w.Write([]byte("\n"))
	}
	return nil
}

// Events returns the detected events.
// Notice: The returned slice is immutable, caller must not modify it.
func (w *EventWriter) Events() []*Event {
	w.lock.Lock()
	defer w.lock.Unlock()
	// We only append, so we can just return the slice.
	return w.events
}
