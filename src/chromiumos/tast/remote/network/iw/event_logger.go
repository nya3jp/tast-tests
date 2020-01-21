// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package iw

import (
	"bufio"
	"context"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/host"
	"chromiumos/tast/testing"
)

// EventType is the classified type of captured Event from EventLogger.
type EventType int

// EventType enums.
const (
	EventTypeDisconnect EventType = iota
	EventTypeUnknown
)

// Event is the structure to store one event from "iw event".
type Event struct {
	Type      EventType
	Timestamp time.Time
	Interface string
	Message   string
}

// EventLogger captures events on wifi interface with "iw event".
type EventLogger struct {
	lock   sync.RWMutex
	done   chan struct{}
	dut    *dut.DUT
	cmd    *host.Cmd
	events []*Event
}

// NewEventLogger creates and starts a new EventLogger.
func NewEventLogger(ctx context.Context, dut *dut.DUT) (*EventLogger, error) {
	e := &EventLogger{
		dut:  dut,
		done: make(chan struct{}),
	}
	if err := e.start(ctx); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *EventLogger) start(ctx context.Context) error {
	e.cmd = e.dut.Command("iw", "event", "-t")
	r, err := e.cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := e.cmd.Start(ctx); err != nil {
		return err
	}
	go func() {
		defer close(e.done)
		e.parser(ctx, r)
	}()
	return nil
}

// Stop the EventLogger.
func (e *EventLogger) Stop(ctx context.Context) error {
	e.cmd.Abort()
	e.cmd.Wait(ctx) // Ignore the error due to abort.
	<-e.done        // Wait the bg routine to end.
	return nil
}

// Events returns the captured events till now.
// Caller should not modify the returned slice.
func (e *EventLogger) Events() []*Event {
	e.lock.RLock()
	defer e.lock.RUnlock()
	// The logger only appends so it's ok to just return the slice.
	return e.events
}

// EventsByType returns events captured with given EventType.
func (e *EventLogger) EventsByType(et EventType) []*Event {
	e.lock.RLock()
	defer e.lock.RUnlock()

	var ret []*Event
	for _, ev := range e.events {
		if ev.Type == et {
			ret = append(ret, ev)
		}
	}
	return ret
}

func (e *EventLogger) parser(ctx context.Context, r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		ev, err := parseEvent(line)
		if err != nil {
			testing.ContextLogf(ctx, "error parsing event string %q, err=%s", line, err.Error())
			continue
		}
		e.lock.Lock()
		e.events = append(e.events, ev)
		e.lock.Unlock()
	}
}

// Format of event from iw: "<time>: <interface>[ <phy id>]: <message>"
// time: epoch time in second to 6 decimal places
// interface: "wdev 0x{idhex}"|"{ifname}"
// phy id: "(phy #{phyid})"
var eventRE = regexp.MustCompile(`\s*(\d+(?:\.\d+)?): ((?:\w+)|(?:wdev \w+))(?: \(phy #\d+\))?: (\w.*)`)

func parseEvent(line string) (*Event, error) {
	m := eventRE.FindStringSubmatch(line)
	if len(m) != 4 {
		return nil, errors.New("not in event format")
	}
	t, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse %q to float", m[1])
	}
	// Convert epoch time to time.Time.
	d, f := math.Modf(t)
	ts := time.Unix(int64(d), int64(f*1e9))
	ev := &Event{
		Timestamp: ts,
		Interface: m[2],
		Message:   m[3],
	}
	ev.Type = detectEventType(ev)
	return ev, nil
}

func detectEventType(ev *Event) EventType {
	if strings.HasPrefix(ev.Message, "disconnected") {
		return EventTypeDisconnect
	}
	if strings.HasPrefix(ev.Message, "Deauthenticated") {
		return EventTypeDisconnect
	}
	if ev.Message == "Previous authentication no longer valid" {
		return EventTypeDisconnect
	}
	return EventTypeUnknown
}
