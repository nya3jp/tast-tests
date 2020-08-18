// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package iw

import (
	"context"
	"sync"
	"time"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// EventLogger captures events on a WiFi interface with "iw event".
type EventLogger struct {
	lock    sync.RWMutex
	done    chan struct{}
	events  []*Event
	watcher *EventWatcher
}

// NewEventLogger creates and starts a new EventLogger.
// Note that the logger may not be ready right after this function returned due to race condition,
// and it probably won't be fixed. Choose other solution if possible.
func NewEventLogger(ctx context.Context, dut *dut.DUT, ops ...EventWatcherOption) (*EventLogger, error) {
	e := &EventLogger{
		done: make(chan struct{}),
	}
	ew, err := NewEventWatcher(ctx, dut, ops...)
	if err != nil {
		return nil, errors.New("failed to create an event watcher")
	}
	e.watcher = ew
	go func() {
		defer close(e.done)
		for {
			ev, err := e.watcher.Wait(ctx)
			if err != nil {
				if err != ErrWatcherClosed {
					testing.ContextLog(ctx, "Unexpected error from EventWatcher: ", err)
				}
				return
			}
			func() {
				e.lock.Lock()
				defer e.lock.Unlock()
				e.events = append(e.events, ev)
			}()
		}
	}()
	return e, nil
}

// Stop the EventLogger.
func (e *EventLogger) Stop(ctx context.Context) error {
	e.watcher.Stop(ctx)
	<-e.done // Wait for the bg routine to end.
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
func (e *EventLogger) EventsByType(ets ...EventType) []*Event {
	e.lock.RLock()
	defer e.lock.RUnlock()

	var ret []*Event
	for _, ev := range e.events {
		for _, et := range ets {
			if ev.Type == et {
				ret = append(ret, ev)
				break
			}
		}
	}
	return ret
}

// DisconnectTime finds the first disconnect event and returns the time.
func (e *EventLogger) DisconnectTime() (time.Time, error) {
	disconnectEvs := e.EventsByType(EventTypeDisconnect)
	if len(disconnectEvs) == 0 {
		return time.Time{}, errors.New("disconnect event not found")
	}
	return disconnectEvs[0].Timestamp, nil
}

// ConnectedTime finds the first connected event and returns the time.
func (e *EventLogger) ConnectedTime() (time.Time, error) {
	connectedEvs := e.EventsByType(EventTypeConnected)
	if len(connectedEvs) == 0 {
		return time.Time{}, errors.New("connected event not found")
	}
	return connectedEvs[0].Timestamp, nil
}
