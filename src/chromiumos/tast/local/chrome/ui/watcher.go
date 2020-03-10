// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// Event represents a chrome.automation AutomationEvent.
// See https://chromium.googlesource.com/chromium/src/+/refs/heads/master/extensions/common/api/automation.idl#492
type Event struct {
	Target    *Node     `json:"target"`
	Type      EventType `json:"type"`
	EventFrom string    `json:"eventFrom"`
	MouseX    int       `json:"mouseX"`
	MouseY    int       `json:"mouseY"`
}

// EventWatcher registers the listener of AutomationEvent and watches the events.
type EventWatcher struct {
	object *chrome.JSObject
}

// NewWatcher creates a new event watcher on a node for the specified event
// type.
func NewWatcher(ctx context.Context, n *Node, eventType EventType) (*EventWatcher, error) {
	object := &chrome.JSObject{}
	expr := fmt.Sprintf(`function() {
		let watcher = {
			"events": [],
			"callback": (ev) => {
				watcher.events.push(ev);
			},
			"release": () => {
				this.removeEventListener(%q, watcher.callback);
			}
		};
		this.addEventListener(%q, watcher.callback);
		return watcher;
	}`, eventType, eventType)
	if err := n.object.Call(ctx, object, expr); err != nil {
		return nil, errors.Wrap(err, "failed to execute the registration")
	}
	ew := &EventWatcher{object: object}
	n.watchers = append(n.watchers, ew)
	return ew, nil
}

// Events returns the list of events which has occurred since the previous
// invocation of Events() or from the creation of the event watcher.
func (ew *EventWatcher) Events(ctx context.Context) ([]Event, error) {
	var events []Event
	if err := ew.object.Call(ctx, &events, `function() {
		let events = this.events;
		this.events = [];
		return events;
	}`); err != nil {
		return nil, errors.Wrap(err, "failed to obtain the event objects")
	}
	return events, nil
}

// WaitForEvent waits for at least one event to occur on the event watcher and
// returns the list of the events.
func (ew *EventWatcher) WaitForEvent(ctx context.Context, timeout time.Duration) ([]Event, error) {
	var events []Event
	var err error
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		events, err = ew.Events(ctx)
		if err != nil {
			return testing.PollBreak(err)
		}
		if len(events) == 0 {
			return errors.New("event hasn't occur yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return nil, err
	}
	return events, nil
}

func (ew *EventWatcher) release(ctx context.Context) {
	if err := ew.object.Call(ctx, nil, `function () { this.release(); }`); err != nil {
		testing.ContextLog(ctx, "Failed to remove the event listener: ", err)
	}
	ew.object.Release(ctx)
}
