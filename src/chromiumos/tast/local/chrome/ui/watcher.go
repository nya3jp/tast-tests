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
// See https://chromium.googlesource.com/chromium/src/+/HEAD/extensions/common/api/automation.idl#492
// Do not forget to release the Target node.
type Event struct {
	Target    *Node
	Type      EventType `json:"type"`
	EventFrom string    `json:"eventFrom"`
	MouseX    int       `json:"mouseX"`
	MouseY    int       `json:"mouseY"`
}

// EventSlice is a slice of Events. It is used for releaseing nodes in a group of events.
type EventSlice []Event

// Release frees the reference to Javascript for this node.
func (es EventSlice) Release(ctx context.Context) {
	for _, e := range es {
		if e.Target != nil {
			defer e.Target.Release(ctx)
		}
	}
}

// EventWatcher registers the listener of AutomationEvent and watches the events.
type EventWatcher struct {
	object *chrome.JSObject
	tconn  *chrome.TestConn
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
	ew := &EventWatcher{object: object, tconn: n.tconn}
	return ew, nil
}

// NewRootWatcher creates a new event watcher on the root node for the specified
// event type.
func NewRootWatcher(ctx context.Context, tconn *chrome.TestConn, eventType EventType) (*EventWatcher, error) {
	root, err := Root(ctx, tconn)
	if err != nil {
		return nil, err
	}
	defer root.Release(ctx)
	return NewWatcher(ctx, root, eventType)
}

// events returns the list of events in the watcher, and clears it.
func (ew *EventWatcher) events(ctx context.Context) (es EventSlice, retErr error) {
	eventsList := &chrome.JSObject{}
	if err := ew.object.Call(ctx, eventsList, `function() {
		let events = this.events;
		this.events = [];
		return events;
	}`); err != nil {
		return nil, err
	}
	defer eventsList.Release(ctx)

	var len int
	if err := eventsList.Call(ctx, &len, "function(){return this.length}"); err != nil {
		return nil, err
	}

	var events EventSlice
	defer func() {
		if retErr != nil {
			events.Release(ctx)
		}
	}()

	for i := 0; i < len; i++ {
		node, err := func() (*Node, error) {
			obj := &chrome.JSObject{}
			if err := eventsList.Call(ctx, obj, "function(i){return this[i].target}", i); err != nil {
				return nil, err
			}
			return NewNode(ctx, ew.tconn, obj)
		}()
		if err != nil {
			return nil, err
		}

		var event Event
		if err := eventsList.Call(ctx, &event, "function(i){return this[i]}", i); err != nil {
			node.Release(ctx)
			return nil, err
		}
		event.Target = node
		events = append(events, event)
	}

	return events, nil
}

// WaitForEvent waits for at least one event to occur on the event watcher and
// returns the list of the events.
// The caller is responsible to release EventSlice.
func (ew *EventWatcher) WaitForEvent(ctx context.Context, timeout time.Duration) (EventSlice, error) {
	var events EventSlice
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var errInPoll error
		if events, errInPoll = ew.events(ctx); errInPoll != nil {
			return testing.PollBreak(errInPoll)
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

// EnsureNoEvents waits the duration and returns nil if no events have
// occurred in the wait.
func (ew *EventWatcher) EnsureNoEvents(ctx context.Context, duration time.Duration) error {
	// First, clears the list of events beforehand.
	es, err := ew.events(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to clear the event list")
	}
	es.Release(ctx)
	// wait, and check the events in the wait.
	if err := testing.Sleep(ctx, duration); err != nil {
		return errors.Wrap(err, "failed to wait")
	}
	events, err := ew.events(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to access to the event list")
	}
	defer events.Release(ctx)
	if len(events) > 0 {
		return errors.Errorf("there are %d events", len(events))
	}
	return nil
}

// Release frees the resources and the reference to Javascript for this watcher.
func (ew *EventWatcher) Release(ctx context.Context) error {
	if err := ew.object.Call(ctx, nil, `function () { this.release(); }`); err != nil {
		testing.ContextLog(ctx, "Failed to remove the event listener: ", err)
	}
	return ew.object.Release(ctx)
}

// WaitForLocationChangeCompletedOnNode waits for any location-change events on
// the given node to be propagated to the automation API. Because
// automation API is asynchronous and eventually consistent with the desktop
// bounds, sometimes the automation API may report the intermediate bounds for
// an already completed animation. This function waits for such changes to be
// propagated fully to the automation API.
func WaitForLocationChangeCompletedOnNode(ctx context.Context, tconn *chrome.TestConn, node *Node) error {
	const (
		entireTimeout = 30 * time.Second
		timeout       = 2 * time.Second
	)

	ew, err := NewWatcher(ctx, node, EventTypeLocationChanged)
	if err != nil {
		return errors.Wrap(err, "failed to create an event watcher")
	}
	defer ew.Release(ctx)
	return testing.Poll(ctx, func(ctx context.Context) error {
		return ew.EnsureNoEvents(ctx, timeout)
	}, &testing.PollOptions{Timeout: entireTimeout})
}

// WaitForLocationChangeCompleted calls WaitForLocationChangeCompletedOnNode
// on the entire desktop.
func WaitForLocationChangeCompleted(ctx context.Context, tconn *chrome.TestConn) error {
	root, err := Root(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to access root")
	}
	defer root.Release(ctx)

	return WaitForLocationChangeCompletedOnNode(ctx, tconn, root)
}
