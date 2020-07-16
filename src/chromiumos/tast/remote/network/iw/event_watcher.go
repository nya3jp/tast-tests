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
	"time"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

// EventType is the classified type of captured Event from EventWatcher.
type EventType int

// EventType enums.
const (
	EventTypeDisconnect EventType = iota
	EventTypeChanSwitch
	EventTypeUnknown
)

// Event is the structure to store one event from "iw event".
type Event struct {
	Type      EventType
	Timestamp time.Time
	Interface string
	Message   string
}

// EventWatcher captures events on wifi interface with "iw event".
type EventWatcher struct {
	done   chan struct{}
	abort  chan struct{}
	dut    *dut.DUT
	cmd    *ssh.Cmd
	events chan *Event
}

// NewEventWatcher creates and starts a new EventWatcher.
func NewEventWatcher(ctx context.Context, dut *dut.DUT) (*EventWatcher, error) {
	e := &EventWatcher{
		dut:    dut,
		done:   make(chan struct{}),
		abort:  make(chan struct{}),
		events: make(chan *Event),
	}
	if err := e.start(ctx); err != nil {
		return nil, err
	}
	return e, nil
}

// start the "iw event" process and parser routine in background.
func (e *EventWatcher) start(ctx context.Context) error {
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
		defer close(e.events)
		e.parse(ctx, r)
	}()
	return nil
}

// Stop stops the EventWatcher.
// Note that it also closes the channel, that is, all events are dropped even if they arrived before calling Stop().
func (e *EventWatcher) Stop(ctx context.Context) error {
	e.cmd.Abort()
	e.cmd.Wait(ctx) // Ignore the error due to abort.
	close(e.abort)
	<-e.done // Wait for the bg routine to end.
	return nil
}

// Wait waits for the next event.
func (e *EventWatcher) Wait(ctx context.Context) (*Event, error) {
	select {
	// Will return (nil, nil) if the channel is closed.
	case ev := <-e.events:
		return ev, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// WaitByType waits for the next matched event.
func (e *EventWatcher) WaitByType(ctx context.Context, et EventType) (*Event, error) {
	for {
		ev, err := e.Wait(ctx)
		if err != nil {
			return nil, err
		}
		if ev == nil {
			return nil, errors.New("failed to wait by type: channel closed")
		}
		if ev.Type == et {
			return ev, nil
		}
	}
}

// parse output of "iw event" from reader.
func (e *EventWatcher) parse(ctx context.Context, r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		ev, err := parseEvent(line)
		if err != nil {
			testing.ContextLogf(ctx, "error parsing event, err=%s", err.Error())
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-e.abort:
			return
		case e.events <- ev:
		}
	}
}

// Format of event from iw: "<time>: <interface>[ <phy id>]: <message>"
// time: epoch time in second to 6 decimal places
// interface: "wdev 0x{idhex}"|"{ifname}"
// phy id: "(phy #{phyid})"
var eventRE = regexp.MustCompile(`\s*(\d+(?:\.\d+)?): ((?:\w+)|(?:wdev \w+))(?: \(phy #\d+\))?: (\w.*)`)

// parseEvent parses a single line from "iw event" into Event object.
func parseEvent(line string) (*Event, error) {
	m := eventRE.FindStringSubmatch(line)
	if len(m) != 4 {
		return nil, errors.Errorf("not a event: %s", line)
	}
	t, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return nil, errors.Wrapf(err, "expect timestamp as a float, actual: %q", m[1])
	}
	// Convert epoch time to time.Time.
	d, f := math.Modf(t)
	nsecMultiplier := float64(time.Second / time.Nanosecond)
	ts := time.Unix(int64(d), int64(f*nsecMultiplier))
	ev := &Event{
		Timestamp: ts,
		Interface: m[2],
		Message:   m[3],
	}
	ev.Type = detectEventType(ev)
	return ev, nil
}

// detectEventType identifies the type of the Event object.
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
	if strings.Contains(ev.Message, "ch_switch_started_notify") {
		return EventTypeChanSwitch
	}
	return EventTypeUnknown
}
