// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
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
	expr := `function(eventType) {
		let watcher = {
			"events": [],
			"callback": (ev) => {
				watcher.events.push(ev);
			},
			"release": () => {
				this.removeEventListener(eventType, watcher.callback);
			}
		};
		this.addEventListener(eventType, watcher.callback);
		return watcher;
	}`
	if err := n.object.Call(ctx, object, expr, eventType); err != nil {
		return nil, errors.Wrap(err, "failed to execute the registration")
	}
	ew := &EventWatcher{object: object}
	return ew, nil
}

// WaitForEvent waits for at least one event to occur on the event watcher and
// returns the list of the events.
func (ew *EventWatcher) WaitForEvent(ctx context.Context, timeout time.Duration) ([]Event, error) {
	var events []Event
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := ew.object.Call(ctx, &events, `function() {
			let events = this.events;
			this.events = [];
			return events;
		}`); err != nil {
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

// Release frees the resources and the reference to Javascript for this watcher.
func (ew *EventWatcher) Release(ctx context.Context) error {
	if err := ew.object.Call(ctx, nil, `function () { this.release(); }`); err != nil {
		testing.ContextLog(ctx, "Failed to remove the event listener: ", err)
	}
	return ew.object.Release(ctx)
}
