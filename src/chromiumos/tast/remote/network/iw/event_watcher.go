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

// EventType defines types of events captured by EventWatcher.
type EventType int

// EventType enums.
const (
	EventTypeDisconnect EventType = iota
	EventTypeChanSwitch
	EventTypeScanStart
	EventTypeConnected
	EventTypeUnknown
)

// Event is the structure to store one event from "iw event".
type Event struct {
	Type      EventType
	Timestamp time.Time
	Interface string
	Message   string
}

// EventWatcher captures events from a WiFi interface with "iw event".
type EventWatcher struct {
	done   chan struct{}
	cancel context.CancelFunc
	dut    *dut.DUT
	cmd    *ssh.Cmd
	events chan *Event
}

type eventWatcherConfig struct {
	eventsBufferSize int
}

// EventWatcherOption is a function object type used to specify options of NewEventWatcher.
type EventWatcherOption func(*eventWatcherConfig)

// EventsBufferSize returns a Option that sets the buffer size of the events channel.
func EventsBufferSize(size int) EventWatcherOption {
	return func(c *eventWatcherConfig) { c.eventsBufferSize = size }
}

// NewEventWatcher creates and starts a new EventWatcher.
// Note that the watcher may not be ready right after this function returned due to race condition,
// and it probably won't be fixed. Choose other solution if possible.
func NewEventWatcher(ctx context.Context, dut *dut.DUT, ops ...EventWatcherOption) (*EventWatcher, error) {
	conf := &eventWatcherConfig{eventsBufferSize: 10}
	for _, op := range ops {
		op(conf)
	}

	e := &EventWatcher{
		dut:    dut,
		done:   make(chan struct{}),
		events: make(chan *Event, conf.eventsBufferSize),
	}

	if err := e.start(ctx); err != nil {
		return nil, err
	}
	return e, nil
}

// start starts the "iw event" process and parser routine in background.
func (e *EventWatcher) start(ctx context.Context) error {
	e.cmd = e.dut.Command("iw", "event", "-t")
	r, err := e.cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := e.cmd.Start(ctx); err != nil {
		return err
	}
	ctx, e.cancel = context.WithCancel(ctx)
	go func() {
		defer close(e.done)
		defer close(e.events)
		e.parse(ctx, r)
	}()
	return nil
}

// Stop stops the EventWatcher.
// Note that it also closes the channel, that is, events are dropped even if
// they arrived before calling Stop() if the channel buffer is full.
func (e *EventWatcher) Stop(ctx context.Context) error {
	e.cmd.Abort()
	e.cmd.Wait(ctx) // Ignore the error due to abort.
	e.cancel()
	<-e.done // Wait for the bg routine to end.
	return nil
}

// ErrWatcherClosed is an error for detecting that watcher is closed.
var ErrWatcherClosed = errors.New("event channel is closed")

// Wait waits for the next event.
// If there is no more event (i.e., channel has been closed), Wait returns an ErrWatcherClosed.
func (e *EventWatcher) Wait(ctx context.Context) (*Event, error) {
	select {
	case ev, ok := <-e.events:
		if !ok {
			return nil, ErrWatcherClosed
		}
		return ev, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// WaitByType waits for the next matched event.
// Similar to Wait, a ErrWatcherClosed may be returned.
func (e *EventWatcher) WaitByType(ctx context.Context, ets ...EventType) (*Event, error) {
	for {
		ev, err := e.Wait(ctx)
		if err != nil {
			return nil, err
		}
		for _, et := range ets {
			if ev.Type == et {
				return ev, nil
			}
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
	if strings.HasPrefix(ev.Message, "scan started") {
		return EventTypeScanStart
	}
	if strings.HasPrefix(ev.Message, "connected") {
		return EventTypeConnected
	}
	return EventTypeUnknown
}
