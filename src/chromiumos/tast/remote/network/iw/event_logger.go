// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package iw

import (
	"context"
	"sync"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
)

// EventLogger captures events on wifi interface with "iw event".
type EventLogger struct {
	lock    sync.RWMutex
	done    chan struct{}
	events  []*Event
	watcher *EventWatcher
}

// NewEventLogger creates and starts a new EventLogger.
func NewEventLogger(ctx context.Context, dut *dut.DUT) (*EventLogger, error) {
	e := &EventLogger{
		done: make(chan struct{}),
	}
	ew, err := NewEventWatcher(ctx, dut)
	if err != nil {
		return nil, errors.New("failed to create event watcher")
	}
	e.watcher = ew
	go func() {
		defer close(e.done)
		for {
			ev, _ := e.watcher.Wait(ctx)
			if ev == nil {
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
